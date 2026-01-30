package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	buildBinaryPath = "bin/dialtone"
	arm64BinaryPath = "bin/dialtone-arm64"
	distIndexPath   = "src/core/web/dist/index.html"
)

func main() {
	fmt.Println("=== Starting Build Integration Test ===")

	allPassed := true
	runTest := func(name string, fn func() error) {
		fmt.Printf("\n[TEST] %s\n", name)
		if err := fn(); err != nil {
			fmt.Printf("FAIL: %s - %v\n", name, err)
			allPassed = false
		} else {
			fmt.Printf("PASS: %s\n", name)
		}
	}

	defer func() {
		fmt.Println("\n=== Build Integration Tests Completed ===")
		if !allPassed {
			fmt.Println("\n!!! SOME TESTS FAILED !!!")
			os.Exit(1)
		}
	}()

	runTest("Build System Stair-Step", TestBuildStairStep)
	fmt.Println()
}

func TestBuildStairStep() error {
	testStart := time.Now()
	fmt.Println("\n--- STEP 1: Verify DIALTONE_ENV ---")
	if os.Getenv("DIALTONE_ENV") == "" {
		return fmt.Errorf("DIALTONE_ENV is not set (run './dialtone.sh install' first)")
	}

	fmt.Println("\n--- STEP 2: Verify Go toolchain ---")
	if err := verifyGo(); err != nil {
		return err
	}

	fmt.Println("\n--- STEP 3: UI install (dependencies) ---")
	if _, err := runCmd("./dialtone.sh", "--timeout", "1800", "ui", "install"); err != nil {
		return err
	}

	fmt.Println("\n--- STEP 4: UI build ---")
	if _, err := runCmd("./dialtone.sh", "--timeout", "1800", "ui", "build"); err != nil {
		return err
	}
	if err := assertFileNonEmpty(distIndexPath); err != nil {
		return err
	}

	fmt.Println("\n--- STEP 5: Local build ---")
	output, err := runCmd("./dialtone.sh", "--timeout", "1800", "build", "--local")
	if err != nil {
		return err
	}
	if !strings.Contains(output, "Build successful") {
		return fmt.Errorf("local build finished without success marker")
	}
	if err := assertFileNonEmpty(buildBinaryPath); err != nil {
		return err
	}

	fmt.Println("\n--- STEP 6: Full build (local) ---")
	output, err = runCmd("./dialtone.sh", "--timeout", "3600", "build", "--full", "--local")
	if err != nil {
		return err
	}
	if !strings.Contains(output, "Full build successful") {
		return fmt.Errorf("full build finished without success marker")
	}

	fmt.Println("\n--- STEP 7: Verify binaries and web folders ---")
	if err := assertFileNonEmpty(buildBinaryPath); err != nil {
		return err
	}
	if err := assertFileNonEmpty(arm64BinaryPath); err != nil {
		return err
	}
	for _, dir := range webDirs() {
		if err := assertDirExists(dir); err != nil {
			return err
		}
	}
	for _, dir := range webBuildDirs() {
		if err := assertRecentFilesInDir(dir, testStart, 10); err != nil {
			return err
		}
	}

	return nil
}

func runCmd(name string, args ...string) (string, error) {
	fmt.Printf("> %s %v\n", name, args)
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		return string(output), fmt.Errorf("command failed: %s\n--- output (last 30 lines) ---\n%s", joinCmd(name, args...), tailLines(string(output), 30))
	}
	return string(output), nil
}

func assertFileNonEmpty(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("expected output at %s, but it was missing", path)
	}
	if info.Size() == 0 {
		return fmt.Errorf("expected %s to be non-empty", filepath.Clean(path))
	}
	return nil
}

func assertDirExists(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("expected directory at %s, but it was missing", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("expected %s to be a directory", filepath.Clean(path))
	}
	return nil
}

func assertRecentFilesInDir(path string, since time.Time, minCount int) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %v", path, err)
	}

	recent := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(since) {
			recent++
		}
	}

	if recent < minCount {
		return fmt.Errorf("expected at least %d recently created files in %s (since %s), found %d", minCount, filepath.Clean(path), since.Format(time.RFC3339), recent)
	}
	return nil
}

func verifyGo() error {
	envDir := os.Getenv("DIALTONE_ENV")
	if envDir != "" {
		goBin := filepath.Join(envDir, "go", "bin", "go")
		if _, err := os.Stat(goBin); err == nil {
			return nil
		}
	}
	if _, err := exec.LookPath("go"); err == nil {
		return nil
	}
	return fmt.Errorf("go not found in DIALTONE_ENV or system PATH (run './dialtone.sh install')")
}

func webDirs() []string {
	return []string{
		filepath.Join("src", "core", "web"),
		filepath.Join("src", "core", "earth"),
		filepath.Join("src", "plugins", "www", "app"),
	}
}

func webBuildDirs() []string {
	return []string{
		filepath.Join("src", "core", "web", "dist"),
	}
}

func joinCmd(name string, args ...string) string {
	if len(args) == 0 {
		return name
	}
	return name + " " + strings.Join(args, " ")
}

func tailLines(s string, max int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) <= max {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-max:], "\n")
}

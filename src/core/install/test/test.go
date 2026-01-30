package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"dialtone/cli/src/core/install"
)

func main() {
	if runInstallChildIfRequested() {
		return
	}
	fmt.Println("=== Starting Install Integration Test ===")

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
		fmt.Println("\n=== Install Integration Tests Completed ===")
		if !allPassed {
			fmt.Println("\n!!! SOME TESTS FAILED !!!")
			os.Exit(1)
		}
	}()

	runTest("Install System Stair-Step", TestInstallStairStep)
	runTest("Install List Report", TestInstallListReport)
	runTest("Install Dependency (zig)", TestInstallDependencyZig)
	runTest("Install Cache Reuse", TestInstallCacheReuse)
	fmt.Println()
}

func TestInstallStairStep() error {
	fmt.Println("\n--- STEP 1: Verify env configuration ---")
	envFile, envDir, err := requireEnvConfig()
	if err != nil {
		return err
	}
	printEnvSummary(envFile, envDir)

	fmt.Println("\n--- STEP 2: Verify env file name ---")
	if filepath.Base(envFile) != "test.env" {
		return fmt.Errorf("expected DIALTONE_ENV_FILE to be test.env, got %s", filepath.Clean(envFile))
	}

	fmt.Println("\n--- STEP 3: Verify expected env folders ---")
	if err := assertDirExists(envDir); err != nil {
		return err
	}
	if err := assertDirWritable(envDir); err != nil {
		return err
	}

	fmt.Println("\n--- STEP 4: HEAD check Go tarball ---")
	if err := headCheckGoTarball(); err != nil {
		return err
	}

	fmt.Println("\n--- STEP 5: Run install check (fast) ---")
	output, err := runInstallChildWithTimeout(3*time.Minute, "--check")
	checkPassed := err == nil && strings.Contains(output, "All dependencies are present.")
	if !checkPassed && !strings.Contains(output, "dependencies are missing") {
		return fmt.Errorf("install check failed unexpectedly")
	}
	if !checkPassed {
		fmt.Println("Install check reported missing dependencies. Proceeding with install...")

		fmt.Println("\n--- STEP 6: Run install ---")
		if _, err := runInstallChildWithTimeout(90*time.Minute); err != nil {
			return err
		}

		fmt.Println("\n--- STEP 7: Re-run install check ---")
		output, err = runInstallChildWithTimeout(3*time.Minute, "--check")
		if err != nil {
			return err
		}
		if !strings.Contains(output, "All dependencies are present.") {
			return fmt.Errorf("install check did not report success after install")
		}
	} else {
		fmt.Println("Install check passed. Skipping install for efficiency.")
	}

	fmt.Println("\n--- STEP 8: Verify Go toolchain ---")
	if err := assertFileExists(filepath.Join(envDir, "go", "bin", "go")); err != nil {
		return err
	}

	fmt.Println("\n--- STEP 9: Verify Node.js toolchain ---")
	if err := assertFileExists(filepath.Join(envDir, "node", "bin", "node")); err != nil {
		return err
	}

	fmt.Println("\n--- STEP 10: Verify GitHub CLI ---")
	if err := assertFileExists(filepath.Join(envDir, "gh", "bin", "gh")); err != nil {
		return err
	}

	fmt.Println("\n--- STEP 11: Verify Pixi ---")
	if err := assertFileExists(filepath.Join(envDir, "pixi", "pixi")); err != nil {
		return err
	}

	fmt.Println("\n--- STEP 12: Verify Zig ---")
	if err := assertFileExists(filepath.Join(envDir, "zig", "zig")); err != nil {
		return err
	}

	fmt.Println("\n--- STEP 13: Verify install folders ---")
	requiredDirs := []string{
		filepath.Join(envDir, "go"),
		filepath.Join(envDir, "node"),
		filepath.Join(envDir, "gh"),
		filepath.Join(envDir, "pixi"),
		filepath.Join(envDir, "zig"),
	}
	for _, dir := range requiredDirs {
		if err := assertDirExists(dir); err != nil {
			return err
		}
	}

	return nil
}

func TestInstallListReport() error {
	output, err := runInstallChildWithTimeout(2*time.Minute, "list")
	if err != nil {
		return err
	}
	if !strings.Contains(output, "Install list for") {
		return fmt.Errorf("install list did not include header")
	}
	if !strings.Contains(output, "download:") || !strings.Contains(output, "installed:") {
		return fmt.Errorf("install list output missing expected fields")
	}
	if !strings.Contains(output, "Zig") {
		return fmt.Errorf("install list output missing Zig entry")
	}
	return nil
}

func TestInstallDependencyZig() error {
	envFile, envDir, err := requireEnvConfig()
	if err != nil {
		return err
	}
	_, err = resolveCacheDir(envFile, envDir)
	if err != nil {
		return err
	}

	zigDir := filepath.Join(envDir, "zig")
	_ = os.RemoveAll(zigDir)

	if _, err := runInstallChildWithTimeout(30*time.Minute, "dependency", "zig"); err != nil {
		return err
	}
	return assertFileExists(filepath.Join(envDir, "zig", "zig"))
}

func TestInstallCacheReuse() error {
	envFile, envDir, err := requireEnvConfig()
	if err != nil {
		return err
	}
	cacheDir, err := resolveCacheDir(envFile, envDir)
	if err != nil {
		return err
	}
	zigArchive, err := zigArchiveName()
	if err != nil {
		return err
	}
	cachePath := filepath.Join(cacheDir, zigArchive)

	if !fileExists(cachePath) {
		if _, err := runInstallChildWithTimeout(30*time.Minute, "dependency", "zig"); err != nil {
			return err
		}
	}

	infoBefore, err := os.Stat(cachePath)
	if err != nil {
		return fmt.Errorf("failed to stat cache file: %w", err)
	}

	_ = os.RemoveAll(filepath.Join(envDir, "zig"))

	if _, err := runInstallChildWithTimeout(30*time.Minute, "dependency", "zig"); err != nil {
		return err
	}

	infoAfter, err := os.Stat(cachePath)
	if err != nil {
		return fmt.Errorf("failed to stat cache file after install: %w", err)
	}
	if !infoAfter.ModTime().Equal(infoBefore.ModTime()) {
		return fmt.Errorf("expected cache file to be reused without modification")
	}

	return assertFileExists(filepath.Join(envDir, "zig", "zig"))
}

func runInstallChildWithTimeout(timeout time.Duration, args ...string) (string, error) {
	name := os.Args[0]
	childArgs := append([]string{"--install-child"}, args...)
	fmt.Printf("> %s\n", joinCmd(name, childArgs...))
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, childArgs...)
	var output bytes.Buffer
	writer := io.MultiWriter(os.Stdout, &output)
	cmd.Stdout = writer
	cmd.Stderr = writer

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return output.String(), fmt.Errorf("command timed out after %s: %s\n--- output (last 30 lines) ---\n%s", timeout, joinCmd(name, childArgs...), tailLines(output.String(), 30))
		}
		return output.String(), fmt.Errorf("command failed: %s\n--- output (last 30 lines) ---\n%s", joinCmd(name, childArgs...), tailLines(output.String(), 30))
	}
	return output.String(), nil
}

func runInstallChildIfRequested() bool {
	if len(os.Args) > 1 && os.Args[1] == "--install-child" {
		args := os.Args[2:]
		if len(args) > 0 {
			switch args[0] {
			case "list":
				install.RunInstallList(args[1:])
				return true
			case "dependency":
				install.RunInstallDependency(args[1:])
				return true
			}
		}
		install.RunInstall(args)
		return true
	}
	return false
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

func assertFileExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("expected file at %s, but it was missing", path)
	}
	return nil
}

func assertDirWritable(path string) error {
	testFile := filepath.Join(path, ".dialtone_write_test")
	if err := os.WriteFile(testFile, []byte("ok"), 0644); err != nil {
		return fmt.Errorf("directory not writable: %s", filepath.Clean(path))
	}
	if err := os.Remove(testFile); err != nil {
		return fmt.Errorf("failed to clean up write test in %s", filepath.Clean(path))
	}
	return nil
}

func requireEnvConfig() (string, string, error) {
	envFile := os.Getenv("DIALTONE_ENV_FILE")
	if envFile == "" {
		return "", "", fmt.Errorf("DIALTONE_ENV_FILE is not set (run with ./dialtone.sh --env test.env install test)")
	}
	if err := assertFileExists(envFile); err != nil {
		return "", "", err
	}

	envDir := os.Getenv("DIALTONE_ENV")
	if envDir == "" {
		return "", "", fmt.Errorf("DIALTONE_ENV is not set (run with ./dialtone.sh --env test.env install test)")
	}

	fileEnv, err := readEnvValue(envFile, "DIALTONE_ENV")
	if err != nil {
		return "", "", err
	}
	if fileEnv == "" {
		return "", "", fmt.Errorf("DIALTONE_ENV not found in %s", filepath.Clean(envFile))
	}
	if normalizeEnvPath(fileEnv) != normalizeEnvPath(envDir) {
		return "", "", fmt.Errorf("DIALTONE_ENV mismatch: env=%s file=%s", envDir, fileEnv)
	}

	if err := assertDirExists(envDir); err != nil {
		return "", "", err
	}

	return envFile, envDir, nil
}

func printEnvSummary(envFile, envDir string) {
	fmt.Printf("Using DIALTONE_ENV_FILE: %s\n", filepath.Clean(envFile))
	fmt.Printf("Using DIALTONE_ENV: %s\n", filepath.Clean(envDir))
}

func readEnvValue(path, key string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		if k != key {
			continue
		}
		v := strings.TrimSpace(parts[1])
		v = strings.Trim(v, `"'`)
		return v, nil
	}
	return "", nil
}

func normalizeEnvPath(path string) string {
	if strings.HasPrefix(path, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

func resolveCacheDir(envFile, envDir string) (string, error) {
	cacheDir := os.Getenv("DIALTONE_CACHE")
	if cacheDir == "" {
		value, err := readEnvValue(envFile, "DIALTONE_CACHE")
		if err != nil {
			return "", err
		}
		cacheDir = value
	}
	if cacheDir == "" {
		cacheDir = filepath.Join(envDir, "cache")
	}
	cacheDir = normalizeEnvPath(cacheDir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}
	return cacheDir, nil
}

func zigArchiveName() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return fmt.Sprintf("zig-macos-aarch64-%s.tar.xz", install.ZigVersion), nil
		}
		if runtime.GOARCH == "amd64" {
			return fmt.Sprintf("zig-macos-x86_64-%s.tar.xz", install.ZigVersion), nil
		}
	case "linux":
		if runtime.GOARCH == "arm64" {
			return fmt.Sprintf("zig-linux-aarch64-%s.tar.xz", install.ZigVersion), nil
		}
		if runtime.GOARCH == "amd64" {
			return fmt.Sprintf("zig-linux-x86_64-%s.tar.xz", install.ZigVersion), nil
		}
	}
	return "", fmt.Errorf("unsupported platform for zig archive: %s/%s", runtime.GOOS, runtime.GOARCH)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func headCheckGoTarball() error {
	url, err := goTarballURL()
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Head(url)
	if err != nil {
		return fmt.Errorf("HEAD request failed for %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HEAD request returned %d for %s", resp.StatusCode, url)
	}

	contentLength := resp.Header.Get("Content-Length")
	if contentLength == "" {
		return fmt.Errorf("HEAD request missing Content-Length for %s", url)
	}
	size, err := strconv.ParseInt(contentLength, 10, 64)
	if err != nil || size <= 0 {
		return fmt.Errorf("invalid Content-Length for %s: %s", url, contentLength)
	}
	fmt.Printf("Go tarball available (%d bytes): %s\n", size, url)
	return nil
}

func goTarballURL() (string, error) {
	osName := runtime.GOOS
	if osName != "darwin" && osName != "linux" && osName != "windows" {
		return "", fmt.Errorf("unsupported OS for Go tarball: %s", osName)
	}

	arch := runtime.GOARCH
	if arch == "amd64" || arch == "arm64" {
		return fmt.Sprintf("https://go.dev/dl/go%s.%s-%s.tar.gz", install.GoVersion, osName, arch), nil
	}
	return "", fmt.Errorf("unsupported architecture for Go tarball: %s", arch)
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

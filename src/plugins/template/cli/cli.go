package cli

import (
	"dialtone/cli/src/plugins/template/test"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	command := args[0]
	switch command {
	case "install":
		dir := "src_v1"
		if len(args) > 1 {
			dir = args[1]
		}
		return RunInstall(dir)
	case "lint":
		dir := "src_v1"
		if len(args) > 1 {
			dir = args[1]
		}
		return RunLint(dir)
	case "smoke":
		if len(args) < 2 {
			return fmt.Errorf("usage: template smoke <dir>")
		}
		dir := args[1]
		// Robust smoke test involves building first
		if err := RunBuild(dir); err != nil {
			return err
		}
		return test.RunSmoke(dir)
	case "build":
		dir := "src_v1"
		if len(args) > 1 {
			dir = args[1]
		}
		return RunBuild(dir)
	case "src":
		srcFlags := flag.NewFlagSet("template src", flag.ExitOnError)
		n := srcFlags.Int("n", 0, "Version number to create")
		srcFlags.Parse(args[1:])
		if *n == 0 {
			return fmt.Errorf("usage: template src --n <N>")
		}
		return RunCreateVersion(*n)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func RunInstall(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Install: %s\n", versionDir)
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "template", versionDir, "ui")
	
	fmt.Println("   [TEMPLATE] Running bun install...")
	cmd := exec.Command("bun", "install")
	cmd.Dir = uiDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunLint(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Lint: %s\n", versionDir)
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "template", versionDir, "ui")
	
	fmt.Println("   [LINT] Running tsc...")
	cmd := exec.Command("bun", "x", "tsc", "--noEmit")
	cmd.Dir = uiDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunBuild(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Build: %s\n", versionDir)
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "template", versionDir, "ui")
	
	if err := RunInstall(versionDir); err != nil {
		return err
	}

	fmt.Println("   [BUILD] Running vite build (skipping tsc)...")
	// Use vite build directly, skipping tsc for speed and stability
	cmd := exec.Command("bun", "run", "vite", "build", "--emptyOutDir")
	cmd.Dir = uiDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %v", err)
	}

	fmt.Println(">> [TEMPLATE] Build successful")
	return nil
}

func RunCreateVersion(newVer int) error {
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "template")
	
	entries, _ := os.ReadDir(pluginDir)
	maxVer := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "src_v") {
			ver, _ := strconv.Atoi(e.Name()[5:])
			if ver > maxVer {
				maxVer = ver
			}
		}
	}

	if maxVer == 0 {
		return fmt.Errorf("no existing src_vN folders found to clone from")
	}

	srcDir := filepath.Join(pluginDir, fmt.Sprintf("src_v%d", maxVer))
	destDir := filepath.Join(pluginDir, fmt.Sprintf("src_v%d", newVer))

	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("version directory already exists: %s", destDir)
	}

	fmt.Printf(">> [TEMPLATE] Creating new version: src_v%d from src_v%d\n", newVer, maxVer)
	
	// Simple copy using cp -r
	cmd := exec.Command("cp", "-r", srcDir, destDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	fmt.Printf(">> [TEMPLATE] New version created at: %s\n", destDir)
	return nil
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh template <command>")
	fmt.Println("\nCommands:")
	fmt.Println("  install <dir>  Install UI dependencies")
	fmt.Println("  lint <dir>     Lint TypeScript code")
	fmt.Println("  build <dir>    Build everything needed (UI assets)")
	fmt.Println("  smoke <dir>    Run robust automated UI tests")
	fmt.Println("  src --n <N>    Generate next src_vN folder")
}
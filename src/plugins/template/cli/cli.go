package cli

import (
	"dialtone/cli/src/plugins/template/test"
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
	case "smoke":
		if len(args) < 2 {
			return fmt.Errorf("usage: nix smoke <dir>")
		}
		dir := args[1]
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
	case "new-version":
		return RunNewVersion()
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func RunBuild(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Build: %s\n", versionDir)
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "template", versionDir, "ui")
	
	fmt.Println("   [TEMPLATE] Running bun install...")
	installCmd := exec.Command("bun", "install")
	installCmd.Dir = uiDir
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return err
	}

	fmt.Println("   [TEMPLATE] Running bun run build...")
	cmd := exec.Command("bun", "run", "build")
	cmd.Dir = uiDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunNewVersion() error {
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

	newVer := maxVer + 1
	srcDir := filepath.Join(pluginDir, fmt.Sprintf("src_v%d", maxVer))
	destDir := filepath.Join(pluginDir, fmt.Sprintf("src_v%d", newVer))

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
	fmt.Println("  smoke <dir>    Run smoke tests")
	fmt.Println("  build <dir>    Build UI assets")
	fmt.Println("  new-version    Generate next src_vN folder")
}
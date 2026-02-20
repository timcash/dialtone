package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	test_plugin "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	versionDir := "src_v1"
	if len(args) > 0 && strings.HasPrefix(args[0], "src_v") {
		versionDir = args[0]
		args = args[1:]
	}

	switch command {
	case "install":
		fmt.Printf("simple-test [%s]: install command\n", versionDir)
	case "test":
		pluginDir := filepath.Join(repoRoot, "src", "plugins", "simple-test", versionDir)
		_ = filepath.Join(pluginDir, "ui")
		testPkg := "./plugins/simple-test/" + versionDir + "/test/cmd"
		
		opts := test_plugin.TestOptions{
			RepoRoot:   repoRoot,
			PluginDir:  pluginDir,
			VersionDir: versionDir,
			TestPkg:    testPkg,
			EnvPrefix:  "SIMPLE_TEST",
			BaseURL:    "http://127.0.0.1:3000", // No backend needed for simple test
			DevBaseURL: "http://127.0.0.1:3000",
		}
		if err := test_plugin.RunPluginTests(opts); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			os.Exit(1)
		}
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", fmt.Errorf("repo root not found")
		}
		cwd = parent
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh simple-test <command> [src_vN] [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  install    Install dependencies")
	fmt.Println("  test       Run tests")
}

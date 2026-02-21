package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "test":
		runTests(args)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown test command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func runTests(args []string) {
	version := "src_v1"
	if len(args) > 0 && args[0] != "" {
		version = args[0]
	}
	if version != "src_v1" {
		fmt.Printf("Error: unsupported version %s\n", version)
		os.Exit(1)
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command("go", "run", "./plugins/test/src_v1/test/01_self_check/main.go")
	cmd.Dir = filepath.Join(repoRoot, "src")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
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
	fmt.Println("Usage: test <command> [args]")
	fmt.Println("  test [src_v1]   Run test plugin verification suite")
}

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

	switch os.Args[1] {
	case "help", "-h", "--help":
		printUsage()
	case "install":
		runInstall(os.Args[2:])
	default:
		fmt.Printf("Unknown robot scaffold command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh robot <command> [args]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  install src_v1       Install robot src_v1 dependencies")
}

func runInstall(args []string) {
	version := "src_v1"
	if len(args) > 0 {
		version = args[0]
	}
	if version != "src_v1" {
		fmt.Printf("Unsupported robot version: %s\n", version)
		os.Exit(1)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Failed to resolve plugin directory: %v\n", err)
		os.Exit(1)
	}

	uiDir := filepath.Join(cwd, version, "ui")
	if _, err := os.Stat(uiDir); err != nil {
		fmt.Printf("Robot UI directory not found: %s\n", uiDir)
		os.Exit(1)
	}

	fmt.Printf(">> [Robot] Install: %s\n", version)
	fmt.Printf(">> [Robot] Checking local prerequisites...\n")
	fmt.Printf(">> [Robot] Installing UI dependencies (bun install)\n")

	installCmd := exec.Command("bun", "install")
	installCmd.Dir = uiDir
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	installCmd.Stdin = os.Stdin
	if err := installCmd.Run(); err != nil {
		fmt.Printf("Robot install error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf(">> [Robot] Install complete: %s\n", version)
}

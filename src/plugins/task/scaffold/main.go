package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: task <command> [args]")
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	// Use src_v1 by default if not specified in args
	srcVersion := "src_v1"
	// Check if any arg is src_vN
	for i, arg := range args {
		if len(arg) > 4 && arg[:4] == "src_" {
			srcVersion = arg
			// Remove from args
			args = append(args[:i], args[i+1:]...)
			break
		}
	}

	switch command {
	case "help":
		printUsage()
	case "test":
		runTest(srcVersion, args)
	default:
		runVersionedCommand(srcVersion, command, args)
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh task <command> [src_vN] [args]")
	fmt.Println("\nCommands:")
	fmt.Println("  create <task-name>   Create a new task in tasks/<name>/v1/root.md")
	fmt.Println("  validate <task-name> Validate a task markdown file")
	fmt.Println("  sign <task-name> --role <role>  Sign a task in v2")
	fmt.Println("  archive <task-name>  Promote v2 to v1 and prepare for next cycle")
	fmt.Println("  sync [issue-id]      Sync GitHub issues into tasks/ folder")
	fmt.Println("  test                 Run plugin tests")
	fmt.Println("  help                 Show this help")
}

func runTest(version string, args []string) {
	cwd, _ := os.Getwd()
	repoRoot := cwd
	if filepath.Base(cwd) == "src" {
		repoRoot = filepath.Dir(cwd)
	}
	
	testPath := filepath.Join(repoRoot, "src", "plugins", "task", version, "test", "03_smoke", "main.go")
	cmd := exec.Command("go", "run", testPath)
	cmd.Dir = filepath.Join(repoRoot, "src")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Test execution failed: %v\n", err)
		os.Exit(1)
	}
}

func runVersionedCommand(version, command string, args []string) {
	cwd, _ := os.Getwd()
	repoRoot := cwd
	if filepath.Base(cwd) == "src" {
		repoRoot = filepath.Dir(cwd)
	}

	sourcePath := filepath.Join(repoRoot, "src", "plugins", "task", version, "go", "main.go")
	if _, err := os.Stat(sourcePath); err != nil {
		fmt.Printf("Version %s not implemented or main.go missing at %s\n", version, sourcePath)
		return
	}

	cmdArgs := append([]string{"run", sourcePath}, command)
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = filepath.Join(repoRoot, "src")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Printf("Error running %s: %v\n", version, err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

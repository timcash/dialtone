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
	fmt.Println("
Commands:")
	fmt.Println("  create <task-name>   Create a new task in database/<name>/v1")
	fmt.Println("  validate <task-name> Validate a task markdown file")
	fmt.Println("  archive <task-name>  Promote v2 to v1 and prepare for next cycle")
	fmt.Println("  test                 Run plugin tests")
	fmt.Println("  help                 Show this help")
}

func runTest(version string, args []string) {
	fmt.Printf("Running tests for task plugin %s...
", version)
	// For now, just a placeholder or run the test binary if implemented
	testDir := filepath.Join("src", "plugins", "task", version, "test")
	fmt.Printf("Tests found in %s
", testDir)
}

func runVersionedCommand(version, command string, args []string) {
	// In a real implementation, we'd compile and run the versioned code.
	// For this scaffold, we'll try to find the compiled versioned tool.
	// For now, we'll run a go run on the versioned source.
	sourcePath := filepath.Join("src", "plugins", "task", version, "go", "main.go")
	if _, err := os.Stat(sourcePath); err != nil {
		fmt.Printf("Version %s not implemented or main.go missing
", version)
		return
	}

	cmdArgs := append([]string{"run", sourcePath, command}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error running %s: %v
", version, err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"dialtone/dev/plugins/worktree/src_v1/go/worktree"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "add":
		if len(args) == 0 {
			fmt.Println("Usage: worktree add <name> [--task <file>] [--branch <branch>]")
			return
		}
		name := args[0]
		var task, branch string
		for i := 1; i < len(args); i++ {
			if args[i] == "--task" && i+1 < len(args) {
				task = args[i+1]
				i++
			} else if args[i] == "--branch" && i+1 < len(args) {
				branch = args[i+1]
				i++
			}
		}
		if err := worktree.Add(name, task, branch); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "remove":
		if len(args) == 0 {
			fmt.Println("Usage: worktree remove <name>")
			return
		}
		if err := worktree.Remove(args[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "list":
		if err := worktree.List(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "test":
		runTests(args)
	default:
		printUsage()
	}
}

func runTests(args []string) {
	// Locate test runner
	cwd, _ := os.Getwd()
	// We are usually in repo root when run via dialtone.sh -> go run scaffold
	// But scaffold is in src/plugins/worktree/scaffold.
	// We need to run src/plugins/worktree/src_v1/test/cmd/main.go
	
	testCmd := filepath.Join("plugins", "worktree", "src_v1", "test", "cmd", "main.go")
	if _, err := os.Stat(testCmd); os.IsNotExist(err) {
		// Try from src?
		testCmd = filepath.Join("src", testCmd)
	}
	
	// Assuming we run from repo root or src
	// Let's find repo root
	root := findRepoRoot(cwd)
	
	// Since we want to use the managed go environment, we use dialtone.sh go run
	// Or just go run if we assume we are inside the env
	
	cmd := exec.Command("go", "run", "./plugins/worktree/src_v1/test/cmd/main.go")
	cmd.Dir = filepath.Join(root, "src")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

func findRepoRoot(cwd string) string {
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			return cwd
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return cwd
		}
		cwd = parent
	}
}

func printUsage() {
	fmt.Println("Usage: worktree <command> [args]")
	fmt.Println("  add <name> ...   Create worktree & tmux session")
	fmt.Println("  remove <name>    Remove worktree & tmux session")
	fmt.Println("  list             List active worktrees")
	fmt.Println("  test [src_v1]    Run tests")
}

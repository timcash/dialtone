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
	case "start":
		if len(args) == 0 {
			fmt.Println("Usage: worktree start <name> [--prompt <text>]")
			return
		}
		name := args[0]
		var prompt string
		for i := 1; i < len(args); i++ {
			if args[i] == "--prompt" && i+1 < len(args) {
				prompt = args[i+1]
				i++
			}
		}
		if err := worktree.Start(name, prompt); err != nil {
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
	case "attach":
		if len(args) == 0 {
			fmt.Println("Usage: worktree attach <worktree-name|list-index>")
			return
		}
		if err := worktree.Attach(args[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "tmux-logs":
		if len(args) == 0 {
			fmt.Println("Usage: worktree tmux-logs <worktree-name|list-index> [-n N]")
			return
		}
		selector := args[0]
		n := 10
		for i := 1; i < len(args); i++ {
			if args[i] == "-n" && i+1 < len(args) {
				var parsed int
				if _, err := fmt.Sscanf(args[i+1], "%d", &parsed); err == nil {
					n = parsed
				}
				i++
			}
		}
		if err := worktree.TmuxLogs(selector, n); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "verify-done":
		if len(args) == 0 {
			fmt.Println("Usage: worktree verify-done <worktree-name|list-index>")
			return
		}
		if err := worktree.VerifyDone(args[0]); err != nil {
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
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/gocache")
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
	fmt.Println("  start <name> ... Start Agent in existing worktree (uses TASK.md)")
	fmt.Println("  remove <name>    Remove worktree & tmux session")
	fmt.Println("  list             List worktrees + task status")
	fmt.Println("  attach <id>      Attach tmux by worktree name or list index")
	fmt.Println("  tmux-logs <id>   Show last tmux lines (-n N, default 10)")
	fmt.Println("  verify-done <id> Verify TASK.md done signature (+agent_test check)")
	fmt.Println("  test [src_v1]    Run tests")
}

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	worktreev1 "dialtone/dev/plugins/worktree/src_v1/go/worktree"
)

func main() {
	logs.SetOutput(os.Stdout)
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	version, command, args, warnedOldOrder, err := parseArgs(os.Args[1:])
	if err != nil {
		logs.Error("%v", err)
		printUsage()
		os.Exit(1)
	}
	if warnedOldOrder {
		logs.Warn("old worktree CLI order is deprecated. Use: ./dialtone.sh worktree src_v1 <command> [args]")
	}
	if version != "src_v1" {
		logs.Error("unsupported version %s", version)
		os.Exit(1)
	}

	switch command {
	case "help", "-h", "--help":
		printUsage()
	case "add":
		runAdd(args)
	case "start":
		runStart(args)
	case "remove":
		runRequireName(args, worktreev1.Remove, "remove")
	case "list":
		if err := worktreev1.List(); err != nil {
			logs.Error("worktree list failed: %v", err)
			os.Exit(1)
		}
	case "attach":
		runRequireName(args, worktreev1.Attach, "attach")
	case "tmux-logs":
		runTmuxLogs(args)
	case "cleanup":
		runCleanup(args)
	case "verify-done":
		runRequireName(args, worktreev1.VerifyDone, "verify-done")
	case "test":
		runTests(args)
	default:
		logs.Error("unknown worktree command: %s", command)
		printUsage()
		os.Exit(1)
	}
}

func parseArgs(args []string) (version, command string, rest []string, warnedOldOrder bool, err error) {
	if len(args) == 0 {
		return "", "", nil, false, fmt.Errorf("missing arguments")
	}
	if isHelp(args[0]) {
		return "src_v1", "help", nil, false, nil
	}
	if strings.HasPrefix(args[0], "src_v") {
		if len(args) < 2 {
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh worktree src_v1 <command> [args])")
		}
		return args[0], args[1], args[2:], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], args[0], args[2:], true, nil
	}
	return "", "", nil, false, fmt.Errorf("expected version as first worktree argument (usage: ./dialtone.sh worktree src_v1 <command> [args])")
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh worktree src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  add <name> [--task <file>] [--branch <branch>]")
	logs.Raw("  start <name> [--prompt <text>]")
	logs.Raw("  remove <name>")
	logs.Raw("  list")
	logs.Raw("  attach <name|index>")
	logs.Raw("  tmux-logs <name|index> [-n N]")
	logs.Raw("  cleanup [--all]")
	logs.Raw("  verify-done <name|index>")
	logs.Raw("  test")
}

func runAdd(args []string) {
	if len(args) == 0 {
		logs.Error("Usage: ./dialtone.sh worktree src_v1 add <name> [--task <file>] [--branch <branch>]")
		os.Exit(1)
	}
	name := args[0]
	var task, branch string
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--task":
			if i+1 < len(args) {
				task = args[i+1]
				i++
			}
		case "--branch":
			if i+1 < len(args) {
				branch = args[i+1]
				i++
			}
		}
	}
	if err := worktreev1.Add(name, task, branch); err != nil {
		logs.Error("worktree add failed: %v", err)
		os.Exit(1)
	}
}

func runStart(args []string) {
	if len(args) == 0 {
		logs.Error("Usage: ./dialtone.sh worktree src_v1 start <name> [--prompt <text>]")
		os.Exit(1)
	}
	name := args[0]
	var prompt string
	for i := 1; i < len(args); i++ {
		if args[i] == "--prompt" && i+1 < len(args) {
			prompt = args[i+1]
			i++
		}
	}
	if err := worktreev1.Start(name, prompt); err != nil {
		logs.Error("worktree start failed: %v", err)
		os.Exit(1)
	}
}

func runRequireName(args []string, fn func(string) error, command string) {
	if len(args) == 0 {
		logs.Error("Usage: ./dialtone.sh worktree src_v1 %s <name|index>", command)
		os.Exit(1)
	}
	if err := fn(args[0]); err != nil {
		logs.Error("worktree %s failed: %v", command, err)
		os.Exit(1)
	}
}

func runTmuxLogs(args []string) {
	if len(args) == 0 {
		logs.Error("Usage: ./dialtone.sh worktree src_v1 tmux-logs <name|index> [-n N]")
		os.Exit(1)
	}
	selector := args[0]
	n := 10
	for i := 1; i < len(args); i++ {
		if args[i] == "-n" && i+1 < len(args) {
			if _, err := fmt.Sscanf(args[i+1], "%d", &n); err == nil {
				i++
			}
		}
	}
	if err := worktreev1.TmuxLogs(selector, n); err != nil {
		logs.Error("worktree tmux-logs failed: %v", err)
		os.Exit(1)
	}
}

func runCleanup(args []string) {
	all := false
	for _, a := range args {
		if a == "--all" {
			all = true
		}
	}
	if err := worktreev1.Cleanup(all); err != nil {
		logs.Error("worktree cleanup failed: %v", err)
		os.Exit(1)
	}
}

func runTests(args []string) {
	if len(args) > 0 {
		logs.Error("Usage: ./dialtone.sh worktree src_v1 test")
		os.Exit(1)
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		logs.Error("%v", err)
		os.Exit(1)
	}
	cmd := exec.Command("go", "run", "./plugins/worktree/src_v1/test/cmd/main.go")
	cmd.Dir = filepath.Join(repoRoot, "src")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
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

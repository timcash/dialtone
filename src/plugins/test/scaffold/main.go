package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	version, cmd, warnedOldOrder, err := parseArgs(os.Args[1:])
	if err != nil {
		logs.Error("%v", err)
		printUsage()
		os.Exit(1)
	}
	if warnedOldOrder {
		logs.Warn("old test CLI order is deprecated. Use: ./dialtone.sh test src_v1 <command> [args]")
	}

	switch cmd {
	case "test":
		runTests(version)
	case "help", "-h", "--help":
		printUsage()
	default:
		logs.Error("Unknown test command: %s", cmd)
		printUsage()
		os.Exit(1)
	}
}

func parseArgs(args []string) (version, command string, warnedOldOrder bool, err error) {
	if len(args) == 0 {
		return "", "", false, fmt.Errorf("missing arguments")
	}
	if isHelp(args[0]) {
		return "src_v1", "help", false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[0], "src_v") {
		return args[0], args[1], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], args[0], true, nil
	}
	return "", "", false, fmt.Errorf("expected version as first test argument (for example: ./dialtone.sh test src_v1 test)")
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func runTests(version string) {
	if version != "src_v1" {
		logs.Error("Unsupported version %s", version)
		os.Exit(1)
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		logs.Error("%v", err)
		os.Exit(1)
	}

	cmd := exec.Command("go", "run", "./plugins/test/src_v1/test/cmd/main.go")
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
	logs.Info("Usage: ./dialtone.sh test src_v1 <command> [args]")
	logs.Info("  test            Run test plugin verification suite")
}

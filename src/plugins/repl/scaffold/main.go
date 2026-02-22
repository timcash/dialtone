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
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		return
	}

	version, command, warnedOldOrder, err := parseArgs(args)
	if err != nil {
		logs.Error("%v", err)
		printUsage()
		os.Exit(1)
	}
	if warnedOldOrder {
		logs.Warn("old repl CLI order is deprecated. Use: ./dialtone.sh repl src_v1 <command> [args]")
	}

	switch command {
	case "test":
		if err := runVersionedTest(version); err != nil {
			logs.Error("REPL test error: %v", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		printUsage()
	default:
		logs.Error("Unknown repl command: %s", command)
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
	if strings.HasPrefix(args[0], "src_v") {
		if len(args) < 2 {
			return "", "", false, fmt.Errorf("missing command (usage: ./dialtone.sh repl src_v1 <command> [args])")
		}
		return args[0], args[1], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], args[0], true, nil
	}
	return "", "", false, fmt.Errorf("expected version as first repl argument (usage: ./dialtone.sh repl src_v1 <command> [args])")
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func runVersionedTest(versionDir string) error {
	cwd, _ := os.Getwd()
	root := cwd
	for {
		if _, err := os.Stat(filepath.Join(root, "dialtone.sh")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			root = cwd
			break
		}
		root = parent
	}

	testPkg := "./plugins/repl/" + versionDir + "/test/cmd/main.go"
	goArgs := []string{"src_v1", "exec", "run", testPkg}
	fullArgs := append([]string{"go"}, goArgs...)
	cmd := exec.Command(filepath.Join(root, "dialtone.sh"), fullArgs...)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh repl src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  test                     Run REPL src_v1 tests")
	logs.Raw("  help                     Show this help")
}

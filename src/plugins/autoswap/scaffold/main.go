package main

import (
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	autoswap "dialtone/dev/plugins/autoswap/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	version, command, rest, warnedOldOrder, err := parseArgs(os.Args[1:])
	if err != nil {
		logs.Error("%v", err)
		printUsage()
		os.Exit(1)
	}
	if warnedOldOrder {
		logs.Warn("old autoswap CLI order is deprecated. Use: ./dialtone.sh autoswap src_v1 <command> [args]")
	}
	if version != "src_v1" {
		logs.Error("unsupported autoswap version: %s", version)
		os.Exit(1)
	}

	switch command {
	case "run":
		err = autoswap.Run(rest)
	case "stage":
		err = autoswap.Stage(rest)
	case "test":
		err = runTests()
	case "help", "-h", "--help":
		printUsage()
		return
	default:
		err = fmt.Errorf("unknown autoswap command: %s", command)
	}
	if err != nil {
		logs.Error("autoswap error: %v", err)
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
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh autoswap src_v1 <command> [args])")
		}
		return args[0], args[1], args[2:], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], args[0], args[2:], true, nil
	}
	return "", "", nil, false, fmt.Errorf("expected version as first autoswap argument (usage: ./dialtone.sh autoswap src_v1 <command> [args])")
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh autoswap src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  stage   Validate manifest and artifact paths")
	logs.Raw("  run     Stage + start composition smoke flow")
	logs.Raw("  test    Run autoswap src_v1 tests")
	logs.Raw("  help    Show this help")
}

func runTests() error {
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return err
	}
	testPkg := "./plugins/autoswap/src_v1/test/cmd"
	cmd := exec.Command(filepath.Join(rt.RepoRoot, "dialtone.sh"), "go", "src_v1", "exec", "run", testPkg)
	cmd.Dir = rt.RepoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

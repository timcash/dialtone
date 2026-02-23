package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	githubv1 "dialtone/dev/plugins/github/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	if len(os.Args) < 2 {
		githubv1.PrintUsage()
		return
	}

	version, cmd, args, warnedOldOrder, err := parseArgs(os.Args[1:])
	if err != nil {
		logs.Error("%v", err)
		githubv1.PrintUsage()
		os.Exit(1)
	}
	if warnedOldOrder {
		logs.Warn("old github CLI order is deprecated. Use: ./dialtone.sh github src_v1 <command> [args]")
	}
	if version != "src_v1" {
		logs.Error("unsupported github version: %s", version)
		os.Exit(1)
	}

	switch cmd {
	case "test":
		if err := runTests(); err != nil {
			logs.Error("%v", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		githubv1.PrintUsage()
	default:
		if err := githubv1.Run(append([]string{cmd}, args...)); err != nil {
			logs.Error("github command failed: %v", err)
			os.Exit(1)
		}
	}
}

func parseArgs(args []string) (version, command string, rest []string, warnedOldOrder bool, err error) {
	if len(args) == 0 {
		return "", "", nil, false, fmt.Errorf("missing arguments")
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		return "src_v1", "help", nil, false, nil
	}
	if len(args) >= 2 && args[0] == "src_v1" {
		return "src_v1", args[1], args[2:], false, nil
	}
	if len(args) >= 2 && args[1] == "src_v1" {
		return "src_v1", args[0], args[2:], true, nil
	}
	return "", "", nil, false, fmt.Errorf("expected version as first github argument (usage: ./dialtone.sh github src_v1 <command> [args])")
}

func runTests() error {
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return err
	}
	preset := configv1.NewPluginPreset(rt, "github", "src_v1")
	testMain := filepath.Join(preset.TestCmd, "main.go")
	cmd := exec.Command(filepath.Join(rt.RepoRoot, "dialtone.sh"), "go", "src_v1", "exec", "run", testMain)
	cmd.Dir = rt.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

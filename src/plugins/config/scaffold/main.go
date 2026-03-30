package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	version, command, rest, warnedOldOrder, err := parseArgs(os.Args[1:])
	if err != nil {
		logs.Error("%v", err)
		printUsage()
		os.Exit(1)
	}
	if warnedOldOrder {
		logs.Warn("old config CLI order is deprecated. Use: ./dialtone.sh config src_v1 <command> [args]")
	}

	if version != "src_v1" {
		logs.Error("unsupported version: %s", version)
		os.Exit(1)
	}

	switch command {
	case "help", "-h", "--help":
		printUsage()
	case "runtime":
		rt, err := configv1.ResolveRuntime("")
		if err != nil {
			logs.Error("config runtime error: %v", err)
			os.Exit(1)
		}
		_ = json.NewEncoder(os.Stdout).Encode(rt)
	case "test":
		rt, err := configv1.ResolveRuntime("")
		if err != nil {
			logs.Error("runtime error: %v", err)
			os.Exit(1)
		}
		goBin := strings.TrimSpace(rt.GoBin)
		if goBin == "" {
			goBin = "go"
		}
		cmd := exec.Command(goBin, "run", "./plugins/config/src_v1/test/cmd/main.go")
		cmd.Dir = rt.SrcRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			os.Exit(1)
		}
	case "apply":
		rt, err := configv1.ResolveRuntime("")
		if err != nil {
			logs.Error("resolve runtime error: %v", err)
			os.Exit(1)
		}
		if err := configv1.LoadEnvFile(rt); err != nil {
			logs.Error("load env error: %v", err)
			os.Exit(1)
		}
		if err := configv1.ApplyRuntimeEnv(rt); err != nil {
			logs.Error("apply runtime env error: %v", err)
			os.Exit(1)
		}
		logs.Info("Applied runtime env for repo=%s", rt.RepoRoot)
	default:
		logs.Error("unknown config command: %s", command)
		printUsage()
		os.Exit(1)
	}

	_ = rest
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
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh config src_v1 <command>)")
		}
		return args[0], args[1], args[2:], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], args[0], args[2:], true, nil
	}
	return "", "", nil, false, fmt.Errorf("expected version as first config argument (for example: ./dialtone.sh config src_v1 runtime)")
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh config src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  runtime     Print resolved runtime config as JSON")
	logs.Raw("  apply       Load env file + apply runtime vars to current process")
	logs.Raw("  test        Run config plugin src_v1 tests")
}

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

	if err := runCommand(command, rest); err != nil {
		logs.Error("config src_v1 %s failed: %v", command, err)
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

func runCommand(command string, args []string) error {
	switch command {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "install":
		return runInstall(args)
	case "format":
		return runManagedGo(command, args, "fmt", "./plugins/config/...")
	case "lint":
		return runManagedGo(command, args, "vet", "./plugins/config/...")
	case "build":
		return runManagedGo(command, args, "build", "./plugins/config/...")
	case "runtime":
		return runRuntime(args)
	case "apply":
		return runApply(args)
	case "test":
		return runTest(args)
	default:
		return fmt.Errorf("unknown config command: %s", command)
	}
}

func runInstall(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("install does not accept extra arguments")
	}
	logs.Info("config src_v1 install: no-op")
	return nil
}

func runRuntime(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("runtime does not accept extra arguments")
	}
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(rt)
}

func runApply(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("apply does not accept extra arguments")
	}
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return err
	}
	if err := configv1.LoadEnvFile(rt); err != nil {
		return err
	}
	if err := configv1.ApplyRuntimeEnv(rt); err != nil {
		return err
	}
	logs.Info("Applied runtime env for repo=%s", rt.RepoRoot)
	return nil
}

func runTest(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("test does not accept extra arguments")
	}
	return runManagedGo("test", args, "run", "./plugins/config/src_v1/test/cmd/main.go")
}

func runManagedGo(command string, extraArgs []string, args ...string) error {
	if len(extraArgs) > 0 {
		return fmt.Errorf("%s does not accept extra arguments", command)
	}
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return err
	}
	goBin := strings.TrimSpace(rt.GoBin)
	if goBin == "" {
		goBin = "go"
	}
	cmd := exec.Command(goBin, args...)
	cmd.Dir = rt.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh config src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  install     Verify shared runtime config access")
	logs.Raw("  format      Run go fmt for the config plugin")
	logs.Raw("  lint        Run go vet for the config plugin")
	logs.Raw("  build       Run go build for the config plugin")
	logs.Raw("  runtime     Print resolved runtime config as JSON")
	logs.Raw("  apply       Load env file + apply runtime vars to current process")
	logs.Raw("  test        Run config plugin src_v1 tests")
}

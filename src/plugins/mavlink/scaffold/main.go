package main

import (
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	"fmt"
	"os"
	"os/exec"
	"strings"
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
		logs.Warn("old mavlink CLI order is deprecated. Use: ./dialtone.sh mavlink src_v1 <command> [args]")
	}
	if version != "src_v1" {
		logs.Error("unsupported mavlink version: %s", version)
		os.Exit(1)
	}

	switch command {
	case "run", "params", "key-params", "stream", "version":
		if err := runMavlinkCommand(command, rest); err != nil {
			logs.Error("mavlink %s failed: %v", command, err)
			os.Exit(1)
		}
	case "test":
		if err := runMavlinkTests(); err != nil {
			logs.Error("mavlink test failed: %v", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		printUsage()
	default:
		logs.Error("unknown mavlink command: %s", command)
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
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh mavlink src_v1 <command> [args])")
		}
		return args[0], args[1], args[2:], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], args[0], args[2:], true, nil
	}
	return "", "", nil, false, fmt.Errorf("expected version as first mavlink argument (usage: ./dialtone.sh mavlink src_v1 <command> [args])")
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh mavlink src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  run         Run mavlink bridge")
	logs.Raw("  params      Read MAVLink params")
	logs.Raw("  key-params  Read key rover params")
	logs.Raw("  stream      Stream mavlink.* from remote host and optionally publish rover.command")
	logs.Raw("  test        Run mavlink tests")
	logs.Raw("  version     Print version")
	logs.Raw("  help        Show this help")
}

func runMavlinkCommand(command string, args []string) error {
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return err
	}
	goBin := filepathOrFallbackGo()
	cmdArgs := []string{"run", "./plugins/mavlink/src_v1/cmd/main.go", command}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command(goBin, cmdArgs...)
	cmd.Dir = rt.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runMavlinkTests() error {
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return err
	}
	goBin := filepathOrFallbackGo()
	cmd := exec.Command(goBin, "test", "./plugins/mavlink/...")
	cmd.Dir = rt.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func filepathOrFallbackGo() string {
	goBin := os.Getenv("DIALTONE_GO_BIN")
	if strings.TrimSpace(goBin) != "" {
		if _, err := os.Stat(goBin); err == nil {
			return goBin
		}
	}
	fallback, err := exec.LookPath("go")
	if err != nil {
		return "go"
	}
	return fallback
}

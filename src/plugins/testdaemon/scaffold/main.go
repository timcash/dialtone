package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	testdaemon "dialtone/dev/plugins/testdaemon/src_v1"
)

func main() {
	logs.SetOutput(os.Stdout)

	version, rest, err := parseArgs(os.Args[1:])
	if err != nil {
		logs.Error("%v", err)
		printUsage()
		os.Exit(1)
	}

	switch version {
	case "src_v1":
		if err := testdaemon.Run(rest); err != nil {
			var exitErr *testdaemon.ExitStatusError
			if errors.As(err, &exitErr) {
				if strings.TrimSpace(exitErr.Message) != "" {
					logs.Error("%s", strings.TrimSpace(exitErr.Message))
				}
				os.Exit(exitErr.Code)
			}
			logs.Error("testdaemon src_v1 error: %v", err)
			os.Exit(1)
		}
	default:
		logs.Error("Unsupported testdaemon version: %s", version)
		os.Exit(1)
	}
}

func parseArgs(args []string) (string, []string, error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("missing arguments")
	}
	if isHelp(args[0]) {
		return "src_v1", []string{"help"}, nil
	}
	if strings.HasPrefix(args[0], "src_v") {
		return args[0], args[1:], nil
	}
	return "", nil, fmt.Errorf("expected src_v1 as first testdaemon argument (for example: ./dialtone.sh testdaemon src_v1 build)")
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func printUsage() {
	logs.Info("Usage: ./dialtone.sh testdaemon src_v1 <command> [args]")
	logs.Info("Commands:")
	logs.Info("  format")
	logs.Info("  build")
	logs.Info("  test")
	logs.Info("  run --mode once")
	logs.Info("  service --mode start|status|stop --name demo")
	logs.Info("  emit-progress --steps 5")
	logs.Info("  sleep --seconds 10")
	logs.Info("  exit-code --code 17")
	logs.Info("  panic")
	logs.Info("  crash")
	logs.Info("  hang")
	logs.Info("  heartbeat --name demo [--mode show|stop|resume]")
	logs.Info("  shutdown --name demo")
}

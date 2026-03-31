package main

import (
	"fmt"
	"os"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	robotv2 "dialtone/dev/plugins/robot/src_v2"
)

func main() {
	logs.SetOutput(os.Stdout)
	version, command, args, warnedOldOrder, err := parseArgs(os.Args[1:])
	if err != nil {
		logs.Error("%v", err)
		robotv2.PrintUsage()
		os.Exit(1)
	}
	if warnedOldOrder {
		logs.Warn("old robot CLI order is deprecated. Use: ./dialtone.sh robot src_v2 <command> [args]")
	}
	if isHelp(command) {
		robotv2.PrintUsage()
		return
	}

	if err := robotv2.Run(version, command, args); err != nil {
		logs.Error("robot error: %v", err)
		os.Exit(1)
	}
}

func parseArgs(args []string) (version, command string, rest []string, warnedOldOrder bool, err error) {
	if len(args) == 0 {
		return "", "", nil, false, fmt.Errorf("missing arguments")
	}
	if isHelp(args[0]) {
		return "src_v2", "help", nil, false, nil
	}
	if strings.HasPrefix(args[0], "src_v") {
		if len(args) < 2 {
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh robot src_v2 <command> [args])")
		}
		return args[0], args[1], args[2:], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], args[0], args[2:], true, nil
	}

	// Fallback: no explicit version provided, use latest version and first arg as command.
	return "", args[0], args[1:], false, nil
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

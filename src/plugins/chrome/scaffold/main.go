package main

import (
	"fmt"
	"os"
	"strings"

	chromev3 "dialtone/dev/plugins/chrome/src_v3"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	version, rest, warnedOldOrder, err := parseChromeArgs(os.Args[1:])
	if err != nil {
		logs.Error("%v", err)
		_ = chromev3.Run([]string{"help"})
		os.Exit(1)
	}
	if warnedOldOrder {
		logs.Warn("old chrome CLI order is deprecated. Use: ./dialtone.sh chrome src_v3 <command> [args]")
	}

	if version != "src_v3" {
		logs.Error("unsupported chrome version: %s", version)
		_ = chromev3.Run([]string{"help"})
		os.Exit(1)
	}
	if len(rest) == 0 {
		rest = []string{"help"}
	}
	if err := chromev3.Run(rest); err != nil {
		logs.Error("chrome src_v3 error: %v", err)
		os.Exit(1)
	}
}

func parseChromeArgs(args []string) (version string, rest []string, warnedOldOrder bool, err error) {
	if len(args) == 0 {
		return "src_v3", []string{"help"}, false, nil
	}
	if isHelp(args[0]) {
		return "src_v3", []string{"help"}, false, nil
	}
	if strings.HasPrefix(args[0], "src_v") {
		if len(args) == 1 {
			return args[0], []string{"help"}, false, nil
		}
		return args[0], args[1:], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], append([]string{args[0]}, args[2:]...), true, nil
	}
	return "", nil, false, fmt.Errorf("expected version as first chrome argument (for example: ./dialtone.sh chrome src_v3 status)")
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

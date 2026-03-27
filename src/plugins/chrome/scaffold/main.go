package main

import (
	"fmt"
	"os"

	chromev3 "dialtone/dev/plugins/chrome/src_v3"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	version, rest, err := parseChromeArgs(os.Args[1:])
	if err != nil {
		logs.Error("%v", err)
		printUsage()
		os.Exit(1)
	}

	if version == "src_v3" {
		if err := chromev3.Run(rest); err != nil {
			logs.Error("chrome src_v3 error: %v", err)
			os.Exit(1)
		}
	} else if version == "src_v4" {
		logs.Error("chrome src_v4 is not available in this checkout")
		os.Exit(1)
	} else {
		printUsage()
		os.Exit(1)
	}
}

func parseChromeArgs(args []string) (string, []string, error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("missing arguments")
	}
	if isHelp(args[0]) {
		// Default to v3 help if no version specified
		return "src_v3", []string{"help"}, nil
	}
	if args[0] == "src_v3" || args[0] == "src_v4" {
		return args[0], args[1:], nil
	}
	return "", nil, fmt.Errorf("expected src_v3 or src_v4 as first chrome argument (for example: ./dialtone.sh chrome src_v4 daemon)")
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func printUsage() {
	logs.Info("Usage: ./dialtone.sh chrome <version> <command> [args]")
	logs.Info("Versions:")
	logs.Info("  src_v3  - Legacy CDP-based automation")
	logs.Info("  src_v4  - Modern MCP-based automation")
}

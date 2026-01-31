package cli

import (
	"fmt"
	"os"
	"strings"
)

func Run(args []string) {
	if len(args) == 0 {
		RunNext(nil)
		return
	}

	switch args[0] {
	case "next":
		RunNext(args[1:])
	case "list":
		RunList()
	default:
		// Treat unknown args as nexttone --sign
		if strings.HasPrefix(args[0], "--sign") {
			RunNext(args)
			return
		}
		fmt.Printf("Unknown nexttone subcommand: %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh nexttone [next|list] [--sign yes|no]")
}

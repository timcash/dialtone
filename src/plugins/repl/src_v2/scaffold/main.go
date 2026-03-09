package main

import (
	"flag"
	"fmt"
	"os"

	"dialtone/dev/plugins/repl/src_v2/go/repl"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "run":
		if err := repl.RunREPLV2(args); err != nil {
			fmt.Fprintf(os.Stderr, "REPL error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown repl src_v2 command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh repl src_v2 <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  run       Start the interactive REPL v2")
}

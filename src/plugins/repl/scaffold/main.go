package main

import (
	"fmt"
	"os"

	"dialtone/dev/plugins/repl/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Printf("REPL plugin error: %v\n", err)
		os.Exit(1)
	}
}

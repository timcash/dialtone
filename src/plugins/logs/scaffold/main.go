package main

import (
	"fmt"
	"os"

	"dialtone/dev/plugins/logs/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Printf("logs error: %v\n", err)
		os.Exit(1)
	}
}

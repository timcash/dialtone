package main

import (
	"os"

	"dialtone/dev/plugins/ui/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		os.Exit(1)
	}
}

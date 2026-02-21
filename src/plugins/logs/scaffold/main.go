package main

import (
	"os"

	"dialtone/dev/plugins/logs/cli"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		logs.Error("logs error: %v", err)
		os.Exit(1)
	}
}

package main

import (
	"os"

	"dialtone/dev/plugins/earth/cli"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	if err := cli.Run(os.Args[1:]); err != nil {
		logs.Error("earth error: %v", err)
		os.Exit(1)
	}
}

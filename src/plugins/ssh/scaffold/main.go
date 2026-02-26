package main

import (
	"os"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"dialtone/dev/plugins/ssh/src_v1/cli"
)

func main() {
	logs.SetOutput(os.Stdout)
	if err := cli.Run(os.Args[1:]); err != nil {
		logs.Error("ssh error: %v", err)
		os.Exit(1)
	}
}

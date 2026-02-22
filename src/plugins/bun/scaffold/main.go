package main

import (
	"os"

	"dialtone/dev/plugins/bun/cli"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	cli.RunBun(os.Args[1:])
}

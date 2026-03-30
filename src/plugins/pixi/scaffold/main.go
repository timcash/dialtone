package main

import (
	"os"

	"dialtone/dev/plugins/pixi/cli"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	cli.RunPixi(os.Args[1:])
}

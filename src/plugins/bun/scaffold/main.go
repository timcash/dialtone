package main

import (
	"dialtone/dev/plugins/bun/cli"
	"os"
)

func main() {
	cli.RunBun(os.Args[1:])
}

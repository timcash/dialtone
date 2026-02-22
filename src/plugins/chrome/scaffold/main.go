package main

import (
	"os"

	chrome_cli "dialtone/dev/plugins/chrome/cli"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	chrome_cli.RunChrome(os.Args[1:])
}

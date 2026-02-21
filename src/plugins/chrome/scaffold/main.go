package main

import (
	"os"

	chrome_cli "dialtone/dev/plugins/chrome/cli"
)

func main() {
	chrome_cli.RunChrome(os.Args[1:])
}

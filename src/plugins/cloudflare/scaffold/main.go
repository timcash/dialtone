package main

import (
	"os"

	cloudflare_cli "dialtone/dev/plugins/cloudflare/cli"
)

func main() {
	cloudflare_cli.RunCloudflare(os.Args[1:])
}


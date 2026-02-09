package main

import (
	"log"
	"dialtone/cli/src/plugins/nix/src_v1"
)

func main() {
	plugin := nix.NewNixPlugin(":8080")
	if err := plugin.Start(); err != nil {
		log.Fatalf("Failed to start plugin: %v", err)
	}
}
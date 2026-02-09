package main

import (
	"dialtone/cli/src/plugins/nix/src_v1"
	"log"
)

func main() {
	plugin := nix.NewNixPlugin("0.0.0.0:8080")
	if err := plugin.Start(); err != nil {
		log.Fatalf("Failed to start plugin: %v", err)
	}
}

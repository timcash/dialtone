package main

import (
	"dialtone/cli/src/plugins/wsl/src_v1"
	"log"
)

func main() {
	plugin := wsl.NewWslPlugin("0.0.0.0:8080")
	if err := plugin.Start(); err != nil {
		log.Fatalf("Failed to start plugin: %v", err)
	}
}

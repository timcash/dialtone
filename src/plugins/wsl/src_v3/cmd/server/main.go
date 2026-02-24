package main

import (
	wslv3 "dialtone/dev/plugins/wsl/src_v3/go"
	"log"
)

func main() {
	plugin := wslv3.NewWslPlugin("0.0.0.0:8080")
	if err := plugin.Start(); err != nil {
		log.Fatalf("Failed to start plugin: %v", err)
	}
}

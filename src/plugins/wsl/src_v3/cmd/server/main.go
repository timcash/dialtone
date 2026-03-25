package main

import (
	wslv3 "dialtone/dev/plugins/wsl/src_v3/go"
	"log"
	"os"
	"strings"
)

func main() {
	addr := strings.TrimSpace(os.Getenv("ADDR"))
	if addr == "" {
		port := strings.TrimSpace(os.Getenv("PORT"))
		if port == "" {
			port = "8080"
		}
		addr = "0.0.0.0:" + port
	}
	plugin := wslv3.NewWslPlugin(addr)
	if err := plugin.Start(); err != nil {
		log.Fatalf("Failed to start plugin: %v", err)
	}
}

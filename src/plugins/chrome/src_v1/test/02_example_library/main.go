package main

import (
	"fmt"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
)

func main() {
	path := chrome.FindChromePath()
	if path == "" {
		fmt.Println("library-example: chrome path not found")
		return
	}

	meta := chrome.BuildSessionMetadata(&chrome.Session{
		PID:          1234,
		Port:         9222,
		WebSocketURL: "ws://127.0.0.1:9222/devtools/browser/example",
		IsNew:        true,
	})
	if meta == nil || meta.DebugURL == "" {
		panic("library-example: expected metadata with debug url")
	}

	fmt.Printf("library-example: ok path=%s debug_url=%s\n", path, meta.DebugURL)
}

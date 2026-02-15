package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	port := "8080"
	cwd, _ := os.Getwd()
	uiPath := filepath.Join(cwd, "ui", "dist")
	if _, err := os.Stat(uiPath); err != nil {
		// When launched via "./dialtone.sh cloudflare serve src_v1", cwd is repo root.
		uiPath = filepath.Join(cwd, "src", "plugins", "cloudflare", "src_v1", "ui", "dist")
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		rel := r.URL.Path
		if len(rel) > 0 && rel[0] == '/' {
			rel = rel[1:]
		}
		path := filepath.Join(uiPath, rel)
		if r.URL.Path == "/" {
			path = filepath.Join(uiPath, "index.html")
		}
		if _, err := os.Stat(path); err != nil {
			path = filepath.Join(uiPath, "index.html")
		}
		http.ServeFile(w, r, path)
	})

	fmt.Printf("Cloudflare Server starting on http://localhost:%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		os.Exit(1)
	}
}

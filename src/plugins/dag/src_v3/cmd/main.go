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
		uiPath = filepath.Join(cwd, "src", "plugins", "dag", "src_v3", "ui", "dist")
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

	fmt.Printf("DAG Server starting on http://localhost:%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		os.Exit(1)
	}
}

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

	http.HandleFunc("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(uiPath, r.URL.Path)
		if r.URL.Path == "/" {
			path = filepath.Join(uiPath, "index.html")
		}
		if _, err := os.Stat(path); err == nil {
			http.ServeFile(w, r, path)
			return
		}
		http.ServeFile(w, r, filepath.Join(uiPath, "index.html"))
	})

	fmt.Printf("DAG Server starting on http://localhost:%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		os.Exit(1)
	}
}

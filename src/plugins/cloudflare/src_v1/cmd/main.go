package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	cloudflarev1 "dialtone/dev/plugins/cloudflare/src_v1/go"
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	port := strings.TrimSpace(configv1.LookupEnvString("CLOUDFLARE_PORT"))
	if port == "" {
		port = "8080"
	}
	paths, _ := cloudflarev1.ResolvePaths("", "src_v1")
	cwd, _ := os.Getwd()
	uiPath := filepath.Join(cwd, "ui", "dist")
	if _, err := os.Stat(uiPath); err != nil && paths.Runtime.RepoRoot != "" {
		// When launched via "./dialtone.sh cloudflare serve src_v1", cwd is repo root.
		uiPath = paths.Preset.UIDist
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

	logs.Info("Cloudflare Server starting on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		logs.Error("Error starting server: %v", err)
		os.Exit(1)
	}
}

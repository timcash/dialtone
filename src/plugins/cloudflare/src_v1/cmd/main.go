package main

import (
	"flag"
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
	fs := flag.NewFlagSet("cloudflare-src-v1-http", flag.ContinueOnError)
	port := fs.String("port", strings.TrimSpace(configv1.LookupEnvString("CLOUDFLARE_PORT")), "HTTP port for the local Cloudflare UI server")
	if err := fs.Parse(os.Args[1:]); err != nil {
		logs.Error("cloudflare serve parse failed: %v", err)
		os.Exit(1)
	}
	resolvedPort := strings.TrimSpace(*port)
	if resolvedPort == "" && len(fs.Args()) > 0 {
		resolvedPort = strings.TrimSpace(fs.Args()[0])
	}
	portValue := resolvedPort
	if portValue == "" {
		portValue = "8080"
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

	logs.Info("Cloudflare Server starting on http://localhost:%s", portValue)
	if err := http.ListenAndServe(":"+portValue, nil); err != nil {
		logs.Error("Error starting server: %v", err)
		os.Exit(1)
	}
}

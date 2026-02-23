package main

import (
	"embed"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"tailscale.com/tsnet"
)

//go:embed *
var content embed.FS

func main() {
	logs.SetOutput(os.Stdout)
	hostname := flag.String("hostname", strings.TrimSpace(os.Getenv("ROBOT_SLEEP_HOSTNAME")), "Tailscale hostname")
	stateDir := flag.String("state-dir", "", "Directory to store Tailscale state")
	flag.Parse()

	if *hostname == "" {
		if h, err := os.Hostname(); err == nil && strings.TrimSpace(h) != "" {
			*hostname = h
		} else {
			*hostname = "dialtone-sleep"
		}
	}
	if *stateDir == "" {
		home, _ := os.UserHomeDir()
		*stateDir = filepath.Join(home, ".config", "dialtone")
	}

	s := &tsnet.Server{
		Hostname: *hostname,
		Dir:      *stateDir,
		AuthKey:  os.Getenv("TS_AUTHKEY"),             // In case we need to re-auth (unlikely if state exists)
		Logf:     func(format string, args ...any) {}, // Quiet logs
	}
	defer s.Close()

	// Listen on Tailscale port 80
	ln, err := s.Listen("tcp", ":80")
	if err != nil {
		logs.Fatal("Failed to listen on Tailscale: %v", err)
	}

	// Also listen on local 8080 for LAN/debug
	go http.ListenAndServe(":8080", http.HandlerFunc(handler))

	logs.Info("Sleep server running on %s (Tailscale:80, Local:8080)", *hostname)

	srv := &http.Server{Handler: http.HandlerFunc(handler)}

	go func() {
		if err := srv.Serve(ln); err != nil {
			logs.Fatal("Serve failed: %v", err)
		}
	}()

	// Wait for signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}

func handler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}

	// Remove leading slash for FS lookup
	fsPath := path
	if len(fsPath) > 0 && fsPath[0] == '/' {
		fsPath = fsPath[1:]
	}

	data, err := content.ReadFile(fsPath)
	if err != nil {
		// Fallback to index.html for SPA-like behavior or unknown files
		data, _ = content.ReadFile("index.html")
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
		return
	}

	switch filepath.Ext(path) {
	case ".html":
		w.Header().Set("Content-Type", "text/html")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript")
	case ".json":
		w.Header().Set("Content-Type", "application/json")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	}

	w.Write(data)
}

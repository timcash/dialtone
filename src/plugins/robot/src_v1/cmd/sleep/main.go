package main

import (
	"embed"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"tailscale.com/tsnet"
)

//go:embed index.html
var content embed.FS

func main() {
	hostname := flag.String("hostname", os.Getenv("DIALTONE_HOSTNAME"), "Tailscale hostname")
	stateDir := flag.String("state-dir", "", "Directory to store Tailscale state")
	flag.Parse()

	if *hostname == "" {
		*hostname = "dialtone-sleep"
	}
	if *stateDir == "" {
		home, _ := os.UserHomeDir()
		*stateDir = filepath.Join(home, ".config", "dialtone")
	}

	s := &tsnet.Server{
		Hostname: *hostname,
		Dir:      *stateDir,
		AuthKey:  os.Getenv("TS_AUTHKEY"), // In case we need to re-auth (unlikely if state exists)
		Logf:     func(format string, args ...any) {}, // Quiet logs
	}
	defer s.Close()

	// Listen on Tailscale port 80
	ln, err := s.Listen("tcp", ":80")
	if err != nil {
		log.Fatalf("Failed to listen on Tailscale: %v", err)
	}

	// Also listen on local 8080 for LAN/debug
	go http.ListenAndServe(":8080", http.HandlerFunc(handler))

	log.Printf("Sleep server running on %s (Tailscale:80, Local:8080)\n", *hostname)

	srv := &http.Server{Handler: http.HandlerFunc(handler)}
	
	go func() {
		if err := srv.Serve(ln); err != nil {
			log.Fatalf("Serve failed: %v", err)
		}
	}()

	// Wait for signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}

func handler(w http.ResponseWriter, r *http.Request) {
	data, _ := content.ReadFile("index.html")
	w.Header().Set("Content-Type", "text/html")
	w.Write(data)
}

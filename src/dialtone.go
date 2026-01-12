package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"tailscale.com/tsnet"
)

func main() {
	// Command-line flags
	hostname := flag.String("hostname", "nats", "Tailscale hostname for this NATS server")
	natsPort := flag.Int("port", 4222, "NATS port to listen on (both local and Tailscale)")
	stateDir := flag.String("state-dir", "", "Directory to store Tailscale state (default: ~/.config/dialtone)")
	ephemeral := flag.Bool("ephemeral", false, "Register as ephemeral node (auto-cleanup on disconnect)")
	localOnly := flag.Bool("local-only", false, "Run without Tailscale (local NATS only)")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	// Determine state directory
	if *stateDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to get home directory: %v", err)
		}
		*stateDir = filepath.Join(homeDir, ".config", "dialtone")
	}

	if *localOnly {
		runLocalOnly(*natsPort, *verbose)
		return
	}

	runWithTailscale(*hostname, *natsPort, *stateDir, *ephemeral, *verbose)
}

// runLocalOnly starts NATS without Tailscale (original behavior)
func runLocalOnly(port int, verbose bool) {
	ns := startNATSServer("0.0.0.0", port, verbose)
	defer ns.Shutdown()

	log.Printf("NATS server started on port %d (local only)", port)

	waitForShutdown()
	log.Printf("Shutting down NATS server...")
}

// runWithTailscale starts NATS exposed via Tailscale
func runWithTailscale(hostname string, port int, stateDir string, ephemeral, verbose bool) {
	// Ensure state directory exists
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		log.Fatalf("Failed to create state directory: %v", err)
	}

	// Configure tsnet server
	ts := &tsnet.Server{
		Hostname:  hostname,
		Dir:       stateDir,
		Ephemeral: ephemeral,
		UserLogf:  log.Printf, // Auth URLs and user-facing messages
	}

	if verbose {
		ts.Logf = log.Printf
	}

	// Print auth instructions for headless scenarios
	printAuthInstructions()

	// Start tsnet and wait for connection
	log.Printf("Connecting to Tailscale...")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	status, err := ts.Up(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to Tailscale: %v", err)
	}
	defer ts.Close()

	// Log connection info
	log.Printf("Connected to Tailscale as %s", hostname)
	for _, ip := range status.TailscaleIPs {
		log.Printf("  Tailscale IP: %s", ip)
	}

	// Start NATS on localhost only (not directly exposed)
	localNATSPort := port + 10000 // Use offset port internally
	ns := startNATSServer("127.0.0.1", localNATSPort, verbose)
	defer ns.Shutdown()

	// Listen on Tailscale network
	ln, err := ts.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Failed to listen on Tailscale: %v", err)
	}
	defer ln.Close()

	log.Printf("NATS server available on Tailscale at %s:%d", hostname, port)
	log.Printf("Connect using: nats://%s:%d", hostname, port)

	// Start proxy to forward Tailscale connections to local NATS
	go proxyListener(ln, fmt.Sprintf("127.0.0.1:%d", localNATSPort))

	waitForShutdown()
	log.Printf("Shutting down...")
}

// startNATSServer creates and starts an embedded NATS server
func startNATSServer(host string, port int, verbose bool) *server.Server {
	opts := &server.Options{
		Host:  host,
		Port:  port,
		Debug: verbose,
		Trace: verbose,
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		log.Fatalf("Failed to create NATS server: %v", err)
	}

	// Configure logging if verbose
	if verbose {
		ns.ConfigureLogger()
	}

	go ns.Start()

	if !ns.ReadyForConnections(10 * time.Second) {
		log.Fatalf("NATS server failed to start")
	}

	return ns
}

// proxyListener accepts connections and proxies them to the target address
func proxyListener(ln net.Listener, targetAddr string) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			// Listener closed
			return
		}
		go proxyConnection(conn, targetAddr)
	}
}

// proxyConnection proxies data between source and destination
func proxyConnection(src net.Conn, targetAddr string) {
	defer src.Close()

	dst, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Printf("Failed to connect to NATS backend: %v", err)
		return
	}
	defer dst.Close()

	// Bidirectional copy
	done := make(chan struct{}, 2)
	go func() {
		io.Copy(dst, src)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(src, dst)
		done <- struct{}{}
	}()

	// Wait for either direction to complete
	<-done
}

// printAuthInstructions prints instructions for headless authentication
func printAuthInstructions() {
	fmt.Print(`
=== Tailscale Authentication ===

For headless/remote authentication (SSH into a server without UI):

1. Generate an auth key at: https://login.tailscale.com/admin/settings/keys
   - Create a reusable key for multiple deployments
   - Or a single-use key for one-time setup

2. Set the TS_AUTHKEY environment variable before running:

   Linux/macOS:
     export TS_AUTHKEY="tskey-auth-xxxxx"
     ./dialtone

   Windows:
     set TS_AUTHKEY=tskey-auth-xxxxx
     dialtone.exe

3. For ephemeral nodes (auto-cleanup when disconnected):
     ./dialtone -ephemeral

If no auth key is set, a login URL will be printed below.
Visit that URL to authenticate this device.

========================================
`)
}

// waitForShutdown blocks until SIGINT or SIGTERM is received
func waitForShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}

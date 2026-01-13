package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"tailscale.com/client/tailscale"
	"tailscale.com/tsnet"
)

func main() {
	// Command-line flags
	hostname := flag.String("hostname", "nats", "Tailscale hostname for this NATS server")
	natsPort := flag.Int("port", 4222, "NATS port to listen on (both local and Tailscale)")
	webPort := flag.Int("web-port", 80, "Web dashboard port")
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

	runWithTailscale(*hostname, *natsPort, *webPort, *stateDir, *ephemeral, *verbose)
}

// runLocalOnly starts NATS without Tailscale (original behavior)
func runLocalOnly(port int, verbose bool) {
	ns := startNATSServer("0.0.0.0", port, verbose)
	defer ns.Shutdown()

	log.Printf("NATS server started on port %d (local only)", port)

	waitForShutdown()
	log.Printf("Shutting down NATS server...")
}

// Global start time for uptime calculation
var startTime = time.Now()

//go:embed index.html
var indexHTML embed.FS

var tmpl = template.Must(template.ParseFS(indexHTML, "index.html"))

type TemplateData struct {
	Hostname    string
	Uptime      string
	OS          string
	Arch        string
	Caller      string
	NATSPort    int
	Connections int
	InMsgs      int64
	OutMsgs     int64
	InBytes     string
	OutBytes    string
	IPs         string
	WebPort     int
}

// runWithTailscale starts NATS exposed via Tailscale
func runWithTailscale(hostname string, port, webPort int, stateDir string, ephemeral, verbose bool) {
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

	// Listen on Tailscale network for NATS
	natsLn, err := ts.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Failed to listen on Tailscale for NATS: %v", err)
	}
	defer natsLn.Close()

	log.Printf("NATS server available on Tailscale at %s:%d", hostname, port)
	log.Printf("Connect using: nats://%s:%d", hostname, port)

	// Start proxy to forward Tailscale connections to local NATS
	go proxyListener(natsLn, fmt.Sprintf("127.0.0.1:%d", localNATSPort))

	// Start web server on Tailscale
	webLn, err := ts.Listen("tcp", fmt.Sprintf(":%d", webPort))
	if err != nil {
		log.Fatalf("Failed to listen on Tailscale for web: %v", err)
	}
	defer webLn.Close()

	// Get LocalClient for identifying callers
	lc, err := ts.LocalClient()
	if err != nil {
		log.Fatalf("Failed to get LocalClient: %v", err)
	}

	// Create web handler
	webHandler := createWebHandler(hostname, port, webPort, ns, lc, status.TailscaleIPs)

	// Start web server in goroutine
	go func() {
		// Use full DNS name from Tailscale status if available
		displayHostname := hostname
		if status.Self != nil && status.Self.DNSName != "" {
			displayHostname = strings.TrimSuffix(status.Self.DNSName, ".")
		} else {
			// Fallback: try CertDomains
			domains := ts.CertDomains()
			if len(domains) > 0 {
				displayHostname = domains[0]
			}
		}

		fmt.Println("\n=======================================================")
		fmt.Printf("WEB SERVER READY\n")
		// We print the FQDN-based URL if available, otherwise hostname
		fmt.Printf("   URL: http://%s:%d\n", displayHostname, webPort)

		// If we have an IP, print that too as a fallback/direct link
		if len(status.TailscaleIPs) > 0 {
			fmt.Printf("   IP:  http://%s:%d\n", status.TailscaleIPs[0], webPort)
		}
		fmt.Println("=======================================================")

		if err := http.Serve(webLn, webHandler); err != nil {
			log.Printf("Web server error: %v", err)
		}
	}()

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

// createWebHandler creates the HTTP handler for the web dashboard
func createWebHandler(hostname string, natsPort, webPort int, ns *server.Server, lc *tailscale.LocalClient, ips []netip.Addr) http.Handler {
	mux := http.NewServeMux()

	// Main dashboard
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Get caller info
		callerInfo := "Unknown"
		if lc != nil {
			who, err := lc.WhoIs(r.Context(), r.RemoteAddr)
			if err == nil && who.UserProfile != nil {
				callerInfo = who.UserProfile.DisplayName
				if who.Node != nil {
					callerInfo += " (" + who.Node.Name + ")"
				}
			}
		}

		// Get NATS stats
		varz, _ := ns.Varz(nil)
		var connections int
		var inMsgs, outMsgs, inBytes, outBytes int64
		if varz != nil {
			connections = varz.Connections
			inMsgs = varz.InMsgs
			outMsgs = varz.OutMsgs
			inBytes = varz.InBytes
			outBytes = varz.OutBytes
		}

		data := TemplateData{
			Hostname:    hostname,
			Uptime:      formatDuration(time.Since(startTime)),
			OS:          runtime.GOOS,
			Arch:        runtime.GOARCH,
			Caller:      callerInfo,
			NATSPort:    natsPort,
			Connections: connections,
			InMsgs:      inMsgs,
			OutMsgs:     outMsgs,
			InBytes:     formatBytes(inBytes),
			OutBytes:    formatBytes(outBytes),
			IPs:         formatIPs(ips),
			WebPort:     webPort,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, data); err != nil {
			log.Printf("Template execution error: %v", err)
		}
	})

	// JSON status API
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		varz, _ := ns.Varz(nil)
		var connections int
		var inMsgs, outMsgs, inBytes, outBytes int64
		if varz != nil {
			connections = varz.Connections
			inMsgs = varz.InMsgs
			outMsgs = varz.OutMsgs
			inBytes = varz.InBytes
			outBytes = varz.OutBytes
		}

		status := map[string]any{
			"hostname":      hostname,
			"uptime":        time.Since(startTime).String(),
			"uptime_secs":   time.Since(startTime).Seconds(),
			"platform":      runtime.GOOS,
			"arch":          runtime.GOARCH,
			"tailscale_ips": formatIPs(ips),
			"nats": map[string]any{
				"url":          fmt.Sprintf("nats://%s:%d", hostname, natsPort),
				"connections":  connections,
				"messages_in":  inMsgs,
				"messages_out": outMsgs,
				"bytes_in":     inBytes,
				"bytes_out":    outBytes,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	// NATS varz - forward the full varz
	mux.HandleFunc("/api/varz", func(w http.ResponseWriter, r *http.Request) {
		varz, err := ns.Varz(nil)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(varz)
	})

	return mux
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// formatBytes formats bytes in a human-readable way
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// formatIPs formats IP addresses for display
func formatIPs(ips []netip.Addr) string {
	if len(ips) == 0 {
		return "none"
	}
	result := ""
	for i, ip := range ips {
		if i > 0 {
			result += ", "
		}
		result += ip.String()
	}
	return result
}

package dialtone

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
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

	"github.com/coder/websocket"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/common"
	"tailscale.com/client/tailscale"
	"tailscale.com/tsnet"
)

var mavlinkPubChan = make(chan mavlinkNatsMsg, 100)

type mavlinkNatsMsg struct {
	Subject string
	Data    []byte
}

func Execute() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	// Load configuration before parsing any flags or running commands
	LoadConfig()

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "build":
		RunBuild(args)
	case "full-build":
		RunBuild(append([]string{"-full"}, args...))
	case "deploy":
		RunDeploy(args)
	case "ssh":
		RunSSH(args)
	case "provision":
		RunProvision(args)
	case "logs":
		runLogs(args)
	case "start":
		runStart(args)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: dialtone <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  build       Build for ARM64 using Podman")
	fmt.Println("  full-build  Build Web UI, local CLI, and ARM64 binary")
	fmt.Println("  deploy      Deploy to remote robot")
	fmt.Println("  ssh         SSH tools (upload, download, cmd)")
	fmt.Println("  provision   Generate Tailscale Auth Key")
	fmt.Println("  logs        Tail remote logs")
	fmt.Println("  start       Start the NATS and Web server")
}

func runStart(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	hostname := fs.String("hostname", "dialtone-1", "Tailscale hostname for this NATS server")
	natsPort := fs.Int("port", 4222, "NATS port to listen on (both local and Tailscale)")
	webPort := fs.Int("web-port", 80, "Web dashboard port")
	stateDir := fs.String("state-dir", "", "Directory to store Tailscale state (default: ~/.config/dialtone)")
	ephemeral := fs.Bool("ephemeral", false, "Register as ephemeral node (auto-cleanup on disconnect)")
	localOnly := fs.Bool("local-only", false, "Run without Tailscale (local NATS only)")
	wsPort := fs.Int("ws-port", 4223, "NATS WebSocket port")
	verbose := fs.Bool("verbose", false, "Enable verbose logging")
	mavlinkAddr := fs.String("mavlink", "", "Mavlink connection string (e.g. serial:/dev/ttyAMA0:57600 or udp:0.0.0.0:14550)")
	fs.Parse(args)

	// Determine state directory
	if *stateDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			LogFatal("Failed to get home directory: %v", err)
		}
		*stateDir = filepath.Join(homeDir, ".config", "dialtone")
	}

	if *localOnly {
		runLocalOnly(*natsPort, *wsPort, *verbose, *mavlinkAddr)
		return
	}

	runWithTailscale(*hostname, *natsPort, *wsPort, *webPort, *stateDir, *ephemeral, *verbose, *mavlinkAddr)
}

// runLocalOnly starts NATS without Tailscale (original behavior)
func runLocalOnly(port, wsPort int, verbose bool, mavlinkAddr string) {
	ns := startNATSServer("0.0.0.0", port, wsPort, verbose)
	defer ns.Shutdown()

	LogInfo("NATS server started on port %d (local only)", port)

	// Start Mavlink service if requested
	if mavlinkAddr != "" {
		go startMavlink(mavlinkAddr, port)
	}

	// Start NATS publisher loop for Mavlink
	startNatsPublisher(port)

	waitForShutdown()
	LogInfo("Shutting down NATS server...")
}

// Global start time for uptime calculation
var startTime = time.Now()

//go:embed all:web_build
var webFS embed.FS

// runWithTailscale starts NATS exposed via Tailscale
func runWithTailscale(hostname string, port, wsPort, webPort int, stateDir string, ephemeral, verbose bool, mavlinkAddr string) {
	// Ensure state directory exists
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		LogFatal("Failed to create state directory: %v", err)
	}

	// Configure tsnet server
	ts := &tsnet.Server{
		Hostname:  hostname,
		Dir:       stateDir,
		Ephemeral: ephemeral,
		UserLogf:  LogInfo, // Auth URLs and user-facing messages
	}

	if verbose {
		ts.Logf = LogInfo
	}

	// Validate required environment variables
	if os.Getenv("TS_AUTHKEY") == "" {
		LogFatal("ERROR: TS_AUTHKEY environment variable is not set. A Tailscale auth key is required for headless operation.")
	}

	// Print auth instructions for headless scenarios
	printAuthInstructions()

	// Start tsnet and wait for connection
	LogInfo("Connecting to Tailscale...")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	status, err := ts.Up(ctx)
	if err != nil {
		LogFatal("Failed to connect to Tailscale: %v", err)
	}
	defer ts.Close()

	// Log connection info
	LogInfo("Connected to Tailscale as %s", hostname)
	for _, ip := range status.TailscaleIPs {
		LogInfo("  Tailscale IP: %s", ip)
	}

	// Start NATS on localhost only (not directly exposed)
	localNATSPort := port + 10000 // Use offset port internally
	localWSPort := wsPort + 10000
	ns := startNATSServer("127.0.0.1", localNATSPort, localWSPort, verbose)
	defer ns.Shutdown()

	// Start Mavlink service if requested (connect to LOCAL NATS port)
	if mavlinkAddr != "" {
		go startMavlink(mavlinkAddr, localNATSPort)
	}

	// Start NATS publisher loop for Mavlink
	startNatsPublisher(localNATSPort)

	// Listen on Tailscale network for NATS
	natsLn, err := ts.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		LogFatal("Failed to listen on Tailscale for NATS: %v", err)
	}
	defer natsLn.Close()

	// Listen on Tailscale network for NATS WebSockets
	wsLn, err := ts.Listen("tcp", fmt.Sprintf(":%d", wsPort))
	if err != nil {
		LogFatal("Failed to listen on Tailscale for NATS WS: %v", err)
	}
	defer wsLn.Close()

	LogInfo("NATS server available on Tailscale at %s:%d", hostname, port)
	LogInfo("NATS WebSockets available on Tailscale at %s:%d", hostname, wsPort)
	LogInfo("Connect using: nats://%s:%d or ws://%s:%d", hostname, port, hostname, wsPort)

	// Start camera in background if available
	go func() {
		cameras, err := ListCameras()
		if err != nil {
			LogInfo("CAMERA: Failed to list devices: %v", err)
			return
		}
		if len(cameras) > 0 {
			LogInfo("CAMERA: Found %d devices, using %s", len(cameras), cameras[0].Device)
			if err := StartCamera(context.Background(), cameras[0].Device); err != nil {
				LogInfo("CAMERA: Failed to start %s: %v", cameras[0].Device, err)
			} else {
				LogInfo("CAMERA: %s started successfully", cameras[0].Device)
			}
		} else {
			LogInfo("CAMERA: No devices found")
		}
	}()

	// Start proxies to forward Tailscale connections to local NATS
	go ProxyListener(natsLn, fmt.Sprintf("127.0.0.1:%d", localNATSPort))
	go ProxyListener(wsLn, fmt.Sprintf("127.0.0.1:%d", localWSPort))

	// Start web server on Tailscale
	webLn, err := ts.Listen("tcp", fmt.Sprintf(":%d", webPort))
	if err != nil {
		LogFatal("Failed to listen on Tailscale for web: %v", err)
	}
	defer webLn.Close()

	// Get LocalClient for identifying callers
	lc, err := ts.LocalClient()
	if err != nil {
		LogFatal("Failed to get LocalClient: %v", err)
	}

	// Create web handler
	webHandler := CreateWebHandler(hostname, port, wsPort, webPort, ns, lc, status.TailscaleIPs)

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

		LogInfo("=======================================================")
		LogInfo("WEB SERVER READY")
		LogInfo("   URL: http://%s:%d", displayHostname, webPort)

		if len(status.TailscaleIPs) > 0 {
			LogInfo("   IP:  http://%s:%d", status.TailscaleIPs[0], webPort)
		}
		LogInfo("=======================================================")

		if err := http.Serve(webLn, webHandler); err != nil {
			LogInfo("Web server error: %v", err)
		}
	}()

	waitForShutdown()
	LogInfo("Shutting down...")
}

// startNATSServer creates and starts an embedded NATS server
func startNATSServer(host string, port, wsPort int, verbose bool) *server.Server {
	opts := &server.Options{
		Host:  host,
		Port:  port,
		Debug: verbose,
		Trace: verbose,
		Websocket: server.WebsocketOpts{
			Host:  host,
			Port:  wsPort,
			NoTLS: true, // Internal/Tailscale networking is trusted
		},
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		LogFatal("Failed to create NATS server: %v", err)
	}

	// Configure logging if verbose
	if verbose {
		ns.SetLogger(GetNATSLogger(), verbose, verbose)
	}

	go ns.Start()

	if !ns.ReadyForConnections(10 * time.Second) {
		LogFatal("NATS server failed to start")
	}

	return ns
}

// ProxyListener accepts connections and proxies them to the target address
func ProxyListener(ln net.Listener, targetAddr string) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			// Listener closed
			return
		}
		go ProxyConnection(conn, targetAddr)
	}
}

// ProxyConnection proxies data between source and destination
func ProxyConnection(src net.Conn, targetAddr string) {
	defer src.Close()

	dst, err := net.Dial("tcp", targetAddr)
	if err != nil {
		LogInfo("Failed to connect to NATS backend: %v", err)
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

func startMavlink(endpoint string, natsPort int) {
	LogInfo("Starting Mavlink Service on %s...", endpoint)

	config := MavlinkConfig{
		Endpoint: endpoint,
		Callback: func(evt *MavlinkEvent) {
			var subject string
			var data []byte
			var err error

			switch evt.Type {
			case "HEARTBEAT":
				if msg, ok := evt.Data.(*common.MessageHeartbeat); ok {
					subject = "mavlink.heartbeat"
					data, err = json.Marshal(map[string]any{
						"type":          "HEARTBEAT",
						"mav_type":      msg.Type,
						"autopilot":     msg.Autopilot,
						"base_mode":     msg.BaseMode,
						"custom_mode":   msg.CustomMode,
						"system_status": msg.SystemStatus,
						"timestamp":     time.Now().Unix(),
					})
				}
			case "COMMAND_ACK":
				if msg, ok := evt.Data.(*common.MessageCommandAck); ok {
					subject = "mavlink.ack"
					data, err = json.Marshal(map[string]any{
						"command": msg.Command,
						"result":  msg.Result,
					})
				}
			case "STATUSTEXT":
				if msg, ok := evt.Data.(*common.MessageStatustext); ok {
					subject = "mavlink.statustext"
					data, err = json.Marshal(map[string]any{
						"severity": msg.Severity,
						"text":     string(msg.Text[:]), // Convert char array to string
					})
				}
			}

			if err == nil && subject != "" {
				select {
				case mavlinkPubChan <- mavlinkNatsMsg{Subject: subject, Data: data}:
				default:
					// Drop message if channel full
				}
			}
		},
	}

	svc, err := NewMavlinkService(config)
	if err != nil {
		LogFatal("Failed to create Mavlink service: %v", err)
	}

	// Connect to NATS for subscribing to commands
	go func() {
		// Wait for NATS to start
		time.Sleep(3 * time.Second)
		
		nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", natsPort))
		if err != nil {
			LogInfo("MAVLINK: Failed to connect to NATS for commands: %v", err)
			return
		}
		
		LogInfo("MAVLINK: Subscribed to rover.command")
		
		nc.Subscribe("rover.command", func(m *nats.Msg) {
			var cmd map[string]interface{}
			if err := json.Unmarshal(m.Data, &cmd); err != nil {
				return
			}
			
			typeStr, _ := cmd["type"].(string)
			if typeStr == "" {
				typeStr, _ = cmd["cmd"].(string)
			}
			
			switch typeStr {
			case "arm":
				svc.Arm()
			case "disarm":
				svc.Disarm()
			}
		})
	}()

	go svc.Start()
}

// startNatsPublisher connects to the local NATS server and publishes messages from the channel
func startNatsPublisher(port int) {
	go func() {
		// Wait for NATS to start
		time.Sleep(2 * time.Second)

		nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", port))
		if err != nil {
			LogInfo("Failed to connect to NATS for publishing: %v", err)
			return
		}
		defer nc.Close()

		LogInfo("Mavlink NATS Publisher connected")

		for msg := range mavlinkPubChan {
			if err := nc.Publish(msg.Subject, msg.Data); err != nil {
				LogInfo("Error publishing to NATS: %v", err)
			}
		}
	}()
}

// waitForShutdown blocks until SIGINT or SIGTERM is received
func waitForShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-
c
}

// CreateWebHandler creates the HTTP handler for the unified web dashboard
func CreateWebHandler(hostname string, natsPort, wsPort, webPort int, ns *server.Server, lc *tailscale.LocalClient, ips []netip.Addr) http.Handler {
	mux := http.NewServeMux()

	// 1. JSON init API for the frontend
	mux.HandleFunc("/api/init", func(w http.ResponseWriter, r *http.Request) {
		data := map[string]interface{}{
			"hostname":  hostname,
			"nats_port": natsPort,
			"ws_port":   wsPort,
			"web_port":  webPort,
			"ips":       ips,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	})

	// 2. JSON status API
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

	// 3. Cameras API
	mux.HandleFunc("/api/cameras", func(w http.ResponseWriter, r *http.Request) {
		cameras, err := ListCameras()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cameras)
	})

	// 4. Video Stream MJPEG
	mux.HandleFunc("/stream", StreamHandler)

	// 5. WebSocket for real-time updates (legacy dashboard)
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			LogInfo("WebSocket accept error: %v", err)
			return
		}
		defer c.Close(websocket.StatusInternalError, "closing")

		ctx := r.Context()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

						for {

							select {

										case <-ctx.Done():

											return

										case <-ticker.C:

							

				

		
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

				callerInfo := "Unknown"
				if lc != nil {
					who, err := lc.WhoIs(ctx, r.RemoteAddr)
					if err == nil && who.UserProfile != nil {
						callerInfo = who.UserProfile.DisplayName
						if who.Node != nil {
							callerInfo += " (" + who.Node.Name + ")"
						}
					}
				}

				stats := map[string]any{
					"uptime":      formatDuration(time.Since(startTime)),
					"os":          runtime.GOOS,
					"arch":        runtime.GOARCH,
					"caller":      callerInfo,
					"connections": connections,
					"in_msgs":     inMsgs,
					"out_msgs":    outMsgs,
					"in_bytes":    formatBytes(inBytes),
					"out_bytes":   formatBytes(outBytes),
				}

				data, _ := json.Marshal(stats)
				if err := c.Write(ctx, websocket.MessageText, data); err != nil {
					return
				}
			}
		}
	})

	// 6. Static Asset Serving (Embedded)
	subFS, err := fs.Sub(webFS, "web_build")
	if err != nil {
		LogInfo("Error accessing sub-filesystem: %v", err)
	}

	// Fallback/SPA logic for embedded web assets
	LogInfo("Using embedded web assets")
	staticHandler := http.FileServer(http.FS(subFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// If it's a known static file, serve it
		f, err := subFS.Open(strings.TrimPrefix(r.URL.Path, "/"))
		if err == nil {
			f.Close()
			staticHandler.ServeHTTP(w, r)
			return
		}
		// Otherwise serve index.html for SPA
		http.ServeFileFS(w, r, subFS, "index.html")
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
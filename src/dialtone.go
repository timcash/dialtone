package dialtone

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"dialtone/cli/src/core/config"
	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/mock"
	"dialtone/cli/src/core/util"
	"dialtone/cli/src/core/web"
	ai_app "dialtone/cli/src/plugins/ai/app"
	camera "dialtone/cli/src/plugins/camera/app"
	dag_cli "dialtone/cli/src/plugins/dag/cli"
	mavlink "dialtone/cli/src/plugins/mavlink/app"
	nix_cli "dialtone/cli/src/plugins/nix/cli"
	template_cli "dialtone/cli/src/plugins/template/cli"

	"github.com/bluenviron/gomavlib/v3/pkg/dialects/common"
	"github.com/coder/websocket"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"tailscale.com/client/tailscale"
	"tailscale.com/tsnet"
)

// (Moved MavlinkPubChan to src/core/mock)

func Execute() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	// Load configuration before parsing any flags or running commands
	config.LoadConfig()

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "start":
		runStart(args)
	case "vpn":
		runVPN(args)
	case "nix":
		if err := nix_cli.Run(args); err != nil {
			fmt.Printf("Nix command error: %v\n", err)
			os.Exit(1)
		}
	case "dag":
		if err := dag_cli.Run(args); err != nil {
			fmt.Printf("DAG command error: %v\n", err)
			os.Exit(1)
		}
	case "template":
		if err := template_cli.Run(args); err != nil {
			fmt.Printf("Template command error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: dialtone <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  start         Start the NATS and Web server")
	fmt.Println("  vpn           Start in simple VPN mode (tsnet only)")
	fmt.Println("  nix           Nix plugin commands (e.g. nix smoke src_v1)")
	fmt.Println("  dag           DAG plugin commands (e.g. dag test src_v3)")
	fmt.Println("  template      Template plugin commands (e.g. template test src_v3)")
}

func runStart(args []string) {
	// Check if running under systemd
	if os.Getenv("INVOCATION_ID") == "" {
		logger.LogInfo("[WARNING] Process is not running as a systemd service. Consider running via systemctl.")
	} else {
		logger.LogInfo("[INFO] Running as systemd service.")
	}

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
	opencode := fs.Bool("opencode", false, "Start opencode AI assistant server")
	useMock := fs.Bool("mock", false, "Use mock telemetry and camera data")
	fs.Parse(args)

	// Determine state directory
	if *stateDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			logger.LogFatal("Failed to get home directory: %v", err)
		}
		*stateDir = filepath.Join(homeDir, ".config", "dialtone")
	}

	// Mock mode forces opencode off
	if *useMock && *opencode {
		logger.LogInfo("Mock mode enabled: Disabling opencode")
		*opencode = false
	}

	if *localOnly {
		runLocalOnly(*natsPort, *wsPort, *webPort, *verbose, *mavlinkAddr, *opencode, *useMock, *hostname)
		return
	}

	runWithTailscale(*hostname, *natsPort, *wsPort, *webPort, *stateDir, *ephemeral, *verbose, *mavlinkAddr, *opencode, *useMock)
}

// runLocalOnly starts NATS without Tailscale (original behavior)
func runLocalOnly(port, wsPort, webPort int, verbose bool, mavlinkAddr string, opencode bool, useMock bool, hostname string) {
	ns := startNATSServer("0.0.0.0", port, wsPort, verbose)
	defer ns.Shutdown()

	logger.LogInfo("NATS server started on port %d (local only)", port)
	verifyNatsWSPort("127.0.0.1", wsPort, "NATS WS (local)")

	// Start Mavlink service if requested
	if useMock {
		go mock.StartMockMavlink(port)
	} else if mavlinkAddr != "" {
		go startMavlink(mavlinkAddr, port)
	}

	// Start opencode if requested
	if opencode {
		go ai_app.RunOpencodeServer(3000) // Default opencode port
	}

	// Start NATS publisher loop for Mavlink
	startNatsPublisher(port)

	// Start Web UI
	// Start Web UI
	webHandler := CreateWebHandler(hostname, port, wsPort, webPort, port, wsPort, ns, nil, nil, useMock)
	localWebAddr := fmt.Sprintf("0.0.0.0:%d", webPort)
	if webPort == 80 {
		localWebAddr = "0.0.0.0:8080" // Use 8080 if 80 is requested but not root
	}

	webServer := &http.Server{
		Addr:    localWebAddr,
		Handler: webHandler,
	}

	go func() {
		logger.LogInfo("Web UI (Local Only): Serving at http://%s", localWebAddr)
		if err := webServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.LogInfo("Local web server error: %v", err)
		}
	}()

	util.WaitForShutdown()
	logger.LogInfo("Shutting down local services...")
}

// Global start time for uptime calculation
var startTime = time.Now()

// Ensure embed is detected
//
//go:embed all:core/web/dist
var webFS embed.FS

// runWithTailscale starts NATS exposed via Tailscale
func runWithTailscale(hostname string, port, wsPort, webPort int, stateDir string, ephemeral, verbose bool, mavlinkAddr string, opencode bool, mock_mode bool) {
	// Ensure state directory exists
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		logger.LogFatal("Failed to create state directory: %v", err)
	}

	// Configure tsnet server
	ts := &tsnet.Server{
		Hostname:  hostname,
		Dir:       stateDir,
		Ephemeral: ephemeral,
		AuthKey:   os.Getenv("TS_AUTHKEY"),
		UserLogf:  logger.LogInfo, // Auth URLs and user-facing messages
	}

	if verbose {
		ts.Logf = logger.LogInfo
	}

	// Pre-flight check for stale MagicDNS entry
	util.CheckStaleHostname(hostname)

	// Validate required environment variables
	if os.Getenv("TS_AUTHKEY") == "" {
		logger.LogFatal("ERROR: TS_AUTHKEY environment variable is not set. A Tailscale auth key is required for headless operation.")
	}

	// Print auth instructions for headless scenarios
	printAuthInstructions()

	// Start tsnet and wait for connection
	logger.LogInfo("Connecting to Tailscale...")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	status, err := ts.Up(ctx)
	if err != nil {
		logger.LogFatal("Failed to connect to Tailscale: %v", err)
	}

	for status == nil || len(status.TailscaleIPs) == 0 {
		logger.LogInfo("Waiting for Tailscale IP (Status: %v)...", status.BackendState)
		time.Sleep(2 * time.Second)
		status, err = ts.Up(ctx)
		if err != nil {
			logger.LogFatal("Failed to connect to Tailscale: %v", err)
		}
		if ctx.Err() != nil {
			logger.LogFatal("Timed out waiting for Tailscale IP")
		}
	}
	defer ts.Close()

	// 1. Connection Logging
	var ips []netip.Addr
	displayHostname := hostname
	ips = status.TailscaleIPs
	if status.Self != nil && status.Self.DNSName != "" {
		displayHostname = strings.TrimSuffix(status.Self.DNSName, ".")
	}

	ipStr := "none"
	if len(ips) > 0 {
		ipStr = ips[0].String()
	}
	logger.LogInfo("TSNet: Connected (IP: %s)", ipStr)
	logger.LogInfo("NATS: Connected")

	// 2. Proxies and Services
	localNATSPort := port + 10000
	localWSPort := wsPort + 10000
	ns := startNATSServer("127.0.0.1", localNATSPort, localWSPort, verbose)
	defer ns.Shutdown()

	verifyNatsWSPort("127.0.0.1", localWSPort, "NATS WS (internal)")

	if mock_mode {
		go mock.StartMockMavlink(localNATSPort)
	} else if mavlinkAddr != "" {
		go startMavlink(mavlinkAddr, localNATSPort)
	}

	// Start opencode if requested
	if opencode {
		go ai_app.RunOpencodeServer(3000)
	}

	startNatsPublisher(localNATSPort)

	natsLn, _ := ts.Listen("tcp", fmt.Sprintf(":%d", port))
	wsLn, _ := ts.Listen("tcp", fmt.Sprintf(":%d", wsPort))
	webLn, _ := ts.Listen("tcp", fmt.Sprintf(":%d", webPort))
	defer natsLn.Close()
	defer wsLn.Close()
	defer webLn.Close()

	if natsLn != nil {
		go util.ProxyListener(natsLn, fmt.Sprintf("127.0.0.1:%d", localNATSPort))
	}
	if wsLn != nil {
		go util.ProxyListener(wsLn, fmt.Sprintf("127.0.0.1:%d", localWSPort))
	}

	// 3. Web Handler and Server
	lc, _ := ts.LocalClient()
	webHandler := CreateWebHandler(hostname, port, wsPort, webPort, localNATSPort, localWSPort, ns, lc, ips, mock_mode)

	// Local listener for testing/dev (use 8080 to avoid permission issues)
	localWebPort := 8080
	if webPort != 80 {
		localWebPort = webPort + 1000 // Offset if 8080 is taken
	}
	localWebAddr := fmt.Sprintf("127.0.0.1:%d", localWebPort)
	localWebLn, err := net.Listen("tcp", localWebAddr)
	if err == nil {
		go func() {
			logger.LogInfo("Web UI (Local): Serving at http://%s", localWebAddr)
			if err := http.Serve(localWebLn, webHandler); err != nil {
				logger.LogInfo("Local web server error: %v", err)
			}
		}()
	} else {
		logger.LogInfo("Warning: Failed to start local web server: %v", err)
	}

	go func() {
		logger.LogInfo("Web UI (Tailscale): Serving at http://%s:%d", displayHostname, webPort)
		time.Sleep(2 * time.Second)
		logger.LogInfo("[SUCCESS] System Operational")
		if err := http.Serve(webLn, webHandler); err != nil {
			logger.LogInfo("Web server error: %v", err)
		}
	}()

	util.WaitForShutdown()
	logger.LogInfo("Shutting down...")
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
		logger.LogFatal("Failed to create NATS server: %v", err)
	}

	// Configure logging if verbose
	if verbose {
		ns.SetLogger(logger.GetNATSLogger(), verbose, verbose)
	}

	go ns.Start()

	if !ns.ReadyForConnections(10 * time.Second) {
		logger.LogFatal("NATS server failed to start")
	}

	return ns
}

func printAuthInstructions() {
	fmt.Print(`
=== Tailscale Authentication ===

For headless/remote authentication (SSH into a server without UI):

1. Generate an auth key at: https://login.tailscale.com/admin/settings/keys
   - Create a reusable key for multiple deployments
   - Or a single-use key for one-time setup

2. Set the TS_AUTHKEY in .env before running:
========================================
`)
}

func startMavlink(endpoint string, natsPort int) {
	logger.LogInfo("Starting Mavlink Service on %s...", endpoint)

	config := mavlink.MavlinkConfig{
		Endpoint: endpoint,
		Callback: func(evt *mavlink.MavlinkEvent) {
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
						"result":  int(msg.Result),
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
			case "GLOBAL_POSITION_INT":
				if msg, ok := evt.Data.(*common.MessageGlobalPositionInt); ok {
					subject = "mavlink.global_position_int"
					data, err = json.Marshal(map[string]any{
						"lat":          float64(msg.Lat) / 1e7,
						"lon":          float64(msg.Lon) / 1e7,
						"alt":          float64(msg.Alt) / 1000.0,
						"relative_alt": float64(msg.RelativeAlt) / 1000.0,
						"vx":           float64(msg.Vx) / 100.0,
						"vy":           float64(msg.Vy) / 100.0,
						"vz":           float64(msg.Vz) / 100.0,
						"hdg":          float64(msg.Hdg) / 100.0,
					})
				}
			case "ATTITUDE":
				if msg, ok := evt.Data.(*common.MessageAttitude); ok {
					subject = "mavlink.attitude"
					data, err = json.Marshal(map[string]any{
						"roll":       msg.Roll,
						"pitch":      msg.Pitch,
						"yaw":        msg.Yaw,
						"rollspeed":  msg.Rollspeed,
						"pitchspeed": msg.Pitchspeed,
						"yawspeed":   msg.Yawspeed,
					})
				}
			}

			if err == nil && subject != "" {
				select {
				case mock.MavlinkPubChan <- mock.MavlinkNatsMsg{Subject: subject, Data: data}:
				default:
					// Drop message if channel full
				}
			}
		},
	}

	svc, err := mavlink.NewMavlinkService(config)
	if err != nil {
		logger.LogFatal("Failed to create Mavlink service: %v", err)
	}

	// Connect to NATS for subscribing to commands
	go func() {
		// Wait for NATS to start
		time.Sleep(3 * time.Second)

		nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", natsPort))
		if err != nil {
			logger.LogInfo("MAVLINK: Failed to connect to NATS for commands: %v", err)
			return
		}

		logger.LogInfo("MAVLINK: Subscribed to rover.command")

		heartbeatLogged := false
		nc.Subscribe("mavlink.heartbeat", func(m *nats.Msg) {
			if !heartbeatLogged {
				logger.LogInfo("MAVLINK: Heartbeat received from flight controller")
				heartbeatLogged = true
			}
		})

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
			case "mode":
				mode, _ := cmd["mode"].(string)
				svc.SetMode(mode)
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
			logger.LogInfo("Failed to connect to NATS for publishing: %v", err)
			return
		}
		defer nc.Close()

		logger.LogInfo("Mavlink NATS Publisher connected")

		for msg := range mock.MavlinkPubChan {
			if err := nc.Publish(msg.Subject, msg.Data); err != nil {
				logger.LogInfo("Error publishing to NATS: %v", err)
			}
		}
	}()
}

// waitForShutdown function is now in src/core/util/signal.go

func verifyNatsWSPort(host string, port int, label string) {
	if port == 0 {
		return
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	if err := waitForTCP(addr, 5*time.Second); err != nil {
		logger.LogFatal("%s not listening on %s: %v", label, addr, err)
	}
	logger.LogInfo("%s listening on %s", label, addr)
}

func waitForTCP(address string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 1*time.Second)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", address)
}

// CreateWebHandler creates the HTTP handler for the unified web dashboard
func CreateWebHandler(hostname string, natsPort, wsPort, webPort, internalNATSPort, internalWSPort int, ns *server.Server, lc *tailscale.LocalClient, ips []netip.Addr, useMock bool) http.Handler {
	mux := http.NewServeMux()

	// In tsnet mode, NATS WS is served on an internal offset port.
	// If the handler was wired with the external port, correct it here.
	if lc != nil && internalWSPort == wsPort {
		internalWSPort = wsPort + 10000
		logger.LogInfo("Adjusted internal NATS WS port to %d for tsnet proxy", internalWSPort)
	}
	logger.LogInfo("NATS WS proxy ports: external=%d internal=%d", wsPort, internalWSPort)

	// 1. JSON init API for the frontend
	mux.HandleFunc("/api/init", func(w http.ResponseWriter, r *http.Request) {
		data := map[string]interface{}{
			"version":   "v1.1.1",
			"hostname":  hostname,
			"nats_port": natsPort,
			"ws_port":   wsPort,
			"ws_path":   "/nats-ws", // Path to the proxied NATS WS
			"web_port":  webPort,
			"ips":       ips,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	})

	// 2. JSON status API
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		var connections int
		var inMsgs, outMsgs, inBytes, outBytes int64

		// Only check NATS vars if server is present
		if ns != nil {
			varz, _ := ns.Varz(nil)
			if varz != nil {
				connections = varz.Connections
				inMsgs = varz.InMsgs
				outMsgs = varz.OutMsgs
				inBytes = varz.InBytes
				outBytes = varz.OutBytes
			}
		}

		status := map[string]any{
			"hostname":      hostname,
			"uptime":        time.Since(startTime).String(),
			"uptime_secs":   time.Since(startTime).Seconds(),
			"platform":      runtime.GOOS,
			"arch":          runtime.GOARCH,
			"tailscale_ips": formatIPs(ips),
			"ws_port":       wsPort,
			"ws_path":       "/nats-ws",
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
		cameras, err := camera.ListCameras()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cameras)
	})

	// 4. Video Stream MJPEG
	if useMock {
		mux.HandleFunc("/stream", mock.MockStreamHandler)
	} else {
		mux.HandleFunc("/stream", camera.StreamHandler)
	}

	// 5. WebSocket for real-time updates (legacy dashboard)
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			logger.LogInfo("WebSocket accept error: %v", err)
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

				var connections int
				var inMsgs, outMsgs, inBytes, outBytes int64

				// Only check NATS vars if server is present
				if ns != nil {
					varz, _ := ns.Varz(nil)
					if varz != nil {
						connections = varz.Connections
						inMsgs = varz.InMsgs
						outMsgs = varz.OutMsgs
						inBytes = varz.InBytes
						outBytes = varz.OutBytes
					}
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

	// 6. NATS WebSocket Proxy
	natsWSUrl, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", internalWSPort))
	natsWSProxy := httputil.NewSingleHostReverseProxy(natsWSUrl)
	// Important: NATS WS expects it at the root of the target, but we handle it at /nats-ws locally
	mux.Handle("/nats-ws", natsWSProxy)

	// 6. Static Asset Serving (Embedded)
	subFS, err := fs.Sub(webFS, "core/web/dist")
	if err != nil {
		logger.LogInfo("Error accessing sub-filesystem: %v", err)
	}

	// Fallback/SPA logic for embedded web assets
	logger.LogInfo("Using embedded web assets")
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

func checkZombieProcess(device string) {
	// Simple check using fuser if available
	cmd := exec.Command("fuser", device)
	output, err := cmd.CombinedOutput()
	if err == nil && len(output) > 0 {
		logger.LogInfo("[Camera Diagnostic] WARNING: Process holding %s: %s", device, strings.TrimSpace(string(output)))
		logger.LogInfo("[Camera Diagnostic] This might be a zombie dialtone process. considers running 'pkill dialtone'")
	}
}

func CheckStaleHostname(hostname string) {
	if hostname == "" {
		return
	}
	// Try to resolve the hostname. If it resolves before we start tsnet, it's likely stale.
	ips, err := net.LookupIP(hostname)
	if err == nil && len(ips) > 0 {
		logger.LogInfo("[Pre-flight] WARNING: Hostname %s already resolves to %v. This might be a stale MagicDNS entry or another node. This can cause 'operation timed out' errors.", hostname, ips)
	} else {
		logger.LogInfo("[Pre-flight] Hostname %s is not currently resolvable (this is good).", hostname)
	}
}

package robot

import (
	"context"
	"dialtone/dev/core/logger"
	"dialtone/dev/core/util"
	"dialtone/dev/core/web"
	"dialtone/dev/core/mock"
	mavlink_app "dialtone/dev/plugins/mavlink/app"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"time"

	"github.com/bluenviron/gomavlib/v3/pkg/dialects/common"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"tailscale.com/tsnet"
	"io/fs"
	"embed"
)

//go:embed all:src_v1/ui
var webFS embed.FS

func RunStart(args []string) {
	// Check if running under systemd
	if os.Getenv("INVOCATION_ID") == "" {
		logger.LogInfo("[WARNING] Process is not running as a systemd service. Consider running via systemctl.")
	} else {
		logger.LogInfo("[INFO] Running as systemd service.")
	}

	fs := flag.NewFlagSet("start", flag.ExitOnError)
	defaultHostname := os.Getenv("DIALTONE_HOSTNAME")
	if defaultHostname == "" {
		defaultHostname = "dialtone-1"
	}
	hostname := fs.String("hostname", defaultHostname, "Tailscale hostname for this NATS server")
	natsPort := fs.Int("port", 4222, "NATS port to listen on (both local and Tailscale)")
	webPort := fs.Int("web-port", 80, "Web dashboard port")
	stateDir := fs.String("state-dir", "", "Directory to store Tailscale state")
	ephemeral := fs.Bool("ephemeral", false, "Register as ephemeral node")
	localOnly := fs.Bool("local-only", false, "Run without Tailscale")
	wsPort := fs.Int("ws-port", 4223, "NATS WebSocket port")
	verbose := fs.Bool("verbose", true, "Enable verbose logging")
	mavlinkAddr := fs.String("mavlink", "", "Mavlink connection string")
	useMock := fs.Bool("mock", false, "Use mock telemetry and camera data")
	fs.Parse(args)

	if *stateDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			logger.LogFatal("Failed to get home directory: %v", err)
		}
		*stateDir = filepath.Join(homeDir, ".config", "dialtone")
	}

	if *localOnly || os.Getenv("TS_AUTHKEY") == "" {
		if os.Getenv("TS_AUTHKEY") == "" && !*localOnly {
			logger.LogInfo("TS_AUTHKEY missing, falling back to local-only mode.")
		}
		runLocalOnly(*natsPort, *wsPort, *webPort, *verbose, *mavlinkAddr, *useMock, *hostname)
		return
	}

	runWithTailscale(*hostname, *natsPort, *wsPort, *webPort, *stateDir, *ephemeral, *verbose, *mavlinkAddr, *useMock)
}

func getUIVersion() string {
	data, err := webFS.ReadFile("src_v1/ui/package.json")
	if err != nil {
		return "unknown"
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "unknown"
	}
	return pkg.Version
}

func runLocalOnly(port, wsPort, webPort int, verbose bool, mavlinkAddr string, useMock bool, hostname string) {
	// Use 0.0.0.0 to ensure local access works without Tailscale
	ns := startNATSServer("0.0.0.0", port, wsPort, verbose)
	defer ns.Shutdown()

	logger.LogInfo("NATS server started on port %d (local only)", port)

	if useMock {
		go mock.StartMockMavlink(port)
	} else if mavlinkAddr != "" {
		startMavlink(mavlinkAddr, port)
	}

	startNatsPublisher(port)

	uiVersion := getUIVersion()
	logger.LogInfo("Starting Robot UI %s", uiVersion)

	webHandler := web.CreateWebHandler(hostname, uiVersion, port, wsPort, webPort, port, wsPort, ns, nil, nil, useMock, webFS)
	if sub, err := fs.Sub(webFS, "src_v1/ui/dist"); err == nil {
		webHandler = web.CreateWebHandler(hostname, uiVersion, port, wsPort, webPort, port, wsPort, ns, nil, nil, useMock, sub)
	}
	
	// Local web listener
	localWebAddr := fmt.Sprintf("0.0.0.0:%d", webPort)
	if webPort == 80 {
		// Try port 80 but fallback to 8080 if it fails (likely permission denied)
		logger.LogInfo("Web UI (Local Only): Attempting port 80...")
		go http.ListenAndServe(":80", webHandler)
		localWebAddr = "0.0.0.0:8080"
	}

	logger.LogInfo("Web UI (Local Only): Serving at http://%s", localWebAddr)
	http.ListenAndServe(localWebAddr, webHandler)
}

func runWithTailscale(hostname string, port, wsPort, webPort int, stateDir string, ephemeral, verbose bool, mavlinkAddr string, useMock bool) {
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		logger.LogFatal("Failed to create state directory: %v", err)
	}

	ts := &tsnet.Server{
		Hostname:  hostname,
		Dir:       stateDir,
		Ephemeral: ephemeral,
		AuthKey:   os.Getenv("TS_AUTHKEY"),
		UserLogf:  logger.LogInfo,
	}
	if verbose {
		ts.Logf = logger.LogInfo
	}

	util.CheckStaleHostname(hostname)

	if os.Getenv("TS_AUTHKEY") == "" {
		logger.LogFatal("ERROR: TS_AUTHKEY environment variable is not set.")
	}

	logger.LogInfo("Connecting to Tailscale...")
	status, err := ts.Up(context.Background())
	if err != nil {
		logger.LogFatal("Failed to connect to Tailscale: %v", err)
	}
	defer ts.Close()

	var ips []netip.Addr
	ips = status.TailscaleIPs
	ipStr := "none"
	if len(ips) > 0 {
		ipStr = ips[0].String()
	}
	logger.LogInfo("TSNet: Connected (IP: %s)", ipStr)

	localNATSPort := port + 10000
	localWSPort := wsPort + 10000
	// Start NATS locally with WebSocket enabled on internal ports
	ns := startNATSServer("0.0.0.0", localNATSPort, localWSPort, verbose)
	defer ns.Shutdown()

	if useMock {
		go mock.StartMockMavlink(localNATSPort)
	} else if mavlinkAddr != "" {
		startMavlink(mavlinkAddr, localNATSPort)
	}

	startNatsPublisher(localNATSPort)

	natsLn, _ := ts.Listen("tcp", fmt.Sprintf(":%d", port))
	webLn, _ := ts.Listen("tcp", fmt.Sprintf(":%d", webPort))
	defer natsLn.Close()
	defer webLn.Close()

	go util.ProxyListener(natsLn, fmt.Sprintf("0.0.0.0:%d", localNATSPort))

	lc, _ := ts.LocalClient()
	
	// Use sub-filesystem for static assets to ensure correct path resolution
	var staticFS fs.FS = webFS
	if sub, err := fs.Sub(webFS, "src_v1/ui/dist"); err == nil {
		staticFS = sub
	}
	
	uiVersion := getUIVersion()
	logger.LogInfo("Starting Robot UI %s", uiVersion)

	webHandler := web.CreateWebHandler(hostname, uiVersion, port, wsPort, webPort, localNATSPort, localWSPort, ns, lc, ips, useMock, staticFS)

	go func() {
		logger.LogInfo("Web UI (Tailscale): Serving at http://%s:%d", hostname, webPort)
		http.Serve(webLn, webHandler)
	}()

	// Start local listener on 8080 for Cloudflare/direct access
	localWebAddr := "0.0.0.0:8080"
	logger.LogInfo("Web UI (Local): Serving at http://%s", localWebAddr)
	go http.ListenAndServe(":8080", webHandler)

	util.WaitForShutdown()
}

func startNATSServer(host string, port, wsPort int, verbose bool) *server.Server {
	opts := &server.Options{
		Host:  host,
		Port:  port,
		Debug: verbose,
		Trace: verbose,
	}
	if wsPort > 0 {
		opts.Websocket = server.WebsocketOpts{
			Host:  host,
			Port:  wsPort,
			NoTLS: true,
		}
	}
	ns, err := server.NewServer(opts)
	if err != nil {
		logger.LogFatal("Failed to create NATS server: %v", err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(10 * time.Second) {
		logger.LogFatal("NATS server failed to start")
	}
	return ns
}

func startMavlink(endpoint string, natsPort int) {
	logger.LogInfo("Starting Mavlink Service on %s...", endpoint)
	
	var svc *mavlink_app.MavlinkService
	
	config := mavlink_app.MavlinkConfig{
		Endpoint: endpoint,
		Callback: func(evt *mavlink_app.MavlinkEvent) {
			var subject string
			var data []byte
			var err error
			
			switch evt.Type {
			case "HEARTBEAT":
				if msg, ok := evt.Data.(*common.MessageHeartbeat); ok {
					now := time.Now().UnixMilli()
					subject = "mavlink.heartbeat"
					data, err = json.Marshal(map[string]any{
						"type":        "HEARTBEAT",
						"mav_type":    msg.Type,
						"custom_mode": msg.CustomMode,
						"timestamp":   now,
						"t_raw":       evt.ReceivedAt,
						"t_pub":       now,
					})
					logger.LogInfo("[HEARTBEAT] Published to NATS at %v", now)
				}
			case "COMMAND_ACK":
				if msg, ok := evt.Data.(*common.MessageCommandAck); ok {
					subject = "mavlink.command_ack"
					data, err = json.Marshal(msg)
				}
			case "STATUSTEXT":
				if msg, ok := evt.Data.(*common.MessageStatustext); ok {
					subject = "mavlink.statustext"
					data, err = json.Marshal(map[string]any{
						"severity": msg.Severity,
						"text":     msg.Text,
					})
				}
			case "GLOBAL_POSITION_INT":
				if msg, ok := evt.Data.(*common.MessageGlobalPositionInt); ok {
					now := time.Now().UnixMilli()
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
						"t_raw":        evt.ReceivedAt,
						"t_pub":        now,
					})
				}
			case "ATTITUDE":
				if msg, ok := evt.Data.(*common.MessageAttitude); ok {
					now := time.Now().UnixMilli()
					subject = "mavlink.attitude"
					data, err = json.Marshal(map[string]any{
						"roll":       msg.Roll,
						"pitch":      msg.Pitch,
						"yaw":        msg.Yaw,
						"rollspeed":  msg.Rollspeed,
						"pitchspeed": msg.Pitchspeed,
						"yawspeed":   msg.Yawspeed,
						"t_raw":      evt.ReceivedAt,
						"t_pub":      now,
					})
				}
			}
			
			if err == nil && subject != "" {
				select {
				case mock.MavlinkPubChan <- mock.MavlinkNatsMsg{Subject: subject, Data: data}:
				default:
				}
			}
		},
	}
	
	var err error
	svc, err = mavlink_app.NewMavlinkService(config)
	if err != nil {
		logger.LogFatal("Failed to create Mavlink service: %v", err)
	}
	go svc.Start()

	// Handle incoming commands from NATS
	go func() {
		time.Sleep(3 * time.Second)
		nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", natsPort))
		if err != nil {
			logger.LogError("Mavlink NATS Command listener failed to connect: %v", err)
			return
		}
		defer nc.Close()

		nc.Subscribe("rover.command", func(m *nats.Msg) {
			var cmd struct {
				Cmd  string `json:"cmd"`
				Mode string `json:"mode"`
			}
			if err := json.Unmarshal(m.Data, &cmd); err != nil {
				return
			}

			logger.LogInfo("[NATS -> MAVLINK] Executing: %s %s", cmd.Cmd, cmd.Mode)
			switch cmd.Cmd {
			case "arm":
				svc.Arm()
			case "disarm":
				svc.Disarm()
			case "mode":
				svc.SetMode(cmd.Mode)
			}
		})
		
		select {}
	}()
}

func startNatsPublisher(port int) {
	go func() {
		time.Sleep(2 * time.Second)
		nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", port))
		if err != nil {
			return
		}
		defer nc.Close()
		for msg := range mock.MavlinkPubChan {
			nc.Publish(msg.Subject, msg.Data)
		}
	}()
}

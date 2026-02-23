package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	cameraapp "dialtone/dev/plugins/camera/app"
	mavlinkapp "dialtone/dev/plugins/mavlink/app"

	"github.com/bluenviron/gomavlib/v3/pkg/dialects/common"
	"github.com/coder/websocket"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"tailscale.com/tsnet"
)

type config struct {
	webPort      int
	natsPort     int
	natsWSPort   int
	uiPath       string
	appVersion   string
	tsEnabled    bool
	tsHostname   string
	tsWebPort    int
	tsNATSPort   int
	tsWSPort     int
	tsStateDir   string
	tsEphemeral  bool
	tsAuthKey    string
	mavlinkEP    string
	publishStats bool
}

type initResponse struct {
	Version        string `json:"version"`
	WSPort         int    `json:"ws_port"`
	InternalWSPort int    `json:"internal_ws_port"`
	WSPath         string `json:"ws_path"`
}

type roverCommand struct {
	Cmd  string `json:"cmd"`
	Mode string `json:"mode"`
}

func main() {
	cfg := loadConfig()
	log.Printf("Robot server config: web=:%d nats=127.0.0.1:%d ws=127.0.0.1:%d tsnet=%t hostname=%s", cfg.webPort, cfg.natsPort, cfg.natsWSPort, cfg.tsEnabled, cfg.tsHostname)

	ns, err := startEmbeddedNATS(cfg.natsPort, cfg.natsWSPort)
	if err != nil {
		log.Fatalf("embedded NATS failed: %v", err)
	}
	defer ns.Shutdown()

	nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", cfg.natsPort))
	if err != nil {
		log.Fatalf("nats local connect failed: %v", err)
	}
	defer nc.Close()

	startStatsPublisher(nc, ns, cfg.publishStats)

	mavSvc := startMAVLinkBridge(cfg, nc)
	if mavSvc != nil {
		defer mavSvc.Close()
		startRoverCommandConsumer(nc, mavSvc)
	}

	mux := buildMux(cfg)

	localSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.webPort),
		Handler: withCommonHeaders(mux),
	}
	errCh := make(chan error, 1)
	go func() {
		log.Printf("Robot web UI on http://0.0.0.0:%d", cfg.webPort)
		err := localSrv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	cleanupTS := maybeStartTSNet(cfg, mux)
	defer cleanupTS()

	if err := <-errCh; err != nil {
		log.Printf("robot web server exited: %v", err)
		os.Exit(1)
	}
}

func loadConfig() config {
	webPort := envInt("ROBOT_WEB_PORT", 8080)
	natsPort := envInt("NATS_PORT", 4222)
	natsWSPort := envInt("NATS_WS_PORT", 4223)
	uiPath := resolveUIPath()
	tsAuthKey := strings.TrimSpace(os.Getenv("ROBOT_TS_AUTHKEY"))
	if tsAuthKey == "" {
		tsAuthKey = strings.TrimSpace(os.Getenv("TS_AUTHKEY"))
	}
	tsEnabled := envBool("ROBOT_TSNET", tsAuthKey != "")
	tsHost := strings.TrimSpace(os.Getenv("DIALTONE_HOSTNAME"))
	if tsHost == "" {
		tsHost = "drone-1"
	}

	stateDir := strings.TrimSpace(os.Getenv("ROBOT_TSNET_DIR"))
	if stateDir == "" {
		home, _ := os.UserHomeDir()
		if home == "" {
			home = "."
		}
		stateDir = filepath.Join(home, ".config", "dialtone", "robot-tsnet")
	}

	mavlinkEP := strings.TrimSpace(os.Getenv("ROBOT_MAVLINK_ENDPOINT"))
	if mavlinkEP == "" {
		mavlinkEP = strings.TrimSpace(os.Getenv("MAVLINK_ENDPOINT"))
	}

	return config{
		webPort:      webPort,
		natsPort:     natsPort,
		natsWSPort:   natsWSPort,
		uiPath:       uiPath,
		appVersion:   resolveAppVersion(uiPath),
		tsEnabled:    tsEnabled,
		tsHostname:   tsHost,
		tsWebPort:    envInt("ROBOT_TSNET_WEB_PORT", 80),
		tsNATSPort:   envInt("ROBOT_TSNET_NATS_PORT", 4222),
		tsWSPort:     envInt("ROBOT_TSNET_WS_PORT", 4223),
		tsStateDir:   stateDir,
		tsEphemeral:  envBool("ROBOT_TSNET_EPHEMERAL", false),
		tsAuthKey:    tsAuthKey,
		mavlinkEP:    mavlinkEP,
		publishStats: envBool("ROBOT_PUBLISH_STATS", true),
	}
}

func resolveAppVersion(uiPath string) string {
	if v := strings.TrimSpace(os.Getenv("APP_VERSION")); v != "" {
		return v
	}
	pkgPath := filepath.Join(filepath.Dir(uiPath), "package.json")
	raw, err := os.ReadFile(pkgPath)
	if err != nil {
		return "dev"
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return "dev"
	}
	v := strings.TrimSpace(pkg.Version)
	if v == "" {
		return "dev"
	}
	return v
}

func buildMux(cfg config) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/api/init", func(w http.ResponseWriter, _ *http.Request) {
		resp := initResponse{
			Version:        cfg.appVersion,
			WSPort:         cfg.webPort,
			InternalWSPort: cfg.webPort,
			WSPath:         "/natsws",
		}
		writeJSON(w, resp)
	})
	mux.HandleFunc("/natsws", func(w http.ResponseWriter, r *http.Request) {
		proxyNATSWS(w, r, fmt.Sprintf("ws://127.0.0.1:%d", cfg.natsWSPort))
	})
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		cameraapp.StreamHandler(w, r)
	})
	mux.HandleFunc("/api/bookmark", bookmarkHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/")
		target := filepath.Join(cfg.uiPath, rel)
		if r.URL.Path == "/" {
			target = filepath.Join(cfg.uiPath, "index.html")
		}
		if _, err := os.Stat(target); err != nil {
			target = filepath.Join(cfg.uiPath, "index.html")
		}
		http.ServeFile(w, r, target)
	})
	return mux
}

func bookmarkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "missing image upload", http.StatusBadRequest)
		return
	}
	defer file.Close()

	home, _ := os.UserHomeDir()
	if home == "" {
		home = "."
	}
	dir := filepath.Join(home, ".dialtone", "robot", "bookmarks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, "failed to create bookmark dir", http.StatusInternalServerError)
		return
	}
	name := sanitizeFilename(header.Filename)
	if name == "" {
		name = fmt.Sprintf("bookmark_%d.jpg", time.Now().UnixMilli())
	}
	dstPath := filepath.Join(dir, name)
	dst, err := os.Create(dstPath)
	if err != nil {
		http.Error(w, "failed to create bookmark file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "failed to save bookmark", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"ok":   true,
		"path": dstPath,
		"name": name,
	})
}

func sanitizeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == "/" {
		return ""
	}
	name = strings.ReplaceAll(name, "..", "")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	return name
}

func maybeStartTSNet(cfg config, handler http.Handler) func() {
	if !cfg.tsEnabled {
		return func() {}
	}
	if err := os.MkdirAll(cfg.tsStateDir, 0o700); err != nil {
		log.Printf("TSNET disabled: cannot create state dir %s: %v", cfg.tsStateDir, err)
		return func() {}
	}

	ts := &tsnet.Server{
		Hostname:  cfg.tsHostname,
		Dir:       cfg.tsStateDir,
		Ephemeral: cfg.tsEphemeral,
		AuthKey:   cfg.tsAuthKey,
		Logf: func(format string, args ...any) {
			log.Printf("tsnet: "+format, args...)
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	status, err := ts.Up(ctx)
	cancel()
	if err != nil {
		log.Printf("TSNET disabled: up failed: %v", err)
		_ = ts.Close()
		return func() {}
	}
	log.Printf("TSNET up hostname=%s ips=%v", status.Self.HostName, status.TailscaleIPs)

	webLn, err := ts.Listen("tcp", fmt.Sprintf(":%d", cfg.tsWebPort))
	if err != nil {
		log.Printf("TSNET web listener disabled: %v", err)
		_ = ts.Close()
		return func() {}
	}
	go func() {
		log.Printf("TSNET web UI on http://%s:%d", cfg.tsHostname, cfg.tsWebPort)
		if err := http.Serve(webLn, withCommonHeaders(handler)); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Printf("TSNET web serve error: %v", err)
		}
	}()

	natsLn, err := ts.Listen("tcp", fmt.Sprintf(":%d", cfg.tsNATSPort))
	if err != nil {
		log.Printf("TSNET nats listener disabled: %v", err)
	} else {
		go tcpProxyLoop(natsLn, fmt.Sprintf("127.0.0.1:%d", cfg.natsPort), "nats")
	}

	wsLn, err := ts.Listen("tcp", fmt.Sprintf(":%d", cfg.tsWSPort))
	if err != nil {
		log.Printf("TSNET ws listener disabled: %v", err)
	} else {
		go tcpProxyLoop(wsLn, fmt.Sprintf("127.0.0.1:%d", cfg.natsWSPort), "nats-ws")
	}

	return func() {
		_ = webLn.Close()
		if natsLn != nil {
			_ = natsLn.Close()
		}
		if wsLn != nil {
			_ = wsLn.Close()
		}
		_ = ts.Close()
	}
}

func tcpProxyLoop(ln net.Listener, targetAddr, tag string) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			log.Printf("TSNET proxy(%s) accept error: %v", tag, err)
			continue
		}
		go proxyConn(conn, targetAddr, tag)
	}
}

func proxyConn(in net.Conn, targetAddr, tag string) {
	defer in.Close()
	out, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Printf("TSNET proxy(%s) dial %s failed: %v", tag, targetAddr, err)
		return
	}
	defer out.Close()

	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(out, in)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(in, out)
		done <- struct{}{}
	}()
	<-done
}

func startEmbeddedNATS(port, wsPort int) (*natsserver.Server, error) {
	opts := &natsserver.Options{
		Host: "127.0.0.1",
		Port: port,
		Websocket: natsserver.WebsocketOpts{
			Host:           "127.0.0.1",
			Port:           wsPort,
			NoTLS:          true,
			AllowedOrigins: []string{"*"},
		},
	}
	ns, err := natsserver.NewServer(opts)
	if err != nil {
		return nil, err
	}
	go ns.Start()
	if !ns.ReadyForConnections(10 * time.Second) {
		return nil, fmt.Errorf("nats server did not become ready on %d/%d", port, wsPort)
	}
	log.Printf("Embedded NATS ready on nats://127.0.0.1:%d and ws://127.0.0.1:%d", port, wsPort)
	return ns, nil
}

func startStatsPublisher(nc *nats.Conn, ns *natsserver.Server, enabled bool) {
	if !enabled {
		return
	}
	start := time.Now()
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			varz, err := ns.Varz(nil)
			if err != nil {
				continue
			}
			payload := map[string]any{
				"type":        "STATS",
				"uptime":      time.Since(start).Round(time.Second).String(),
				"nats_total":  varz.InMsgs,
				"connections": varz.Connections,
				"timestamp":   time.Now().UnixMilli(),
			}
			if b, err := json.Marshal(payload); err == nil {
				_ = nc.Publish("mavlink.stats", b)
			}
		}
	}()
}

func startMAVLinkBridge(cfg config, nc *nats.Conn) *mavlinkapp.MavlinkService {
	if cfg.mavlinkEP == "" {
		log.Printf("MAVLink bridge disabled (set ROBOT_MAVLINK_ENDPOINT or MAVLINK_ENDPOINT)")
		return nil
	}
	log.Printf("MAVLink bridge: connecting to %s", cfg.mavlinkEP)

	svc, err := mavlinkapp.NewMavlinkService(mavlinkapp.MavlinkConfig{
		Endpoint: cfg.mavlinkEP,
		Callback: func(evt *mavlinkapp.MavlinkEvent) {
			subj, payload := toNATSPayload(evt)
			if subj == "" || payload == nil {
				return
			}
			data, err := json.Marshal(payload)
			if err != nil {
				return
			}
			_ = nc.Publish(subj, data)
		},
	})
	if err != nil {
		log.Printf("MAVLink bridge disabled: %v", err)
		return nil
	}
	go svc.Start()
	return svc
}

func startRoverCommandConsumer(nc *nats.Conn, svc *mavlinkapp.MavlinkService) {
	_, err := nc.Subscribe("rover.command", func(msg *nats.Msg) {
		var cmd roverCommand
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			log.Printf("rover.command decode error: %v", err)
			return
		}
		switch strings.ToLower(strings.TrimSpace(cmd.Cmd)) {
		case "arm":
			if err := svc.Arm(); err != nil {
				log.Printf("rover.command arm failed: %v", err)
			}
		case "disarm":
			if err := svc.Disarm(); err != nil {
				log.Printf("rover.command disarm failed: %v", err)
			}
		case "mode":
			if err := svc.SetMode(strings.TrimSpace(cmd.Mode)); err != nil {
				log.Printf("rover.command mode failed: %v", err)
			}
		default:
			log.Printf("rover.command unknown cmd=%q", cmd.Cmd)
		}
	})
	if err != nil {
		log.Printf("rover.command subscription failed: %v", err)
		return
	}
	_ = nc.Flush()
}

func toNATSPayload(evt *mavlinkapp.MavlinkEvent) (string, map[string]any) {
	if evt == nil {
		return "", nil
	}
	now := evt.ReceivedAt
	if now == 0 {
		now = time.Now().UnixMilli()
	}
	switch msg := evt.Data.(type) {
	case *common.MessageHeartbeat:
		return "mavlink.heartbeat", map[string]any{
			"type":        "HEARTBEAT",
			"mav_type":    msg.Type,
			"custom_mode": msg.CustomMode,
			"timestamp":   now,
			"t_raw":       now,
		}
	case *common.MessageAttitude:
		return "mavlink.attitude", map[string]any{
			"type":       "ATTITUDE",
			"roll":       msg.Roll,
			"pitch":      msg.Pitch,
			"yaw":        msg.Yaw,
			"rollspeed":  msg.Rollspeed,
			"pitchspeed": msg.Pitchspeed,
			"yawspeed":   msg.Yawspeed,
			"timestamp":  now,
			"t_raw":      now,
		}
	case *common.MessageGlobalPositionInt:
		var hdg float64 = -1
		if msg.Hdg != 65535 {
			hdg = float64(msg.Hdg) / 100.0
		}
		return "mavlink.global_position_int", map[string]any{
			"type":         "GLOBAL_POSITION_INT",
			"lat":          float64(msg.Lat) / 1e7,
			"lon":          float64(msg.Lon) / 1e7,
			"alt":          float64(msg.Alt) / 1000.0,
			"relative_alt": float64(msg.RelativeAlt) / 1000.0,
			"vx":           float64(msg.Vx) / 100.0,
			"vy":           float64(msg.Vy) / 100.0,
			"vz":           float64(msg.Vz) / 100.0,
			"hdg":          hdg,
			"timestamp":    now,
			"t_raw":        now,
		}
	case *common.MessageStatustext:
		text := strings.TrimRight(string(msg.Text[:]), "\x00")
		return "mavlink.statustext", map[string]any{
			"type":      "STATUSTEXT",
			"severity":  msg.Severity,
			"text":      text,
			"timestamp": now,
			"t_raw":     now,
		}
	case *common.MessageCommandAck:
		return "mavlink.command_ack", map[string]any{
			"type":      "COMMAND_ACK",
			"command":   msg.Command,
			"result":    msg.Result,
			"timestamp": now,
			"t_raw":     now,
		}
	default:
		return "", nil
	}
}

func proxyNATSWS(w http.ResponseWriter, r *http.Request, upstreamURL string) {
	ctx := r.Context()
	downstream, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer downstream.Close(websocket.StatusNormalClosure, "closing")

	upstream, _, err := websocket.Dial(ctx, upstreamURL, nil)
	if err != nil {
		_ = downstream.Close(websocket.StatusPolicyViolation, "nats ws unavailable")
		return
	}
	defer upstream.Close(websocket.StatusNormalClosure, "closing")

	errc := make(chan error, 2)
	go pipeWS(ctx, downstream, upstream, errc)
	go pipeWS(ctx, upstream, downstream, errc)
	<-errc
}

func pipeWS(ctx context.Context, src, dst *websocket.Conn, errc chan<- error) {
	for {
		msgType, msg, err := src.Read(ctx)
		if err != nil {
			errc <- err
			return
		}
		if err := dst.Write(ctx, msgType, msg); err != nil {
			errc <- err
			return
		}
	}
}

func withCommonHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func resolveUIPath() string {
	cwd, _ := os.Getwd()
	uiPath := filepath.Join(cwd, "ui", "dist")
	if _, err := os.Stat(uiPath); err == nil {
		return uiPath
	}
	return filepath.Join(cwd, "src", "plugins", "robot", "src_v1", "ui", "dist")
}

func envDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	var v int
	if _, err := fmt.Sscanf(raw, "%d", &v); err != nil || v <= 0 {
		return fallback
	}
	return v
}

func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if raw == "" {
		return fallback
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

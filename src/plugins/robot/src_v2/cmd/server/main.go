package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"github.com/coder/websocket"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

type initResponse struct {
	Version        string `json:"version"`
	WSPort         int    `json:"ws_port"`
	InternalWSPort int    `json:"internal_ws_port"`
	WSPath         string `json:"ws_path"`
	WSPathCompat   string `json:"wsPath"`
}

func main() {
	logs.SetOutput(os.Stdout)

	listen := flag.String("listen", envOrDefault("ROBOT_V2_LISTEN", ":8080"), "HTTP listen address")
	uiDist := flag.String("ui-dist", envOrDefault("ROBOT_V2_UI_DIST", ""), "Path to robot src_v2 ui/dist")
	natsPort := flag.Int("nats-port", envIntOrDefault("ROBOT_V2_NATS_PORT", 4222), "Embedded NATS TCP port")
	natsWSPort := flag.Int("nats-ws-port", envIntOrDefault("ROBOT_V2_NATS_WS_PORT", 4223), "Embedded NATS websocket port")
	flag.Parse()
	cameraStreamURL := strings.TrimSpace(envOrDefault("ROBOT_V2_CAMERA_STREAM_URL", ""))
	mavlinkEnabled := strings.TrimSpace(envOrDefault("ROBOT_V2_MAVLINK_ENABLED", "0")) == "1"

	resolvedUIDist := strings.TrimSpace(*uiDist)
	if resolvedUIDist == "" {
		if auto, ok := resolveDefaultUIDist(); ok {
			resolvedUIDist = auto
			logs.Info("robot src_v2 using inferred ui/dist path: %s", resolvedUIDist)
		}
	}
	appVersion := resolveAppVersion(resolvedUIDist)

	ns, err := startEmbeddedNATS(*natsPort, *natsWSPort)
	if err != nil {
		logs.Error("robot src_v2 nats startup failed: %v", err)
		os.Exit(1)
	}
	defer ns.Shutdown()
	startStatsPublisher(*natsPort, ns, mavlinkEnabled)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/api/init", func(w http.ResponseWriter, _ *http.Request) {
		payload := initResponse{
			Version:        appVersion,
			WSPort:         *natsWSPort,
			InternalWSPort: *natsWSPort,
			WSPath:         "/natsws",
			WSPathCompat:   "/natsws",
		}
		writeJSON(w, payload)
	})
	mux.HandleFunc("/api/integration-health", func(w http.ResponseWriter, _ *http.Request) {
		cameraStatus := "not-configured"
		if cameraStreamURL != "" {
			cameraStatus = "configured"
		}
		mavlinkStatus := "not-configured"
		if mavlinkEnabled {
			mavlinkStatus = "configured"
		}
		payload := map[string]any{
			"status": "degraded",
			"natsws": map[string]any{
				"status": "ok",
			},
			"ui": map[string]any{
				"status": func() string {
					if uiDistReady(resolvedUIDist) {
						return "ok"
					}
					return "not-configured"
				}(),
			},
			"camera": map[string]any{
				"status": cameraStatus,
			},
			"mavlink": map[string]any{
				"status": mavlinkStatus,
			},
		}
		writeJSON(w, payload)
	})
	mux.HandleFunc("/api/bookmark", func(w http.ResponseWriter, r *http.Request) {
		bookmarkHandler(w, r)
	})
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		if cameraStreamURL == "" {
			http.Error(w, "camera stream not configured in scaffold", http.StatusServiceUnavailable)
			return
		}
		if err := proxyStream(w, r, cameraStreamURL); err != nil {
			http.Error(w, "camera stream upstream unavailable", http.StatusBadGateway)
			return
		}
	})
	mux.HandleFunc("/natsws", func(w http.ResponseWriter, r *http.Request) {
		proxyNATSWS(w, r, fmt.Sprintf("ws://127.0.0.1:%d", *natsWSPort))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !uiDistReady(resolvedUIDist) {
			http.Error(w, "robot src_v2 server scaffold active; ui/dist not configured", http.StatusServiceUnavailable)
			return
		}
		serveUISPA(w, r, resolvedUIDist)
	})

	logs.Info("robot src_v2 server listening on %s (nats=%d ws=%d)", *listen, *natsPort, *natsWSPort)
	if err := http.ListenAndServe(*listen, mux); err != nil {
		logs.Error("robot src_v2 server failed: %v", err)
		os.Exit(1)
	}
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
	return ns, nil
}

func startStatsPublisher(natsPort int, ns *natsserver.Server, mavlinkEnabled bool) {
	natsURL := fmt.Sprintf("nats://127.0.0.1:%d", natsPort)
	nc, err := nats.Connect(natsURL, nats.Timeout(2*time.Second))
	if err != nil {
		logs.Warn("robot src_v2 stats publisher disabled (nats connect failed): %v", err)
		return
	}
	started := time.Now()
	var lastMavlinkTelemetryAt atomic.Int64
	_, _ = nc.Subscribe("mavlink.>", func(msg *nats.Msg) {
		subj := strings.TrimSpace(msg.Subject)
		switch subj {
		case "mavlink.heartbeat", "mavlink.attitude", "mavlink.global_position_int", "mavlink.statustext", "mavlink.command_ack":
			lastMavlinkTelemetryAt.Store(time.Now().UnixMilli())
		}
	})
	_ = nc.Flush()

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			varz, err := ns.Varz(nil)
			if err != nil {
				continue
			}
			errors := make([]string, 0, 2)
			if mavlinkEnabled {
				last := lastMavlinkTelemetryAt.Load()
				if last == 0 {
					errors = append(errors, "mavlink telemetry not received yet")
				} else {
					since := time.Since(time.UnixMilli(last))
					if since > 3*time.Second {
						errors = append(errors, fmt.Sprintf("mavlink telemetry stale (%s ago)", since.Round(time.Second)))
					}
				}
			} else {
				errors = append(errors, "mavlink disabled")
			}
			payload := map[string]any{
				"type":        "STATS",
				"uptime":      time.Since(started).Round(time.Second).String(),
				"nats_total":  varz.InMsgs,
				"connections": varz.Connections,
				"timestamp":   time.Now().UnixMilli(),
				"errors":      errors,
			}
			if b, err := json.Marshal(payload); err == nil {
				_ = nc.Publish("mavlink.stats", b)
			}
		}
	}()
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

func envOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func envIntOrDefault(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	out, err := strconv.Atoi(raw)
	if err != nil || out <= 0 {
		return fallback
	}
	return out
}

func resolveDefaultUIDist() (string, bool) {
	cwd, _ := os.Getwd()
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	candidates := []string{
		filepath.Join(cwd, "src", "plugins", "robot", "src_v2", "ui", "dist"),
		filepath.Join(exeDir, "..", "src", "plugins", "robot", "src_v2", "ui", "dist"),
	}
	for _, candidate := range candidates {
		p := filepath.Clean(candidate)
		if uiDistReady(p) {
			return p, true
		}
	}
	return "", false
}

func uiDistReady(uiDist string) bool {
	uiDist = strings.TrimSpace(uiDist)
	if uiDist == "" {
		return false
	}
	index := filepath.Join(uiDist, "index.html")
	if _, err := os.Stat(index); err != nil {
		return false
	}
	return true
}

func resolveAppVersion(uiDist string) string {
	if v := strings.TrimSpace(os.Getenv("APP_VERSION")); v != "" {
		return v
	}
	if strings.TrimSpace(uiDist) == "" {
		return "src_v2-dev"
	}
	pkgPath := filepath.Join(filepath.Dir(uiDist), "package.json")
	raw, err := os.ReadFile(pkgPath)
	if err != nil {
		return "src_v2-dev"
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return "src_v2-dev"
	}
	v := strings.TrimSpace(pkg.Version)
	if v == "" {
		return "src_v2-dev"
	}
	return v
}

func serveUISPA(w http.ResponseWriter, r *http.Request, uiDist string) {
	rel := strings.TrimPrefix(r.URL.Path, "/")
	target := filepath.Join(uiDist, rel)
	if r.URL.Path == "/" {
		target = filepath.Join(uiDist, "index.html")
	}
	if _, err := os.Stat(target); err != nil {
		target = filepath.Join(uiDist, "index.html")
	}
	http.ServeFile(w, r, target)
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
	return strings.ReplaceAll(name, "..", "_")
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func proxyStream(w http.ResponseWriter, r *http.Request, upstreamBase string) error {
	u, err := url.Parse(strings.TrimSpace(upstreamBase))
	if err != nil {
		return err
	}
	proxy := httputil.NewSingleHostReverseProxy(u)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Path = "/stream"
		req.Host = u.Host
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(rw, "camera stream upstream unavailable", http.StatusBadGateway)
	}
	proxy.ServeHTTP(w, r)
	return nil
}

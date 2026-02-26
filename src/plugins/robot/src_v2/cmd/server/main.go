package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"github.com/coder/websocket"
	natsserver "github.com/nats-io/nats-server/v2/server"
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
				"status": "not-configured",
			},
			"mavlink": map[string]any{
				"status": "not-configured",
			},
		}
		writeJSON(w, payload)
	})
	mux.HandleFunc("/api/bookmark", func(w http.ResponseWriter, r *http.Request) {
		bookmarkHandler(w, r)
	})
	mux.HandleFunc("/stream", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "camera stream not configured in scaffold", http.StatusServiceUnavailable)
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

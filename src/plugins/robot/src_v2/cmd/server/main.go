package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coder/websocket"
	natsserver "github.com/nats-io/nats-server/v2/server"
)

func main() {
	listen := flag.String("listen", envOrDefault("ROBOT_V2_LISTEN", ":8080"), "HTTP listen address")
	uiDist := flag.String("ui-dist", envOrDefault("ROBOT_V2_UI_DIST", ""), "Path to robot src_v2 ui/dist")
	natsPort := flag.Int("nats-port", envIntOrDefault("ROBOT_V2_NATS_PORT", 4222), "Embedded NATS TCP port")
	natsWSPort := flag.Int("nats-ws-port", envIntOrDefault("ROBOT_V2_NATS_WS_PORT", 4223), "Embedded NATS websocket port")
	flag.Parse()

	ns, err := startEmbeddedNATS(*natsPort, *natsWSPort)
	if err != nil {
		fmt.Fprintf(os.Stderr, "robot src_v2 nats startup failed: %v\n", err)
		os.Exit(1)
	}
	defer ns.Shutdown()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/api/init", func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"status": "scaffold",
			"wsPath": "/natsws",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	})
	mux.HandleFunc("/stream", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "camera stream not configured in scaffold", http.StatusServiceUnavailable)
	})
	mux.HandleFunc("/natsws", func(w http.ResponseWriter, r *http.Request) {
		proxyNATSWS(w, r, fmt.Sprintf("ws://127.0.0.1:%d", *natsWSPort))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(*uiDist) == "" {
			http.Error(w, "robot src_v2 server scaffold active; ui/dist not configured", http.StatusServiceUnavailable)
			return
		}
		index := filepath.Join(*uiDist, "index.html")
		if _, err := os.Stat(index); err != nil {
			http.Error(w, fmt.Sprintf("ui index missing at %s", index), http.StatusServiceUnavailable)
			return
		}
		http.FileServer(http.Dir(*uiDist)).ServeHTTP(w, r)
	})

	fmt.Printf("robot src_v2 scaffold server listening on %s\n", *listen)
	if err := http.ListenAndServe(*listen, mux); err != nil {
		fmt.Fprintf(os.Stderr, "robot src_v2 server failed: %v\n", err)
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
	var out int
	if _, err := fmt.Sscanf(raw, "%d", &out); err != nil || out <= 0 {
		return fallback
	}
	return out
}

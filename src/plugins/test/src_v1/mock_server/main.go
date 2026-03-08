package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

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

type bufferWriter struct {
	data []byte
}

func (b *bufferWriter) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *bufferWriter) Bytes() []byte {
	return b.data
}

func (b *bufferWriter) Len() int {
	return len(b.data)
}

func main() {
	listen := flag.String("listen", envOrDefault("TEST_UI_MOCK_LISTEN", ":8787"), "HTTP listen address")
	natsPort := flag.Int("nats-port", envIntOrDefault("TEST_UI_MOCK_NATS_PORT", 4322), "Embedded NATS TCP port")
	natsWSPort := flag.Int("nats-ws-port", envIntOrDefault("TEST_UI_MOCK_NATS_WS_PORT", 4323), "Embedded NATS websocket port")
	flag.Parse()

	ns, err := startEmbeddedNATS(*natsPort, *natsWSPort)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mock server nats startup failed: %v\n", err)
		os.Exit(1)
	}
	defer ns.Shutdown()

	if err := startMockPublishers(*natsPort, ns); err != nil {
		fmt.Fprintf(os.Stderr, "mock server publisher startup failed: %v\n", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/api/init", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, initResponse{
			Version:        "mock",
			WSPort:         *natsWSPort,
			InternalWSPort: *natsWSPort,
			WSPath:         "/natsws",
			WSPathCompat:   "/natsws",
		})
	})
	mux.HandleFunc("/api/integration-health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{
			"status":  "ok",
			"natsws":  map[string]any{"status": "ok"},
			"camera":  map[string]any{"status": "ok"},
			"mavlink": map[string]any{"status": "ok"},
			"source":  "mock",
		})
	})
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
		ticker := time.NewTicker(120 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case now := <-ticker.C:
				img := image.NewRGBA(image.Rect(0, 0, 1280, 720))
				offset := int(now.UnixMilli()/8) % 255
				for y := 0; y < 720; y++ {
					for x := 0; x < 1280; x++ {
						img.Set(x, y, color.RGBA{
							R: uint8((x/6 + offset) % 255),
							G: uint8((y/5 + offset/2) % 255),
							B: uint8(72 + (offset % 120)),
							A: 255,
						})
					}
				}
				buf := new(bufferWriter)
				if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 72}); err != nil {
					continue
				}
				_, _ = fmt.Fprintf(w, "--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", buf.Len())
				_, _ = w.Write(buf.Bytes())
				_, _ = w.Write([]byte("\r\n"))
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}
		}
	})
	mux.HandleFunc("/natsws", func(w http.ResponseWriter, r *http.Request) {
		proxyNATSWS(w, r, fmt.Sprintf("ws://127.0.0.1:%d", *natsWSPort))
	})

	fmt.Printf("test ui mock server listening on %s\n", *listen)
	fmt.Printf(" - nats tcp  : %d\n", *natsPort)
	fmt.Printf(" - nats ws   : %d\n", *natsWSPort)
	fmt.Printf(" - api init  : http://127.0.0.1:%s/api/init\n", strings.TrimPrefix(*listen, ":"))
	if err := http.ListenAndServe(*listen, mux); err != nil {
		fmt.Fprintf(os.Stderr, "mock server failed: %v\n", err)
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

func startMockPublishers(natsPort int, ns *natsserver.Server) error {
	nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", natsPort), nats.Timeout(2*time.Second))
	if err != nil {
		return err
	}

	var commands atomic.Int64
	_, _ = nc.Subscribe("rover.command", func(msg *nats.Msg) {
		commands.Add(1)
		payload := map[string]any{}
		_ = json.Unmarshal(msg.Data, &payload)
		cmd := strings.TrimSpace(fmt.Sprint(payload["cmd"]))
		if cmd == "" {
			cmd = "unknown"
		}
		publishJSON(nc, "rover.command_ack", map[string]any{
			"cmd":    cmd,
			"status": "ok",
		})
		publishJSON(nc, "rover.log", map[string]any{
			"line": fmt.Sprintf("[mock] ack %s", cmd),
		})
	})

	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		started := time.Now()
		for now := range ticker.C {
			t := now.Sub(started).Seconds()
			publishJSON(nc, "mavlink.heartbeat", map[string]any{
				"type":          "HEARTBEAT",
				"mode":          modeForTick(t),
				"mav_type":      10,
				"base_mode":     209,
				"custom_mode":   int(t*3) % 7,
				"system_status": 4,
				"timestamp":     now.UnixMilli(),
			})
			publishJSON(nc, "mavlink.vfr_hud", map[string]any{
				"groundspeed": 2.3 + 0.7*sinApprox(t*0.8),
				"alt":         1.5 + 0.4*sinApprox(t*0.3),
				"heading":     int(t*18) % 360,
			})
			publishJSON(nc, "mavlink.attitude", map[string]any{
				"roll":  0.22 * sinApprox(t),
				"pitch": 0.18 * sinApprox(t*0.7),
				"yaw":   0.12 * t,
			})
			publishJSON(nc, "mavlink.sys_status", map[string]any{
				"voltage_battery": 12400 + int(180*sinApprox(t*0.4)),
			})
			publishJSON(nc, "mavlink.gps_raw_int", map[string]any{
				"satellites_visible": 11 + int(2*sinApprox(t*0.5)),
			})
			publishJSON(nc, "mavlink.global_position_int", map[string]any{
				"lat":          37.7749 + 0.0012*sinApprox(t*0.23),
				"lon":          -122.4194 + 0.0011*sinApprox(t*0.19+0.8),
				"relative_alt": 1.6 + 0.3*sinApprox(t*0.37),
				"hdg":          (int(t*18) % 360) * 100,
			})
			publishJSON(nc, "rover.status", map[string]any{
				"mode":         modeForTick(t),
				"feed":         "mock-front",
				"fps":          8,
				"latency_ms":   42 + int(7*sinApprox(t*0.9)),
				"bitrate_mbps": 3.6 + 0.3*sinApprox(t*0.35),
			})
			publishJSON(nc, "rover.steering", map[string]any{
				"trim":          1500 + int(12*sinApprox(t*0.4)),
				"turn_rate_max": 28 + int(2*sinApprox(t*0.8)),
				"throttle_expo": fmt.Sprintf("%.2f", 0.42+0.03*sinApprox(t*0.6)),
				"brake_force":   fmt.Sprintf("%.2f", 0.18+0.02*sinApprox(t*0.7)),
			})
			publishJSON(nc, "rover.params", map[string]any{
				"cruise_speed":  2.8 + 0.2*sinApprox(t*0.25),
				"rtl_speed":     2.1 + 0.15*sinApprox(t*0.31),
				"nav_l1_period": 12,
				"wpnav_radius":  1.4,
			})
			if int(t*2)%2 == 0 {
				publishJSON(nc, "rover.log", map[string]any{
					"line": fmt.Sprintf("[mock] telemetry alive mode=%s cmds=%d", modeForTick(t), commands.Load()),
				})
			}
			if varz, err := ns.Varz(nil); err == nil {
				publishJSON(nc, "mavlink.stats", map[string]any{
					"type":        "STATS",
					"uptime":      now.Sub(started).Round(time.Second).String(),
					"nats_total":  varz.InMsgs,
					"connections": varz.Connections,
					"timestamp":   now.UnixMilli(),
					"errors":      []string{},
				})
			}
		}
	}()
	return nil
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

func publishJSON(nc *nats.Conn, subject string, payload map[string]any) {
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = nc.Publish(subject, body)
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	var parsed int
	_, err := fmt.Sscanf(v, "%d", &parsed)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func modeForTick(t float64) string {
	modes := []string{"GUIDED", "AUTO", "MANUAL", "HOLD"}
	idx := int(t/4) % len(modes)
	return modes[idx]
}

func sinApprox(v float64) float64 {
	// Good enough for mock telemetry without importing math.
	const pi = 3.141592653589793
	x := v
	for x > pi {
		x -= 2 * pi
	}
	for x < -pi {
		x += 2 * pi
	}
	x2 := x * x
	return x * (1 - x2/6 + (x2*x2)/120)
}

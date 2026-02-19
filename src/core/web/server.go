package web

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"runtime"
	"strings"
	"time"

	"dialtone/dev/core/logger"
	"dialtone/dev/core/mock"
	camera "dialtone/dev/plugins/camera/app"

	"github.com/coder/websocket"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"tailscale.com/client/tailscale"
)

// Global start time for uptime calculation
var startTime = time.Now()

// CreateWebHandler creates the HTTP handler for the unified web dashboard
func CreateWebHandler(hostname, version string, natsPort, wsPort, webPort, internalNATSPort, internalWSPort int, ns *server.Server, lc *tailscale.LocalClient, ips []netip.Addr, useMock bool, staticFS fs.FS) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

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
			"version":            version,
			"hostname":           hostname,
			"nats_port":          natsPort,
			"internal_nats_port": internalNATSPort,
			"ws_port":            wsPort,
			"internal_ws_port":   internalWSPort,
			"ws_path":            "/nats-ws", // Path to the proxied NATS WS
			"web_port":           webPort,
			"ips":                ips,
		}
		w.Header().Set("Content-Type", "application/json")
		// Prevent caching of init data to ensure version check is fresh
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
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
			"version":       version,
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
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
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

	// 5. WebSocket for real-time updates (unified dashboard)
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
		ticker := time.NewTicker(100 * time.Millisecond) // Faster ticker for attitude
		defer ticker.Stop()

		// Channel to relay messages to the websocket in a thread-safe manner
		msgChan := make(chan []byte, 100)

		// Goroutine to gather NATS messages
		if ns != nil {
			nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", internalNATSPort))
			if err == nil {
				nc.Subscribe("mavlink.>", func(m *nats.Msg) {
					// Add relay timestamp
					var raw map[string]any
					if err := json.Unmarshal(m.Data, &raw); err == nil {
						raw["t_relay"] = time.Now().UnixMilli()
						if newData, err := json.Marshal(raw); err == nil {
							select {
							case msgChan <- newData:
							default:
							}
							return
						}
					}

					// Fallback to original data if unmarshal fails
					select {
					case msgChan <- m.Data:
					default:
						// Drop message if buffer is full
					}
				})
				defer nc.Close()
			}
		}

		// Local state for NATS message count
		var natsMsgCount int64

		// Main Relay Loop
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case data := <-msgChan:
					if err := c.Write(ctx, websocket.MessageText, data); err != nil {
						return
					}
				case <-ticker.C:
					var connections int
					var inMsgs, outMsgs, inBytes, outBytes int64

					if ns != nil {
						varz, _ := ns.Varz(nil)
						if varz != nil {
							connections = varz.Connections
							inMsgs = varz.InMsgs
							outMsgs = varz.OutMsgs
							inBytes = varz.InBytes
							outBytes = varz.OutBytes
							natsMsgCount = inMsgs
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
						"nats_total":  natsMsgCount,
					}

					// If in mock mode, add mock telemetry directly to the stats
					if useMock {
						t := time.Since(startTime).Seconds()
						stats["lat"] = float64(37.7749 + 0.0001*float64(time.Now().Second())/60.0)
						stats["lon"] = float64(-122.4194 + 0.0001*float64(time.Now().Second())/60.0)
						stats["alt"] = float64(10.5)
						stats["roll"] = float64(0.1 * float64(time.Now().Second()%10))
						stats["pitch"] = float64(0.05 * float64(time.Now().Second()%5))
						stats["yaw"] = float64(t * 0.1)
						stats["sats"] = float64(12)
						stats["battery"] = float64(12.4)
						stats["mode"] = "GUIDED"
						stats["errors"] = []string{"Link latency high", "GPS offset detected"}
					}
					data, _ := json.Marshal(stats)
					select {
					case msgChan <- data:
					default:
					}
				}
			}
		}()

		// Keep connection open until context is done
		<-ctx.Done()
	})

	// 6. NATS WebSocket Proxy
	natsWSUrl, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", internalWSPort))
	natsWSProxy := httputil.NewSingleHostReverseProxy(natsWSUrl)
	mux.Handle("/nats-ws", natsWSProxy)

	// 6. Static Asset Serving
	if staticFS != nil {
		logger.LogInfo("Using provided static web assets")
		staticHandler := http.FileServer(http.FS(staticFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Disable caching for index.html and root to ensure updates are seen immediately
			if r.URL.Path == "/" || strings.HasSuffix(r.URL.Path, "/index.html") {
				w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
				w.Header().Set("Pragma", "no-cache")
				w.Header().Set("Expires", "0")
			} else {
				// For other assets (likely hashed by Vite), allow caching
				w.Header().Set("Cache-Control", "public, max-age=3600")
			}

			f, err := staticFS.Open(strings.TrimPrefix(r.URL.Path, "/"))
			if err == nil {
				f.Close()
				staticHandler.ServeHTTP(w, r)
				return
			}
			// SPA Fallback
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
			http.ServeFileFS(w, r, staticFS, "index.html")
		})
	}

	return mux
}

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

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

	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/mock"
	camera "dialtone/cli/src/plugins/camera/app"

	"github.com/coder/websocket"
	"github.com/nats-io/nats-server/v2/server"
	"tailscale.com/client/tailscale"
)

// Global start time for uptime calculation
var startTime = time.Now()

// CreateWebHandler creates the HTTP handler for the unified web dashboard
func CreateWebHandler(hostname string, natsPort, wsPort, webPort, internalNATSPort, internalWSPort int, ns *server.Server, lc *tailscale.LocalClient, ips []netip.Addr, useMock bool, staticFS fs.FS) http.Handler {
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

		// Local state for NATS message count
		var natsMsgCount int64

		// Subscribe to NATS if available to forward to WS
		// (In a real system we'd use a more robust pub/sub bridge)
		
		for {
			select {
			case <-ctx.Done():
				return
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
								// In real mode, this would come from the NATS subscription
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
	mux.Handle("/nats-ws", natsWSProxy)

	// 6. Static Asset Serving
	if staticFS != nil {
		logger.LogInfo("Using provided static web assets")
		staticHandler := http.FileServer(http.FS(staticFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			f, err := staticFS.Open(strings.TrimPrefix(r.URL.Path, "/"))
			if err == nil {
				f.Close()
				staticHandler.ServeHTTP(w, r)
				return
			}
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

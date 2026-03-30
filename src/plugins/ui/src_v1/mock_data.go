package uiv1

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"net/http"
	"sync"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// RunMockData starts a standalone mock server for the UI.
func RunMockData(_ []string) {
	natsPort := 4222
	wsPort := 4223
	streamPort := 8080
	logs.Info("Starting mock data server with embedded NATS")
	logs.Info("mock data server: nats=:%d ws=:%d stream=:%d/stream", natsPort, wsPort, streamPort)

	ns := startMockNATSServer(natsPort, wsPort)
	defer ns.Shutdown()

	go runMockPublisher(natsPort)

	go func() {
		streamMux := http.NewServeMux()
		streamMux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
			for {
				select {
				case <-r.Context().Done():
					return
				default:
					img := image.NewRGBA(image.Rect(0, 0, 640, 480))
					offset := int(time.Now().UnixMilli()/10) % 255
					for y := 0; y < 480; y++ {
						for x := 0; x < 640; x++ {
							img.Set(x, y, color.RGBA{uint8((x + offset) % 255), uint8(y % 255), 100, 255})
						}
					}

					buf := new(multiWriter)
					if err := jpeg.Encode(buf, img, nil); err != nil {
						continue
					}

					fmt.Fprintf(w, "--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", buf.Len())
					w.Write(buf.Bytes())
					w.Write([]byte("\r\n"))
					time.Sleep(100 * time.Millisecond)
				}
			}
		})

		logs.Info("mock camera stream listening on :%d/stream", streamPort)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", streamPort), streamMux); err != nil {
			logs.Error("stream server failed: %v", err)
		}
	}()

	wg := sync.WaitGroup{}
	wg.Add(1)
	wg.Wait()
}

func startMockNATSServer(port, wsPort int) *server.Server {
	opts := &server.Options{
		Host: "0.0.0.0",
		Port: port,
		Websocket: server.WebsocketOpts{
			Host:           "0.0.0.0",
			Port:           wsPort,
			NoTLS:          true,
			AllowedOrigins: []string{"*"},
		},
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		logs.Error("failed to create NATS server: %v", err)
		return nil
	}

	go ns.Start()

	if !ns.ReadyForConnections(10 * time.Second) {
		logs.Error("NATS server failed to start")
		return nil
	}
	return ns
}

func runMockPublisher(natsPort int) {
	nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", natsPort))
	if err != nil {
		logs.Error("publisher failed to connect to NATS: %v", err)
		return
	}
	defer nc.Close()

	logs.Info("mock publisher connected and streaming telemetry")

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	start := time.Now()

	for range ticker.C {
		t := time.Since(start).Seconds()

		heartbeat := map[string]interface{}{
			"type":          "HEARTBEAT",
			"mav_type":      10,
			"base_mode":     209,
			"custom_mode":   5,
			"system_status": 4,
			"timestamp":     time.Now().Unix(),
		}
		publishJSON(nc, "mavlink.heartbeat", heartbeat)

		hud := map[string]interface{}{
			"airspeed": 5.0 + math.Sin(t),
			"alt":      10.0 + math.Cos(t),
			"heading":  int(t*10) % 360,
		}
		publishJSON(nc, "mavlink.vfr_hud", hud)

		att := map[string]interface{}{
			"roll":  math.Sin(t) * 0.5,
			"pitch": math.Cos(t*0.5) * 0.3,
			"yaw":   t * 0.1,
		}
		publishJSON(nc, "mavlink.attitude", att)

		sysStatus := map[string]interface{}{
			"voltage_battery": 12000 + math.Sin(t)*500,
		}
		publishJSON(nc, "mavlink.sys_status", sysStatus)

		gps := map[string]interface{}{
			"satellites_visible": 8 + int(math.Sin(t)*2),
		}
		publishJSON(nc, "mavlink.gps_raw_int", gps)

		gpos := map[string]interface{}{
			"lat":          37.7749 + math.Sin(t)*0.001,
			"lon":          -122.4194 + math.Cos(t)*0.001,
			"alt":          10.0 + math.Cos(t),
			"relative_alt": 5.0 + math.Cos(t),
			"hdg":          (int(t*10) % 360) * 100,
		}
		publishJSON(nc, "mavlink.global_position_int", gpos)
	}
}

func publishJSON(nc *nats.Conn, subject string, data interface{}) {
	b, _ := json.Marshal(data)
	nc.Publish(subject, b)
}

type multiWriter struct {
	data []byte
}

func (m *multiWriter) Write(p []byte) (n int, err error) {
	m.data = append(m.data, p...)
	return len(p), nil
}

func (m *multiWriter) Len() int {
	return len(m.data)
}

func (m *multiWriter) Bytes() []byte {
	return m.data
}

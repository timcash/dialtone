package mock

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"net/http"
	"time"

	"dialtone/cli/src/core/logger"
)

type MavlinkNatsMsg struct {
	Subject string
	Data    []byte
}

var MavlinkPubChan = make(chan MavlinkNatsMsg, 100)

// StartMockMavlink periodically publishes fake telemetry to MavlinkPubChan
func StartMockMavlink(natsPort int) {
	logger.LogInfo("Starting Mock Mavlink Publisher...")

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		start := time.Now()

		for range ticker.C {
			t := time.Since(start).Seconds()

			// 1. Heartbeat
			heartbeat := map[string]interface{}{
				"type":          "HEARTBEAT",
				"mav_type":      10, // MAV_TYPE_GROUND_ROVER
				"autopilot":     3,  // MAV_AUTOPILOT_ARDUPILOTMEGA
				"base_mode":     209,
				"custom_mode":   5,
				"system_status": 4,
				"timestamp":     time.Now().UnixMilli(),
				"t_raw":         time.Now().UnixMilli(),
				"t_pub":         time.Now().UnixMilli(),
			}
			PublishMockJSON("mavlink.heartbeat", heartbeat)

			// 2. Global Position
			gpos := map[string]interface{}{
				"lat":          37.7749 + math.Sin(t)*0.0001,
				"lon":          -122.4194 + math.Cos(t)*0.0001,
				"alt":          10.0 + math.Cos(t),
				"relative_alt": 5.0 + math.Cos(t),
				"vx":           math.Cos(t),
				"vy":           math.Sin(t),
				"vz":           0.0,
				"hdg":          float64(int(t*10) % 360),
				"t_raw":        time.Now().UnixMilli(),
				"t_pub":        time.Now().UnixMilli(),
			}
			PublishMockJSON("mavlink.global_position_int", gpos)

			// 3. Attitude
			att := map[string]interface{}{
				"roll":       math.Sin(t) * 0.1,
				"pitch":      math.Cos(t*0.5) * 0.05,
				"yaw":        t * 0.1,
				"rollspeed":  0.0,
				"pitchspeed": 0.0,
				"yawspeed":   0.0,
				"t_raw":      time.Now().UnixMilli(),
				"t_pub":      time.Now().UnixMilli(),
			}
			PublishMockJSON("mavlink.attitude", att)
		}
	}()
}

func PublishMockJSON(subject string, data interface{}) {
	b, err := json.Marshal(data)
	if err != nil {
		return
	}

	select {
	case MavlinkPubChan <- MavlinkNatsMsg{Subject: subject, Data: b}:
	default:
		// Drop message if channel full
	}
}

// MockStreamHandler serves a fake MJPEG stream with a moving gradient
func MockStreamHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")

	ticker := time.NewTicker(100 * time.Millisecond) // 10 FPS
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			// Simple moving gradient
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
		}
	}
}

// multiWriter is a simple buffer that implements io.Writer
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

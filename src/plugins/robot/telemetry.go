package robot

import (
	"dialtone/cli/src/core/logger"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
)

func RunTelemetry(args []string) {
	port := 4222
	// If ROBOT_HOST is set, we are likely on a robot where nats is on 14222
	if os.Getenv("TS_AUTHKEY") != "" {
		port = 14222
		// Try to discover port dynamically
		resp, err := http.Get("http://127.0.0.1:8080/api/init")
		if err == nil {
			defer resp.Body.Close()
			var initData struct {
				InternalNatsPort int `json:"internal_nats_port"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&initData); err == nil && initData.InternalNatsPort > 0 {
				port = initData.InternalNatsPort
				logger.LogInfo("[TELEMETRY] Discovered internal NATS port: %d", port)
			}
		}
	}

	for i, arg := range args {
		if arg == "--port" && i+1 < len(args) {
			if p, err := strconv.Atoi(args[i+1]); err == nil {
				port = p
			}
		}
	}

	natsURL := fmt.Sprintf("nats://127.0.0.1:%d", port)
	logger.LogInfo("[TELEMETRY] Monitoring MAVLink latency on %s...", natsURL)
	nc, err := nats.Connect(natsURL)
	if err != nil {
		logger.LogFatal("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	fmt.Printf("%-25s | %-8s | %-8s | %-8s\n", "SUBJECT", "P (ms)", "Q (ms)", "TOTAL (local)")
	fmt.Println("-------------------------------------------------------")

	nc.Subscribe("mavlink.>", func(m *nats.Msg) {
		var data map[string]any
		if err := json.Unmarshal(m.Data, &data); err != nil {
			return
		}

		t_raw_any, ok1 := data["t_raw"]
		t_pub_any, ok2 := data["t_pub"]
		if ok1 && ok2 {
			t_raw := float64(0)
			t_pub := float64(0)

			switch v := t_raw_any.(type) {
			case float64:
				t_raw = v
			case int64:
				t_raw = float64(v)
			}
			switch v := t_pub_any.(type) {
			case float64:
				t_pub = v
			case int64:
				t_pub = float64(v)
			}

			if t_raw > 0 && t_pub > 0 {
				now := float64(time.Now().UnixMilli())
				p := t_pub - t_raw
				q := now - t_pub
				total := now - t_raw
				fmt.Printf("%-25s | %-8.1f | %-8.1f | %-8.1f\n", m.Subject, p, q, total)
			}
		}
	})

	select {}
}

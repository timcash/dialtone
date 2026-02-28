package main

import (
	logs "dialtone/dev/plugins/logs/src_v1/go"
	mavlinkapp "dialtone/dev/plugins/mavlink/app"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bluenviron/gomavlib/v3/pkg/dialects/common"
	"github.com/nats-io/nats.go"
)

type roverCommand struct {
	Cmd  string `json:"cmd"`
	Mode string `json:"mode"`
}

func main() {
	logs.SetOutput(os.Stdout)
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "version":
		logs.Raw("mavlink_v1")
	case "run":
		if err := run(os.Args[2:]); err != nil {
			logs.Error("mavlink run failed: %v", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		usage()
	default:
		logs.Error("unknown command: %s", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	endpoint := fs.String("endpoint", envOrDefault("MAVLINK_ENDPOINT", ""), "MAVLink endpoint (serial:/dev/...:baud or udp:host:port)")
	natsURL := fs.String("nats-url", "nats://127.0.0.1:4222", "NATS URL")
	mockIfNoEndpoint := fs.Bool("mock-if-no-endpoint", true, "Publish mock heartbeat if endpoint not set")
	if err := fs.Parse(args); err != nil {
		return err
	}

	nc, err := nats.Connect(strings.TrimSpace(*natsURL), nats.Timeout(2*time.Second))
	if err != nil {
		return err
	}
	defer nc.Close()
	go publishServiceHeartbeat(nc)

	if strings.TrimSpace(*endpoint) == "" {
		if !*mockIfNoEndpoint {
			return fmt.Errorf("mavlink endpoint is required")
		}
		return runMockHeartbeat(nc)
	}

	svc, err := mavlinkapp.NewMavlinkService(mavlinkapp.MavlinkConfig{
		Endpoint: strings.TrimSpace(*endpoint),
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
		if *mockIfNoEndpoint {
			logs.Warn("mavlink endpoint unavailable, using mock heartbeat: %v", err)
			return runMockHeartbeat(nc)
		}
		return err
	}
	defer svc.Close()

	startRoverCommandConsumer(nc, svc)
	logs.Info("mavlink_v1 bridge started endpoint=%s nats=%s", *endpoint, *natsURL)
	svc.Start()
	return nil
}

func startRoverCommandConsumer(nc *nats.Conn, svc *mavlinkapp.MavlinkService) {
	_, err := nc.Subscribe("rover.command", func(msg *nats.Msg) {
		var cmd roverCommand
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			logs.Error("rover.command decode error: %v", err)
			return
		}
		switch strings.ToLower(strings.TrimSpace(cmd.Cmd)) {
		case "arm":
			if err := svc.Arm(); err != nil {
				logs.Error("rover.command arm failed: %v", err)
			}
		case "disarm":
			if err := svc.Disarm(); err != nil {
				logs.Error("rover.command disarm failed: %v", err)
			}
		case "mode":
			if err := svc.SetMode(strings.TrimSpace(cmd.Mode)); err != nil {
				logs.Error("rover.command mode failed: %v", err)
			}
		case "pulse_fwd":
			go func() {
				if err := svc.PulseForward(); err != nil {
					logs.Error("rover.command pulse_fwd failed: %v", err)
				}
			}()
		default:
			logs.Warn("rover.command unknown cmd=%q", cmd.Cmd)
		}
	})
	if err != nil {
		logs.Error("rover.command subscription failed: %v", err)
		return
	}
	_ = nc.Flush()
}

func runMockHeartbeat(nc *nats.Conn) error {
	logs.Warn("mavlink_v1 running in mock mode")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for t := range ticker.C {
		payload := map[string]any{
			"type":      "HEARTBEAT",
			"timestamp": t.UnixMilli(),
			"source":    "mavlink_v1_mock",
		}
		data, _ := json.Marshal(payload)
		_ = nc.Publish("mavlink.heartbeat", data)
		_ = nc.Flush()
	}
	return nil
}

func publishServiceHeartbeat(nc *nats.Conn) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for t := range ticker.C {
		payload := map[string]any{
			"type":      "SERVICE_HEARTBEAT",
			"timestamp": t.UnixMilli(),
			"source":    "mavlink_v1",
		}
		data, _ := json.Marshal(payload)
		_ = nc.Publish("mavlink.service", data)
		_ = nc.Flush()
	}
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
		return "mavlink.heartbeat", map[string]any{"type": "HEARTBEAT", "mav_type": msg.Type, "custom_mode": msg.CustomMode, "timestamp": now, "t_raw": now}
	case *common.MessageAttitude:
		return "mavlink.attitude", map[string]any{"type": "ATTITUDE", "roll": msg.Roll, "pitch": msg.Pitch, "yaw": msg.Yaw, "rollspeed": msg.Rollspeed, "pitchspeed": msg.Pitchspeed, "yawspeed": msg.Yawspeed, "timestamp": now, "t_raw": now}
	case *common.MessageGlobalPositionInt:
		var hdg float64 = -1
		if msg.Hdg != 65535 {
			hdg = float64(msg.Hdg) / 100.0
		}
		return "mavlink.global_position_int", map[string]any{"type": "GLOBAL_POSITION_INT", "lat": float64(msg.Lat) / 1e7, "lon": float64(msg.Lon) / 1e7, "alt": float64(msg.Alt) / 1000.0, "relative_alt": float64(msg.RelativeAlt) / 1000.0, "vx": float64(msg.Vx) / 100.0, "vy": float64(msg.Vy) / 100.0, "vz": float64(msg.Vz) / 100.0, "hdg": hdg, "timestamp": now, "t_raw": now}
	case *common.MessageStatustext:
		text := strings.TrimRight(string(msg.Text[:]), "\x00")
		return "mavlink.statustext", map[string]any{"type": "STATUSTEXT", "severity": msg.Severity, "text": text, "timestamp": now, "t_raw": now}
	case *common.MessageCommandAck:
		return "mavlink.command_ack", map[string]any{"type": "COMMAND_ACK", "command": msg.Command, "result": msg.Result, "timestamp": now, "t_raw": now}
	default:
		return "", nil
	}
}

func envOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func usage() {
	logs.Raw("Usage: dialtone_mavlink_v1 <command>")
	logs.Raw("Commands:")
	logs.Raw("  run [--endpoint MAVLINK_ENDPOINT] [--nats-url URL] [--mock-if-no-endpoint]")
	logs.Raw("  version")
}

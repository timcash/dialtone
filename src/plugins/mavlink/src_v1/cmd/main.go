package main

import (
	logs "dialtone/dev/plugins/logs/src_v1/go"
	mavlinkapp "dialtone/dev/plugins/mavlink/app"
	sshplugin "dialtone/dev/plugins/ssh/src_v1/go"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/common"
	"github.com/nats-io/nats.go"
)

type roverCommand struct {
	Cmd          string `json:"cmd"`
	Mode         string `json:"mode"`
	DurationMs   int    `json:"durationMs,omitempty"`
	ThrottlePWM  int    `json:"throttlePwm,omitempty"`
	SteeringPWM  int    `json:"steeringPwm,omitempty"`
	NoStop       bool   `json:"noStop,omitempty"`
	SteeringOnly bool   `json:"steeringOnly,omitempty"`
}

const defaultRoverKeyParamsCSV = "RCMAP_STEERING,RCMAP_THROTTLE,RCMAP_ROLL,RCMAP_PITCH,RCMAP_YAW,RC1_MIN,RC1_TRIM,RC1_MAX,RC3_MIN,RC3_TRIM,RC3_MAX,SERVO1_FUNCTION,SERVO1_MIN,SERVO1_TRIM,SERVO1_MAX,SERVO3_FUNCTION,SERVO3_MIN,SERVO3_TRIM,SERVO3_MAX,CRUISE_SPEED,CRUISE_THROTTLE,WP_SPEED"

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
	case "params":
		if err := params(os.Args[2:]); err != nil {
			logs.Error("mavlink params failed: %v", err)
			os.Exit(1)
		}
	case "key-params":
		if err := keyParams(os.Args[2:]); err != nil {
			logs.Error("mavlink key-params failed: %v", err)
			os.Exit(1)
		}
	case "stream":
		if err := stream(os.Args[2:]); err != nil {
			logs.Error("mavlink stream failed: %v", err)
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

func params(args []string) error {
	return runParams(args, "")
}

func keyParams(args []string) error {
	return runParams(args, defaultRoverKeyParamsCSV)
}

func runParams(args []string, defaultNamesCSV string) error {
	fs := flag.NewFlagSet("params", flag.ContinueOnError)
	endpoint := fs.String("endpoint", envOrDefault("MAVLINK_ENDPOINT", ""), "MAVLink endpoint (serial:/dev/...:baud or udp:host:port)")
	defaultNames := strings.TrimSpace(defaultNamesCSV)
	if defaultNames == "" {
		defaultNames = "RCMAP_STEERING,RCMAP_THROTTLE,RCMAP_ROLL,RCMAP_PITCH,RCMAP_YAW"
	}
	namesCSV := fs.String("names", defaultNames, "CSV parameter names to query")
	timeout := fs.Duration("timeout", 10*time.Second, "Total wait timeout")
	targetSystem := fs.Int("target-system", 0, "Target autopilot system ID (0=broadcast)")
	targetComponent := fs.Int("target-component", 0, "Target autopilot component ID (0=broadcast)")
	asJSON := fs.Bool("json", false, "Emit JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ep := strings.TrimSpace(*endpoint)
	if ep == "" {
		return fmt.Errorf("mavlink endpoint is required (set --endpoint or MAVLINK_ENDPOINT)")
	}
	names := splitParamNames(*namesCSV)
	if len(names) == 0 {
		return fmt.Errorf("no parameter names specified")
	}

	endpointConf, err := parseMAVLinkEndpoint(ep)
	if err != nil {
		return err
	}
	node := &gomavlib.Node{
		Endpoints:   []gomavlib.EndpointConf{endpointConf},
		Dialect:     common.Dialect,
		OutVersion:  gomavlib.V2,
		OutSystemID: 255,
	}
	if err := node.Initialize(); err != nil {
		return err
	}
	defer node.Close()

	want := make(map[string]struct{}, len(names))
	for _, name := range names {
		want[name] = struct{}{}
	}
	values := make(map[string]float32, len(names))

	sendRequests := func() {
		for _, name := range names {
			_ = node.WriteMessageAll(&common.MessageParamRequestRead{
				TargetSystem:    uint8(*targetSystem),
				TargetComponent: uint8(*targetComponent),
				ParamId:         name,
				ParamIndex:      -1,
			})
			time.Sleep(30 * time.Millisecond)
		}
	}
	sendRequests()
	sendRequests()

	deadline := time.Now().Add(*timeout)
	for time.Now().Before(deadline) {
		if len(values) == len(want) {
			break
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		select {
		case evt := <-node.Events():
			frame, ok := evt.(*gomavlib.EventFrame)
			if !ok {
				continue
			}
			paramValue, ok := frame.Message().(*common.MessageParamValue)
			if !ok {
				continue
			}
			paramID := strings.TrimSpace(strings.TrimRight(paramValue.ParamId, "\x00"))
			if _, ok := want[paramID]; ok {
				values[paramID] = paramValue.ParamValue
			}
		case <-time.After(minDuration(150*time.Millisecond, remaining)):
		}
	}

	if *asJSON {
		missing := make([]string, 0, len(names))
		for _, name := range names {
			if _, ok := values[name]; !ok {
				missing = append(missing, name)
			}
		}
		out := map[string]any{
			"endpoint": ep,
			"params":   values,
			"missing":  missing,
		}
		raw, _ := json.MarshalIndent(out, "", "  ")
		logs.Raw("%s", string(raw))
	} else {
		logs.Raw("endpoint=%s", ep)
		for _, name := range names {
			if v, ok := values[name]; ok {
				logs.Raw("%s=%.0f", name, v)
			} else {
				logs.Raw("%s=<no-response>", name)
			}
		}
	}

	if len(values) == 0 {
		return fmt.Errorf("no PARAM_VALUE responses received (endpoint busy or autopilot not responding)")
	}
	return nil
}

func parseMAVLinkEndpoint(endpoint string) (gomavlib.EndpointConf, error) {
	switch {
	case strings.HasPrefix(endpoint, "serial:"):
		parts := strings.Split(endpoint, ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid serial endpoint format. expected serial:/dev/ttyXXX:baud, got %q", endpoint)
		}
		baud, err := strconv.Atoi(strings.TrimSpace(parts[2]))
		if err != nil {
			return nil, fmt.Errorf("invalid serial baud in endpoint %q: %w", endpoint, err)
		}
		return gomavlib.EndpointSerial{Device: strings.TrimSpace(parts[1]), Baud: baud}, nil
	case strings.HasPrefix(endpoint, "udp:"):
		return gomavlib.EndpointUDPServer{Address: strings.TrimSpace(strings.TrimPrefix(endpoint, "udp:"))}, nil
	case strings.HasPrefix(endpoint, "tcp:"):
		return gomavlib.EndpointTCPClient{Address: strings.TrimSpace(strings.TrimPrefix(endpoint, "tcp:"))}, nil
	default:
		return nil, fmt.Errorf("unsupported MAVLINK_ENDPOINT %q", endpoint)
	}
}

func splitParamNames(csv string) []string {
	parts := strings.Split(strings.TrimSpace(csv), ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.ToUpper(strings.TrimSpace(p))
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func run(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	endpoint := fs.String("endpoint", envOrDefault("MAVLINK_ENDPOINT", ""), "MAVLink endpoint (serial:/dev/...:baud or udp:host:port)")
	natsURL := fs.String("nats-url", "nats://127.0.0.1:4222", "NATS URL")
	natsConnectTimeout := fs.Duration("nats-connect-timeout", 30*time.Second, "Max time to wait for initial NATS connection")
	mockIfNoEndpoint := fs.Bool("mock-if-no-endpoint", true, "Publish mock heartbeat if endpoint not set")
	if err := fs.Parse(args); err != nil {
		return err
	}

	nc, err := connectNATSWithRetry(strings.TrimSpace(*natsURL), *natsConnectTimeout)
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

	endpointValue := strings.TrimSpace(*endpoint)
	retryDelay := 1 * time.Second
	const retryDelayMax = 10 * time.Second
	for {
		svc, err := mavlinkapp.NewMavlinkService(mavlinkapp.MavlinkConfig{
			Endpoint: endpointValue,
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
			logs.Warn("mavlink endpoint connect failed endpoint=%s: %v", endpointValue, err)
			time.Sleep(retryDelay)
			retryDelay = minDuration(retryDelay*2, retryDelayMax)
			continue
		}
		retryDelay = 1 * time.Second
		sub, subErr := startRoverCommandConsumer(nc, svc)
		if subErr != nil {
			svc.Close()
			return fmt.Errorf("rover.command subscribe failed: %w", subErr)
		}
		logs.Info("mavlink_v1 bridge started endpoint=%s nats=%s", endpointValue, *natsURL)
		svc.Start()
		_ = sub.Unsubscribe()
		svc.Close()
		logs.Warn("mavlink_v1 endpoint stream ended; reconnecting in %s", retryDelay)
		time.Sleep(retryDelay)
		retryDelay = minDuration(retryDelay*2, retryDelayMax)
	}
}

func stream(args []string) error {
	fs := flag.NewFlagSet("stream", flag.ContinueOnError)
	host := fs.String("host", "", "SSH mesh host alias (for example rover)")
	user := fs.String("user", "", "SSH user override")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	port := fs.String("port", "", "SSH port override")
	remoteNATSPort := fs.Int("remote-nats-port", 18236, "Remote NATS TCP port on host")
	duration := fs.Duration("duration", 12*time.Second, "How long to stream MAVLink subjects")
	cmdName := fs.String("cmd", "", "Optional rover command to publish (for example mode, drive_up, stop)")
	cmdMode := fs.String("mode", "", "Optional mode value when --cmd=mode")
	throttlePWM := fs.Int("throttle-pwm", 0, "Optional rover.command throttlePwm")
	steeringPWM := fs.Int("steering-pwm", 0, "Optional rover.command steeringPwm")
	durationMS := fs.Int("duration-ms", 0, "Optional rover.command durationMs")
	noStop := fs.Bool("no-stop", false, "Optional rover.command noStop=true")
	steeringOnly := fs.Bool("steering-only", false, "Optional rover.command steeringOnly=true")
	if err := fs.Parse(args); err != nil {
		return err
	}
	target := strings.TrimSpace(*host)
	if target == "" {
		return fmt.Errorf("stream requires --host")
	}
	node, err := sshplugin.ResolveMeshNode(target)
	if err != nil {
		return err
	}
	client, node, hostAddr, usePort, err := sshplugin.DialMeshNode(node.Name, sshplugin.CommandOptions{
		User:     strings.TrimSpace(*user),
		Port:     strings.TrimSpace(*port),
		Password: strings.TrimSpace(*pass),
	})
	if err != nil {
		return err
	}
	defer client.Close()
	logs.Info("mavlink stream dialing host=%s addr=%s port=%s", node.Name, hostAddr, usePort)

	localPort, err := allocateLocalPort()
	if err != nil {
		return err
	}
	localAddr := fmt.Sprintf("127.0.0.1:%d", localPort)
	remoteAddr := fmt.Sprintf("127.0.0.1:%d", *remoteNATSPort)
	if err := sshplugin.ForwardRemoteToLocal(client, remoteAddr, localAddr); err != nil {
		return err
	}
	natsURL := fmt.Sprintf("nats://%s", localAddr)
	nc, err := nats.Connect(
		natsURL,
		nats.Timeout(2*time.Second),
		nats.ReconnectWait(500*time.Millisecond),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return err
	}
	defer nc.Close()

	if strings.TrimSpace(*cmdName) != "" {
		cmd := roverCommand{
			Cmd:          strings.TrimSpace(*cmdName),
			Mode:         strings.TrimSpace(*cmdMode),
			DurationMs:   *durationMS,
			ThrottlePWM:  *throttlePWM,
			SteeringPWM:  *steeringPWM,
			NoStop:       *noStop,
			SteeringOnly: *steeringOnly,
		}
		raw, _ := json.Marshal(cmd)
		if err := nc.Publish("rover.command", raw); err != nil {
			return err
		}
		_ = nc.Flush()
		logs.Info("published rover.command host=%s cmd=%s mode=%s", node.Name, cmd.Cmd, cmd.Mode)
	}

	msgCh := make(chan *nats.Msg, 256)
	sub, err := nc.Subscribe("mavlink.>", func(m *nats.Msg) {
		select {
		case msgCh <- m:
		default:
		}
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	if err := nc.Flush(); err != nil {
		return err
	}

	deadline := time.Now().Add(*duration)
	for time.Now().Before(deadline) {
		select {
		case m := <-msgCh:
			logs.Raw("[%s] %s %s", time.Now().UTC().Format(time.RFC3339Nano), m.Subject, strings.TrimSpace(string(m.Data)))
		case <-time.After(300 * time.Millisecond):
		}
	}
	logs.Info("mavlink stream complete host=%s duration=%s", node.Name, duration.String())
	return nil
}

func connectNATSWithRetry(url string, maxWait time.Duration) (*nats.Conn, error) {
	if strings.TrimSpace(url) == "" {
		return nil, fmt.Errorf("nats url is required")
	}
	deadline := time.Now().Add(maxWait)
	var lastErr error
	attempt := 0
	for {
		attempt++
		nc, err := nats.Connect(
			url,
			nats.Timeout(2*time.Second),
			nats.ReconnectWait(1*time.Second),
			nats.MaxReconnects(-1),
		)
		if err == nil {
			if attempt > 1 {
				logs.Info("mavlink_v1 connected to NATS after %d attempts: %s", attempt, url)
			}
			return nc, nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			break
		}
		if attempt == 1 || attempt%5 == 0 {
			logs.Warn("mavlink_v1 waiting for NATS (%s): attempt=%d err=%v", url, attempt, err)
		}
		time.Sleep(800 * time.Millisecond)
	}
	return nil, fmt.Errorf("nats connect failed after %s: %w", maxWait, lastErr)
}

func startRoverCommandConsumer(nc *nats.Conn, svc *mavlinkapp.MavlinkService) (*nats.Subscription, error) {
	sub, err := nc.Subscribe("rover.command", func(msg *nats.Msg) {
		var cmd roverCommand
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			logs.Error("rover.command decode error: %v", err)
			return
		}
		logs.Info("rover.command received cmd=%q mode=%q", strings.TrimSpace(cmd.Cmd), strings.TrimSpace(cmd.Mode))
		resolvePWM := func(value, fallback int) uint16 {
			v := value
			if v == 0 {
				v = fallback
			}
			if v < 1000 {
				v = 1000
			}
			if v > 2000 {
				v = 2000
			}
			return uint16(v)
		}
		resolveDuration := func(value, fallback int) time.Duration {
			v := value
			if v == 0 {
				v = fallback
			}
			if v < 200 {
				v = 200
			}
			if v > 5000 {
				v = 5000
			}
			return time.Duration(v) * time.Millisecond
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
		case "drive_up":
			go func() {
				if cmd.DurationMs != 0 || cmd.ThrottlePWM != 0 || cmd.SteeringPWM != 0 {
					throttle := resolvePWM(cmd.ThrottlePWM, 2000)
					steering := resolvePWM(cmd.SteeringPWM, 1500)
					dur := resolveDuration(cmd.DurationMs, 2000)
					if cmd.NoStop {
						if err := svc.PulseCustomNoStop(throttle, steering, dur, "PulseForwardCustomNoStop"); err != nil {
							logs.Error("rover.command drive_up custom (noStop) failed: %v", err)
						}
						return
					}
					if err := svc.PulseCustom(throttle, steering, dur, "PulseForwardCustom"); err != nil {
						logs.Error("rover.command drive_up custom failed: %v", err)
					}
					return
				}
				if err := svc.PulseForward(); err != nil {
					logs.Error("rover.command drive_up failed: %v", err)
				}
			}()
		case "drive_down":
			go func() {
				if cmd.DurationMs != 0 || cmd.ThrottlePWM != 0 || cmd.SteeringPWM != 0 {
					throttle := resolvePWM(cmd.ThrottlePWM, 1000)
					steering := resolvePWM(cmd.SteeringPWM, 1500)
					dur := resolveDuration(cmd.DurationMs, 2000)
					if cmd.NoStop {
						if err := svc.PulseCustomNoStop(throttle, steering, dur, "PulseReverseCustomNoStop"); err != nil {
							logs.Error("rover.command drive_down custom (noStop) failed: %v", err)
						}
						return
					}
					if err := svc.PulseCustom(throttle, steering, dur, "PulseReverseCustom"); err != nil {
						logs.Error("rover.command drive_down custom failed: %v", err)
					}
					return
				}
				if err := svc.PulseReverse(); err != nil {
					logs.Error("rover.command drive_down failed: %v", err)
				}
			}()
		case "drive_left":
			go func() {
				if cmd.DurationMs != 0 || cmd.ThrottlePWM != 0 || cmd.SteeringPWM != 0 {
					throttle := resolvePWM(cmd.ThrottlePWM, 1800)
					steering := resolvePWM(cmd.SteeringPWM, 1000)
					dur := resolveDuration(cmd.DurationMs, 1200)
					if cmd.NoStop && cmd.SteeringOnly {
						if err := svc.PulseSteeringNoStop(steering, dur, "PulseLeftSteeringOnlyNoStop"); err != nil {
							logs.Error("rover.command drive_left steering-only (noStop) failed: %v", err)
						}
						return
					}
					if cmd.NoStop {
						if err := svc.PulseCustomNoStop(throttle, steering, dur, "PulseLeftCustomNoStop"); err != nil {
							logs.Error("rover.command drive_left custom (noStop) failed: %v", err)
						}
						return
					}
					if err := svc.PulseCustom(throttle, steering, dur, "PulseLeftCustom"); err != nil {
						logs.Error("rover.command drive_left custom failed: %v", err)
					}
					return
				}
				if err := svc.PulseLeft(); err != nil {
					logs.Error("rover.command drive_left failed: %v", err)
				}
			}()
		case "drive_right":
			go func() {
				if cmd.DurationMs != 0 || cmd.ThrottlePWM != 0 || cmd.SteeringPWM != 0 {
					throttle := resolvePWM(cmd.ThrottlePWM, 1800)
					steering := resolvePWM(cmd.SteeringPWM, 2000)
					dur := resolveDuration(cmd.DurationMs, 1200)
					if cmd.NoStop && cmd.SteeringOnly {
						if err := svc.PulseSteeringNoStop(steering, dur, "PulseRightSteeringOnlyNoStop"); err != nil {
							logs.Error("rover.command drive_right steering-only (noStop) failed: %v", err)
						}
						return
					}
					if cmd.NoStop {
						if err := svc.PulseCustomNoStop(throttle, steering, dur, "PulseRightCustomNoStop"); err != nil {
							logs.Error("rover.command drive_right custom (noStop) failed: %v", err)
						}
						return
					}
					if err := svc.PulseCustom(throttle, steering, dur, "PulseRightCustom"); err != nil {
						logs.Error("rover.command drive_right custom failed: %v", err)
					}
					return
				}
				if err := svc.PulseRight(); err != nil {
					logs.Error("rover.command drive_right failed: %v", err)
				}
			}()
		case "stop", "stop_motion", "halt":
			go func() {
				if err := svc.StopMotion(); err != nil {
					logs.Error("rover.command stop failed: %v", err)
				}
			}()
		case "guided_hold":
			go func() {
				if err := svc.StopMotion(); err != nil {
					logs.Error("rover.command guided_hold failed: %v", err)
				}
			}()
		case "guided_forward_1m":
			go func() {
				if err := svc.SetMode("GUIDED"); err != nil {
					logs.Error("rover.command guided_forward_1m mode set failed: %v", err)
					return
				}
				if err := svc.GuidedForwardMeters(1.0); err != nil {
					logs.Error("rover.command guided_forward_1m failed: %v", err)
				}
			}()
		case "guided_square_5m":
			go func() {
				if err := svc.SetMode("GUIDED"); err != nil {
					logs.Error("rover.command guided_square_5m mode set failed: %v", err)
					return
				}
				if err := svc.GuidedSquareMeters(5.0); err != nil {
					logs.Error("rover.command guided_square_5m failed: %v", err)
				}
			}()
		default:
			logs.Warn("rover.command unknown cmd=%q", cmd.Cmd)
		}
	})
	if err != nil {
		return nil, err
	}
	_ = nc.Flush()
	return sub, nil
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
	case *mavlinkapp.ControlFeedback:
		return "mavlink.control_feedback", map[string]any{
			"type":             "CONTROL_FEEDBACK",
			"source":           msg.Source,
			"steering_channel": msg.SteeringChannel,
			"throttle_channel": msg.ThrottleChannel,
			"steering_raw":     msg.SteeringRaw,
			"throttle_raw":     msg.ThrottleRaw,
			"timestamp":        now,
			"t_raw":            now,
		}
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
	logs.Raw("  stream --host rover [--cmd stop|mode|drive_up ...] [--duration 12s]")
	logs.Raw("  params [--endpoint MAVLINK_ENDPOINT] [--names CSV] [--timeout 10s] [--target-system 0] [--target-component 0] [--json]")
	logs.Raw("  key-params [--endpoint MAVLINK_ENDPOINT] [--timeout 10s] [--target-system 0] [--target-component 0] [--json]")
	logs.Raw("  version")
}

func allocateLocalPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	addr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("failed to allocate local tcp addr")
	}
	return addr.Port, nil
}

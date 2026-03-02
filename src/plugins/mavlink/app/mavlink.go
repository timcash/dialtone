package mavlink

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"dialtone/dev/plugins/logs/src_v1/go"
	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/common"
)

// MavlinkConfig holds configuration for the MAVLink service
type MavlinkConfig struct {
	Endpoint string
	Callback func(*MavlinkEvent)
}

// MavlinkEvent represents a simplified event from MAVLink
type MavlinkEvent struct {
	Type       string
	Data       interface{}
	ReceivedAt int64
}

// ControlFeedback reports observed steering/throttle values from MAVLink telemetry.
type ControlFeedback struct {
	Source          string
	SteeringChannel uint8
	ThrottleChannel uint8
	SteeringRaw     uint16
	ThrottleRaw     uint16
}

// MavlinkService handles MAVLink communication
type MavlinkService struct {
	node            *gomavlib.Node
	config          MavlinkConfig
	steeringChannel uint8
	throttleChannel uint8
	lastDiagLog     time.Time
	targetMu        sync.RWMutex
	targetSystem    uint8
	targetComponent uint8
	posMu           sync.RWMutex
	latestLatDeg    float64
	latestLonDeg    float64
	latestRelAltM   float32
	havePosition    bool
}

// NewMavlinkService creates a new MAVLink service
func NewMavlinkService(config MavlinkConfig) (*MavlinkService, error) {
	// Parse endpoint string to determine type
	// Supported formats:
	// - serial:/dev/ttyAMA0:57600
	// - udp:0.0.0.0:14550 (Server)
	// - tcp:127.0.0.1:5760 (Client)

	var endpoints []gomavlib.EndpointConf

	if strings.HasPrefix(config.Endpoint, "serial:") {
		parts := strings.Split(config.Endpoint, ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid serial endpoint format. Expected serial:port:baud, got %s", config.Endpoint)
		}
		port := parts[1]
		baud, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid baud rate: %v", err)
		}
		endpoints = []gomavlib.EndpointConf{
			gomavlib.EndpointSerial{Device: port, Baud: baud},
		}
	} else if strings.HasPrefix(config.Endpoint, "udp:") {
		addr := strings.TrimPrefix(config.Endpoint, "udp:")
		endpoints = []gomavlib.EndpointConf{
			gomavlib.EndpointUDPServer{Address: addr},
		}
	} else if strings.HasPrefix(config.Endpoint, "tcp:") {
		addr := strings.TrimPrefix(config.Endpoint, "tcp:")
		endpoints = []gomavlib.EndpointConf{
			gomavlib.EndpointTCPClient{Address: addr},
		}
	} else {
		return nil, fmt.Errorf("unsupported or invalid endpoint: %s", config.Endpoint)
	}

	node := &gomavlib.Node{
		Endpoints:   endpoints,
		Dialect:     common.Dialect,
		OutVersion:  gomavlib.V2,
		OutSystemID: 255, // GCS-like sender ID; improves ArduPilot RC override compatibility
	}
	err := node.Initialize()
	if err != nil {
		return nil, err
	}

	svc := &MavlinkService{
		node:            node,
		config:          config,
		steeringChannel: 1,
		throttleChannel: 3,
		targetSystem:    1,
		targetComponent: 1,
	}
	svc.requestRCMap()
	return svc, nil
}

// Start starts the MAVLink event loop
func (s *MavlinkService) Start() {
	// defer s.node.Close() // handled by Close()
	logs.Info("MavlinkService: Starting event loop on %s", s.config.Endpoint)

	for evt := range s.node.Events() {
		receivedAt := time.Now().UnixMilli()
		switch e := evt.(type) {
		case *gomavlib.EventFrame:
			s.updateTargetIDs(e.SystemID(), e.ComponentID())
			// LOG EVERY FRAME FOR DEBUGGING
			// logs.Info("[MAVLINK-RAW] Frame from sys %d comp %d at %v", e.SystemID(), e.ComponentID(), receivedAt)

			msg := e.Message()
			msgType := fmt.Sprintf("%T", msg)
			if strings.Contains(msgType, "Message") {
				parts := strings.Split(msgType, "Message")
				if len(parts) > 1 {
					msgType = strings.ToUpper(parts[1])
				}
			}
			logs.Info("[MAVLINK-RAW] %s received at %v", msgType, receivedAt)

			switch msg := e.Message().(type) {
			case *common.MessageHeartbeat:
				armed := (uint8(msg.BaseMode) & 0x80) != 0 // MAV_MODE_FLAG_SAFETY_ARMED
				logs.Info("[MAVLINK-RAW] HEARTBEAT mode=%d armed=%t status=%d received_at=%v", msg.CustomMode, armed, msg.SystemStatus, receivedAt)
				if s.config.Callback != nil {
					s.config.Callback(&MavlinkEvent{
						Type:       "HEARTBEAT",
						Data:       msg,
						ReceivedAt: receivedAt,
					})
				}
			case *common.MessageParamValue:
				paramID := strings.TrimSpace(strings.TrimRight(msg.ParamId, "\x00"))
				switch paramID {
				case "RCMAP_STEERING":
					ch := uint8(msg.ParamValue)
					if ch >= 1 && ch <= 8 && ch != s.steeringChannel {
						s.steeringChannel = ch
						logs.Info("MavlinkService: learned RCMAP_STEERING=ch%d", s.steeringChannel)
					}
				case "RCMAP_THROTTLE":
					ch := uint8(msg.ParamValue)
					if ch >= 1 && ch <= 8 && ch != s.throttleChannel {
						s.throttleChannel = ch
						logs.Info("MavlinkService: learned RCMAP_THROTTLE=ch%d", s.throttleChannel)
					}
				}
			case *common.MessageRcChannels:
				if time.Since(s.lastDiagLog) > 800*time.Millisecond {
					logs.Info("[MAVLINK-DIAG] RC ch1=%d ch2=%d ch3=%d ch4=%d rssi=%d", msg.Chan1Raw, msg.Chan2Raw, msg.Chan3Raw, msg.Chan4Raw, msg.Rssi)
					s.lastDiagLog = time.Now()
				}
				if s.config.Callback != nil {
					s.config.Callback(&MavlinkEvent{
						Type: "CONTROL_FEEDBACK",
						Data: &ControlFeedback{
							Source:          "RC_CHANNELS",
							SteeringChannel: s.steeringChannel,
							ThrottleChannel: s.throttleChannel,
							SteeringRaw:     s.readRCChannel(msg, s.steeringChannel),
							ThrottleRaw:     s.readRCChannel(msg, s.throttleChannel),
						},
						ReceivedAt: receivedAt,
					})
				}
			case *common.MessageServoOutputRaw:
				if time.Since(s.lastDiagLog) > 800*time.Millisecond {
					logs.Info("[MAVLINK-DIAG] SERVO port=%d s1=%d s2=%d s3=%d s4=%d", msg.Port, msg.Servo1Raw, msg.Servo2Raw, msg.Servo3Raw, msg.Servo4Raw)
					s.lastDiagLog = time.Now()
				}
				if s.config.Callback != nil {
					s.config.Callback(&MavlinkEvent{
						Type: "CONTROL_FEEDBACK",
						Data: &ControlFeedback{
							Source:          "SERVO_OUTPUT_RAW",
							SteeringChannel: s.steeringChannel,
							ThrottleChannel: s.throttleChannel,
							SteeringRaw:     s.readServoChannel(msg, s.steeringChannel),
							ThrottleRaw:     s.readServoChannel(msg, s.throttleChannel),
						},
						ReceivedAt: receivedAt,
					})
				}
			case *common.MessageCommandAck:
				logs.Info("[MAVLINK-RAW] COMMAND_ACK: cmd=%v res=%v", msg.Command, msg.Result)
				if s.config.Callback != nil {
					s.config.Callback(&MavlinkEvent{
						Type:       "COMMAND_ACK",
						Data:       msg,
						ReceivedAt: receivedAt,
					})
				}
			case *common.MessageStatustext:
				text := strings.TrimRight(string(msg.Text[:]), "\x00")
				logs.Info("[MAVLINK-RAW] STATUSTEXT: sev=%v text=%q", msg.Severity, text)
				if s.config.Callback != nil {
					s.config.Callback(&MavlinkEvent{
						Type:       "STATUSTEXT",
						Data:       msg,
						ReceivedAt: receivedAt,
					})
				}
			case *common.MessageGlobalPositionInt:
				s.posMu.Lock()
				s.latestLatDeg = float64(msg.Lat) / 1e7
				s.latestLonDeg = float64(msg.Lon) / 1e7
				s.latestRelAltM = float32(msg.RelativeAlt) / 1000.0
				s.havePosition = true
				s.posMu.Unlock()
				if s.config.Callback != nil {
					s.config.Callback(&MavlinkEvent{
						Type:       "GLOBAL_POSITION_INT",
						Data:       msg,
						ReceivedAt: receivedAt,
					})
				}
			case *common.MessageAttitude:
				if s.config.Callback != nil {
					s.config.Callback(&MavlinkEvent{
						Type:       "ATTITUDE",
						Data:       msg,
						ReceivedAt: receivedAt,
					})
				}
			}
		case *gomavlib.EventParseError:
			logs.Warn("MAVLink parse error: %v", e.Error)
		case *gomavlib.EventStreamRequested:
			logs.Info("MAVLink stream requested")
		case *gomavlib.EventChannelOpen:
			logs.Info("MAVLink channel open")
		case *gomavlib.EventChannelClose:
			logs.Info("MAVLink channel close")
		}
	}
}

// Close closes the MAVLink service
func (s *MavlinkService) Close() {
	s.node.Close()
}

func (s *MavlinkService) pulseRCOverride(throttlePWM, steeringPWM uint16, duration time.Duration) error {
	ticker := time.NewTicker(50 * time.Millisecond) // 20Hz
	defer ticker.Stop()
	deadline := time.Now().Add(duration)
	for {
		if err := s.node.WriteMessageAll(s.overrideMessageSelective(&steeringPWM, &throttlePWM, false)); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			break
		}
		<-ticker.C
	}
	return nil
}

// PulseCustom sends a timed RC override pulse with caller-supplied values, then stops.
func (s *MavlinkService) PulseCustom(throttlePWM, steeringPWM uint16, duration time.Duration, label string) error {
	logs.Info("MavlinkService: %s (%dms throttle=%d steering=%d @ 20Hz)", label, duration.Milliseconds(), throttlePWM, steeringPWM)
	if err := s.pulseRCOverride(throttlePWM, steeringPWM, duration); err != nil {
		return err
	}
	return s.StopMotion()
}

// PulseCustomNoStop sends a timed RC override pulse with caller-supplied values and does not auto-stop.
// Rover UI can chain these pulses to maintain steering+throttle continuously.
func (s *MavlinkService) PulseCustomNoStop(throttlePWM, steeringPWM uint16, duration time.Duration, label string) error {
	logs.Info("MavlinkService: %s (%dms throttle=%d steering=%d @ 20Hz, no-stop)", label, duration.Milliseconds(), throttlePWM, steeringPWM)
	return s.pulseRCOverride(throttlePWM, steeringPWM, duration)
}

// PulseSteeringNoStop sends steering-only RC override pulses and leaves throttle untouched.
func (s *MavlinkService) PulseSteeringNoStop(steeringPWM uint16, duration time.Duration, label string) error {
	logs.Info("MavlinkService: %s (%dms steering=%d @ 20Hz, steering-only no-stop)", label, duration.Milliseconds(), steeringPWM)
	ticker := time.NewTicker(50 * time.Millisecond) // 20Hz
	defer ticker.Stop()
	deadline := time.Now().Add(duration)
	for {
		if err := s.node.WriteMessageAll(s.overrideMessageSelective(&steeringPWM, nil, false)); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			break
		}
		<-ticker.C
	}
	return nil
}

// PulseForward streams full-forward throttle for 1 second via RC override (Channel 3),
// then returns to neutral. Streaming improves reliability on ArduRover.
func (s *MavlinkService) PulseForward() error {
	logs.Info("MavlinkService: PulseForward 2s (throttle=2000 steering=1500 @ 20Hz)")
	if err := s.pulseRCOverride(2000, 1500, 2*time.Second); err != nil {
		return err
	}
	return s.StopMotion()
}

// PulseReverse streams full-reverse throttle for 1 second then returns to neutral.
func (s *MavlinkService) PulseReverse() error {
	logs.Info("MavlinkService: PulseReverse 2s (throttle=1000 steering=1500 @ 20Hz)")
	if err := s.pulseRCOverride(1000, 1500, 2*time.Second); err != nil {
		return err
	}
	return s.StopMotion()
}

// PulseLeft steers left briefly while maintaining neutral throttle.
func (s *MavlinkService) PulseLeft() error {
	logs.Info("MavlinkService: PulseLeft 1200ms (steering=1000 throttle=1800 @ 20Hz)")
	if err := s.pulseRCOverride(1800, 1000, 1200*time.Millisecond); err != nil {
		return err
	}
	return s.StopMotion()
}

// PulseRight steers right briefly while maintaining neutral throttle.
func (s *MavlinkService) PulseRight() error {
	logs.Info("MavlinkService: PulseRight 1200ms (steering=2000 throttle=1800 @ 20Hz)")
	if err := s.pulseRCOverride(1800, 2000, 1200*time.Millisecond); err != nil {
		return err
	}
	return s.StopMotion()
}

// StopMotion actively commands neutral throttle, then releases override.
func (s *MavlinkService) StopMotion() error {
	logs.Info("MavlinkService: StopMotion (neutral throttle @ 20Hz + release)")
	ticker := time.NewTicker(50 * time.Millisecond) // 20Hz
	defer ticker.Stop()
	deadline := time.Now().Add(600 * time.Millisecond)
	for {
		neutral := uint16(1500)
		if err := s.node.WriteMessageAll(s.overrideMessageSelective(&neutral, &neutral, false)); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			break
		}
		<-ticker.C
	}

	return s.node.WriteMessageAll(s.overrideMessageSelective(nil, nil, true))
}

func (s *MavlinkService) requestRCMap() {
	sys, comp := s.getTargetIDs()
	request := func(name string) {
		targets := [][2]uint8{
			{0, 0}, // broadcast
			{sys, comp},
			{1, 1}, // common autopilot ids
			{1, 0},
			{0, 1},
		}
		for _, t := range targets {
			_ = s.node.WriteMessageAll(&common.MessageParamRequestRead{
				TargetSystem:    t[0],
				TargetComponent: t[1],
				ParamId:         name,
				ParamIndex:      -1,
			})
		}
	}
	request("RCMAP_STEERING")
	request("RCMAP_THROTTLE")
	time.Sleep(30 * time.Millisecond)
	request("RCMAP_STEERING")
	request("RCMAP_THROTTLE")
	logs.Info("MavlinkService: requested RC map params; defaults steering=ch%d throttle=ch%d until learned", s.steeringChannel, s.throttleChannel)
}

func (s *MavlinkService) overrideMessage(steeringPWM, throttlePWM uint16, release bool) *common.MessageRcChannelsOverride {
	return s.overrideMessageSelective(&steeringPWM, &throttlePWM, release)
}

func (s *MavlinkService) overrideMessageSelective(steeringPWM *uint16, throttlePWM *uint16, release bool) *common.MessageRcChannelsOverride {
	sys, comp := s.getTargetIDs()
	msg := &common.MessageRcChannelsOverride{
		TargetSystem:    sys,
		TargetComponent: comp,
		Chan1Raw:        65535,
		Chan2Raw:        65535,
		Chan3Raw:        65535,
		Chan4Raw:        65535,
		Chan5Raw:        65535,
		Chan6Raw:        65535,
		Chan7Raw:        65535,
		Chan8Raw:        65535,
	}
	setChannel := func(ch uint8, pwm uint16) {
		switch ch {
		case 1:
			msg.Chan1Raw = pwm
		case 2:
			msg.Chan2Raw = pwm
		case 3:
			msg.Chan3Raw = pwm
		case 4:
			msg.Chan4Raw = pwm
		case 5:
			msg.Chan5Raw = pwm
		case 6:
			msg.Chan6Raw = pwm
		case 7:
			msg.Chan7Raw = pwm
		case 8:
			msg.Chan8Raw = pwm
		}
	}
	if release {
		setChannel(s.steeringChannel, 0)
		setChannel(s.throttleChannel, 0)
		return msg
	}
	if steeringPWM != nil {
		setChannel(s.steeringChannel, *steeringPWM)
	}
	if throttlePWM != nil {
		setChannel(s.throttleChannel, *throttlePWM)
	}
	return msg
}

func (s *MavlinkService) updateTargetIDs(systemID, componentID uint8) {
	if systemID == 0 || componentID == 0 {
		return
	}
	s.targetMu.Lock()
	changed := s.targetSystem != systemID || s.targetComponent != componentID
	s.targetSystem = systemID
	s.targetComponent = componentID
	s.targetMu.Unlock()
	if changed {
		logs.Info("MavlinkService: target IDs updated system=%d component=%d", systemID, componentID)
	}
}

func (s *MavlinkService) getTargetIDs() (uint8, uint8) {
	s.targetMu.RLock()
	defer s.targetMu.RUnlock()
	return s.targetSystem, s.targetComponent
}

func (s *MavlinkService) latestPosition() (float64, float64, float32, bool) {
	s.posMu.RLock()
	defer s.posMu.RUnlock()
	return s.latestLatDeg, s.latestLonDeg, s.latestRelAltM, s.havePosition
}

func (s *MavlinkService) readRCChannel(msg *common.MessageRcChannels, channel uint8) uint16 {
	switch channel {
	case 1:
		return msg.Chan1Raw
	case 2:
		return msg.Chan2Raw
	case 3:
		return msg.Chan3Raw
	case 4:
		return msg.Chan4Raw
	case 5:
		return msg.Chan5Raw
	case 6:
		return msg.Chan6Raw
	case 7:
		return msg.Chan7Raw
	case 8:
		return msg.Chan8Raw
	default:
		return 0
	}
}

func (s *MavlinkService) readServoChannel(msg *common.MessageServoOutputRaw, channel uint8) uint16 {
	switch channel {
	case 1:
		return msg.Servo1Raw
	case 2:
		return msg.Servo2Raw
	case 3:
		return msg.Servo3Raw
	case 4:
		return msg.Servo4Raw
	case 5:
		return msg.Servo5Raw
	case 6:
		return msg.Servo6Raw
	case 7:
		return msg.Servo7Raw
	case 8:
		return msg.Servo8Raw
	default:
		return 0
	}
}

// Arm sends the arm command to the rover
func (s *MavlinkService) Arm() error {
	logs.Info("MavlinkService: Sending ARM command")
	sys, comp := s.getTargetIDs()
	return s.node.WriteMessageAll(&common.MessageCommandLong{
		TargetSystem:    sys,
		TargetComponent: comp,
		Command:         common.MAV_CMD_COMPONENT_ARM_DISARM,
		Param1:          1, // 1 = Arm
		Param2:          0, // 0 = Emergency Disarm (not used for arming)
	})
}

// Disarm sends the disarm command to the rover
func (s *MavlinkService) Disarm() error {
	logs.Info("MavlinkService: Sending DISARM command")
	sys, comp := s.getTargetIDs()
	return s.node.WriteMessageAll(&common.MessageCommandLong{
		TargetSystem:    sys,
		TargetComponent: comp,
		Command:         common.MAV_CMD_COMPONENT_ARM_DISARM,
		Param1:          0, // 0 = Disarm
		Param2:          0,
	})
}

// SetMode sets the rover mode (e.g., MANUAL, GUIDED, STEERING)
func (s *MavlinkService) SetMode(mode string) error {
	var customMode uint32

	switch strings.ToUpper(mode) {
	case "MANUAL":
		customMode = 0 // ArduRover MANUAL
	case "GUIDED":
		customMode = 15 // ArduRover GUIDED
	case "STEERING":
		customMode = 3 // ArduRover STEERING
	default:
		return fmt.Errorf("unsupported mode: %s", mode)
	}

	logs.Info("MavlinkService: Setting mode to %s (custom_mode=%d)", mode, customMode)
	sys, comp := s.getTargetIDs()

	return s.node.WriteMessageAll(&common.MessageCommandLong{
		TargetSystem:    sys,
		TargetComponent: comp,
		Command:         common.MAV_CMD_DO_SET_MODE,
		Param1:          1, // MAV_MODE_FLAG_CUSTOM_MODE_ENABLED
		Param2:          float32(customMode),
	})
}

// GuidedForwardMeters moves rover forward by an approximate north-relative offset in GUIDED mode
// using position targets derived from current GPS fix.
func (s *MavlinkService) GuidedForwardMeters(distanceM float64) error {
	lat, lon, alt, ok := s.latestPosition()
	if !ok {
		return fmt.Errorf("no recent GLOBAL_POSITION_INT fix available")
	}
	goalLat, goalLon := offsetMeters(lat, lon, distanceM, 0)
	logs.Info("MavlinkService: GuidedForwardMeters start=(%.7f,%.7f) goal=(%.7f,%.7f) dist=%.2fm", lat, lon, goalLat, goalLon, distanceM)
	return s.sendGuidedTarget(goalLat, goalLon, alt, 4*time.Second)
}

// GuidedSquareMeters sends four position targets forming a square from the current position.
func (s *MavlinkService) GuidedSquareMeters(edgeM float64) error {
	lat0, lon0, alt, ok := s.latestPosition()
	if !ok {
		return fmt.Errorf("no recent GLOBAL_POSITION_INT fix available")
	}
	p1Lat, p1Lon := offsetMeters(lat0, lon0, edgeM, 0)    // north
	p2Lat, p2Lon := offsetMeters(p1Lat, p1Lon, 0, edgeM)  // east
	p3Lat, p3Lon := offsetMeters(p2Lat, p2Lon, -edgeM, 0) // south
	p4Lat, p4Lon := offsetMeters(p3Lat, p3Lon, 0, -edgeM) // west back near start
	pts := [][2]float64{{p1Lat, p1Lon}, {p2Lat, p2Lon}, {p3Lat, p3Lon}, {p4Lat, p4Lon}}
	for i, pt := range pts {
		logs.Info("MavlinkService: GuidedSquare edge=%d goal=(%.7f,%.7f) edge=%.2fm", i+1, pt[0], pt[1], edgeM)
		if err := s.sendGuidedTarget(pt[0], pt[1], alt, 5*time.Second); err != nil {
			return err
		}
		time.Sleep(250 * time.Millisecond)
	}
	return nil
}

func (s *MavlinkService) sendGuidedTarget(latDeg, lonDeg float64, relAltM float32, duration time.Duration) error {
	sys, comp := s.getTargetIDs()
	typeMask := common.POSITION_TARGET_TYPEMASK_VX_IGNORE |
		common.POSITION_TARGET_TYPEMASK_VY_IGNORE |
		common.POSITION_TARGET_TYPEMASK_VZ_IGNORE |
		common.POSITION_TARGET_TYPEMASK_AX_IGNORE |
		common.POSITION_TARGET_TYPEMASK_AY_IGNORE |
		common.POSITION_TARGET_TYPEMASK_AZ_IGNORE |
		common.POSITION_TARGET_TYPEMASK_YAW_IGNORE |
		common.POSITION_TARGET_TYPEMASK_YAW_RATE_IGNORE
	msg := &common.MessageSetPositionTargetGlobalInt{
		TimeBootMs:      uint32(time.Now().UnixMilli() & 0xffffffff),
		TargetSystem:    sys,
		TargetComponent: comp,
		CoordinateFrame: common.MAV_FRAME_GLOBAL_RELATIVE_ALT,
		TypeMask:        typeMask,
		LatInt:          int32(math.Round(latDeg * 1e7)),
		LonInt:          int32(math.Round(lonDeg * 1e7)),
		Alt:             relAltM,
	}
	ticker := time.NewTicker(200 * time.Millisecond) // 5Hz setpoint refresh
	defer ticker.Stop()
	deadline := time.Now().Add(duration)
	for {
		msg.TimeBootMs = uint32(time.Now().UnixMilli() & 0xffffffff)
		if err := s.node.WriteMessageAll(msg); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			return nil
		}
		<-ticker.C
	}
}

func offsetMeters(latDeg, lonDeg, northM, eastM float64) (float64, float64) {
	const earthRadius = 6378137.0
	dLat := northM / earthRadius
	cosLat := math.Cos(latDeg * math.Pi / 180.0)
	if math.Abs(cosLat) < 1e-6 {
		cosLat = 1e-6
	}
	dLon := eastM / (earthRadius * cosLat)
	return latDeg + dLat*180.0/math.Pi, lonDeg + dLon*180.0/math.Pi
}

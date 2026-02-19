package mavlink

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"dialtone/dev/core/logger"
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

// MavlinkService handles MAVLink communication
type MavlinkService struct {
	node   *gomavlib.Node
	config MavlinkConfig
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
		OutSystemID: 10, // Dialtone ID
	}
	err := node.Initialize()
	if err != nil {
		return nil, err
	}

	return &MavlinkService{
		node:   node,
		config: config,
	}, nil
}

// Start starts the MAVLink event loop
func (s *MavlinkService) Start() {
	// defer s.node.Close() // handled by Close()
	logger.LogInfo("MavlinkService: Starting event loop on %s", s.config.Endpoint)

	for evt := range s.node.Events() {
		receivedAt := time.Now().UnixMilli()
		switch e := evt.(type) {
		case *gomavlib.EventFrame:
			// LOG EVERY FRAME FOR DEBUGGING
			// logger.LogInfo("[MAVLINK-RAW] Frame from sys %d comp %d at %v", e.SystemID(), e.ComponentID(), receivedAt)

			msg := e.Message()
			msgType := fmt.Sprintf("%T", msg)
			if strings.Contains(msgType, "Message") {
				parts := strings.Split(msgType, "Message")
				if len(parts) > 1 {
					msgType = strings.ToUpper(parts[1])
				}
			}
			logger.LogInfo("[MAVLINK-RAW] %s received at %v", msgType, receivedAt)

			switch msg := e.Message().(type) {
			case *common.MessageHeartbeat:
				logger.LogInfo("[MAVLINK-RAW] HEARTBEAT received at %v", receivedAt)
				if s.config.Callback != nil {
					s.config.Callback(&MavlinkEvent{
						Type:       "HEARTBEAT",
						Data:       msg,
						ReceivedAt: receivedAt,
					})
				}
			case *common.MessageCommandAck:
				if s.config.Callback != nil {
					s.config.Callback(&MavlinkEvent{
						Type:       "COMMAND_ACK",
						Data:       msg,
						ReceivedAt: receivedAt,
					})
				}
			case *common.MessageStatustext:
				if s.config.Callback != nil {
					s.config.Callback(&MavlinkEvent{
						Type:       "STATUSTEXT",
						Data:       msg,
						ReceivedAt: receivedAt,
					})
				}
			case *common.MessageGlobalPositionInt:
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
			// logger.LogInfo("MAVLink parse error: %v", e.Error)
		case *gomavlib.EventStreamRequested:
			logger.LogInfo("MAVLink stream requested")
		case *gomavlib.EventChannelOpen:
			logger.LogInfo("MAVLink channel open")
		case *gomavlib.EventChannelClose:
			logger.LogInfo("MAVLink channel close")
		}
	}
}

// Close closes the MAVLink service
func (s *MavlinkService) Close() {
	s.node.Close()
}

// Arm sends the arm command to the rover
func (s *MavlinkService) Arm() error {
	logger.LogInfo("MavlinkService: Sending ARM command")
	return s.node.WriteMessageAll(&common.MessageCommandLong{
		TargetSystem:    0, // Broadcast
		TargetComponent: 0, // Broadcast
		Command:         common.MAV_CMD_COMPONENT_ARM_DISARM,
		Param1:          1, // 1 = Arm
		Param2:          0, // 0 = Emergency Disarm (not used for arming)
	})
}

// Disarm sends the disarm command to the rover
func (s *MavlinkService) Disarm() error {
	logger.LogInfo("MavlinkService: Sending DISARM command")
	return s.node.WriteMessageAll(&common.MessageCommandLong{
		TargetSystem:    0,
		TargetComponent: 0,
		Command:         common.MAV_CMD_COMPONENT_ARM_DISARM,
		Param1:          0, // 0 = Disarm
		Param2:          0,
	})
}

// SetMode sets the rover mode (e.g., MANUAL, GUIDED)
func (s *MavlinkService) SetMode(mode string) error {
	var customMode uint32

	switch strings.ToUpper(mode) {
	case "MANUAL":
		customMode = 0 // ArduRover MANUAL
	case "GUIDED":
		customMode = 15 // ArduRover GUIDED
	default:
		return fmt.Errorf("unsupported mode: %s", mode)
	}

	logger.LogInfo("MavlinkService: Setting mode to %s (custom_mode=%d)", mode, customMode)

	return s.node.WriteMessageAll(&common.MessageCommandLong{
		TargetSystem:    0,
		TargetComponent: 0,
		Command:         common.MAV_CMD_DO_SET_MODE,
		Param1:          1, // MAV_MODE_FLAG_CUSTOM_MODE_ENABLED
		Param2:          float32(customMode),
	})
}

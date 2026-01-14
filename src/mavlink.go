package dialtone

import (
	"fmt"
	"strconv"
	"strings"

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
	Type string
	Data interface{}
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
	LogInfo("MavlinkService: Starting event loop on %s", s.config.Endpoint)

	for evt := range s.node.Events() {
		switch e := evt.(type) {
		case *gomavlib.EventFrame:
			// LogInfo("MAVLink frame received: systemID=%d componentID=%d", e.SystemID(), e.ComponentID())
			
			switch msg := e.Message().(type) {
			case *common.MessageHeartbeat:
				// LogInfo("Heartbeat received from system %d", e.SystemID())
				if s.config.Callback != nil {
					s.config.Callback(&MavlinkEvent{
						Type: "HEARTBEAT",
						Data: msg,
					})
				}
			}
		case *gomavlib.EventParseError:
			// LogInfo("MAVLink parse error: %v", e.Error)
		case *gomavlib.EventStreamRequested:
			LogInfo("MAVLink stream requested")
		case *gomavlib.EventChannelOpen:
			LogInfo("MAVLink channel open")
		case *gomavlib.EventChannelClose:
			LogInfo("MAVLink channel close")
		}
	}
}

// Close closes the MAVLink service
func (s *MavlinkService) Close() {
	s.node.Close()
}

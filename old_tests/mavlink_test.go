package test

import (
	"testing"
	"time"
    "fmt"

	dialtone "dialtone/cli/src"
	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/common"
)

func TestMavlinkHeartbeat(t *testing.T) {
	// 1. Start the Dialtone MAVLink listener (server) on a random port
    // We'll use a channel to signal when a heartbeat is received
    heartbeatReceived := make(chan bool)
    
    // Create a mock callback for the test
    callback := func(evt *dialtone.MavlinkEvent) {
        if evt.Type == "HEARTBEAT" {
            heartbeatReceived <- true
        }
    }

    // Start the service on localhost UDP
    port := 14550
    config := dialtone.MavlinkConfig{
        Endpoint: fmt.Sprintf("udp:0.0.0.0:%d", port),
        Callback: callback,
    }

    service, err := dialtone.NewMavlinkService(config)
    if err != nil {
        t.Fatalf("Failed to create mavlink service: %v", err)
    }
    defer service.Close()

    go service.Start()

    // 2. Create a client to send a heartbeat
    // We wait a bit for the server to start
    time.Sleep(500 * time.Millisecond)

    clientNode := &gomavlib.Node{
		Endpoints: []gomavlib.EndpointConf{
			gomavlib.EndpointUDPClient{Address: fmt.Sprintf("127.0.0.1:%d", port)},
		},
		Dialect:     common.Dialect,
		OutVersion:  gomavlib.V2,
		OutSystemID: 11, // Different from dialtone
	}
    err = clientNode.Initialize()
	if err != nil {
		t.Fatalf("Failed to create client node: %v", err)
	}
	defer clientNode.Close()

    // Send a heartbeat
    msg := &common.MessageHeartbeat{
        Type:           common.MAV_TYPE_GCS,
        Autopilot:      common.MAV_AUTOPILOT_INVALID,
        BaseMode:       common.MAV_MODE_FLAG_CUSTOM_MODE_ENABLED,
        CustomMode:     0,
        SystemStatus:   common.MAV_STATE_ACTIVE,
        MavlinkVersion: 3,
    }

    fmt.Println("Test: Sending heartbeat...")
    err = clientNode.WriteMessageAll(msg)
    if err != nil {
        t.Fatalf("Failed to send heartbeat: %v", err)
    }

    // 3. Wait for the heartbeat to be received
    select {
    case <-heartbeatReceived:
        // Success
    case <-time.After(2 * time.Second):
        t.Fatal("Timeout waiting for heartbeat")
    }
}

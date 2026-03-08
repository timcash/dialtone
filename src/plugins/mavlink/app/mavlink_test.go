package mavlink

import (
	"net"
	"testing"
	"time"

	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/common"
)

func TestMavlinkServiceReceivesHeartbeatAndPosition(t *testing.T) {
	port := freeUDPPort(t)
	addr := "127.0.0.1:" + port

	events := make(chan *MavlinkEvent, 16)
	svc, err := NewMavlinkService(MavlinkConfig{
		Endpoint: "udp:" + addr,
		Callback: func(evt *MavlinkEvent) {
			select {
			case events <- evt:
			default:
			}
		},
	})
	if err != nil {
		t.Fatalf("new mavlink service: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		svc.Start()
	}()

	sender := &gomavlib.Node{
		Endpoints:   []gomavlib.EndpointConf{gomavlib.EndpointUDPClient{Address: addr}},
		Dialect:     common.Dialect,
		OutVersion:  gomavlib.V2,
		OutSystemID: 1,
	}
	if err := sender.Initialize(); err != nil {
		t.Fatalf("init sender: %v", err)
	}
	defer sender.Close()

	time.Sleep(150 * time.Millisecond)
	if err := sender.WriteMessageAll(&common.MessageHeartbeat{
		CustomMode:   7,
		BaseMode:     0x80,
		SystemStatus: 4,
	}); err != nil {
		t.Fatalf("send heartbeat: %v", err)
	}
	if err := sender.WriteMessageAll(&common.MessageGlobalPositionInt{
		Lat:         int32(377749000),
		Lon:         int32(-1224194000),
		RelativeAlt: 1200,
	}); err != nil {
		t.Fatalf("send global position: %v", err)
	}

	var sawHeartbeat bool
	var sawPosition bool
	deadline := time.After(5 * time.Second)
	for !(sawHeartbeat && sawPosition) {
		select {
		case evt := <-events:
			switch evt.Type {
			case "HEARTBEAT":
				sawHeartbeat = true
			case "GLOBAL_POSITION_INT":
				sawPosition = true
			}
		case <-deadline:
			t.Fatalf("timed out waiting for heartbeat=%t position=%t", sawHeartbeat, sawPosition)
		}
	}

	lat, lon, relAlt, ok := svc.latestPosition()
	if !ok {
		t.Fatalf("expected latest position to be available")
	}
	if lat == 0 || lon == 0 || relAlt == 0 {
		t.Fatalf("expected non-zero latest position, got lat=%f lon=%f relAlt=%f", lat, lon, relAlt)
	}

	svc.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("service did not stop")
	}
}

func freeUDPPort(t *testing.T) string {
	t.Helper()
	ln, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate udp port: %v", err)
	}
	defer ln.Close()
	addr, ok := ln.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("unexpected udp addr type %T", ln.LocalAddr())
	}
	return strconvItoa(addr.Port)
}

func strconvItoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	n := v
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

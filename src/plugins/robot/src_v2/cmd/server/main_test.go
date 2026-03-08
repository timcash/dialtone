package main

import (
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestStartStatsPublisherClearsErrorAfterHeartbeat(t *testing.T) {
	natsPort := freeTCPPort(t)
	wsPort := freeTCPPort(t)

	ns, err := startEmbeddedNATS(natsPort, wsPort)
	if err != nil {
		t.Fatalf("start embedded nats: %v", err)
	}
	defer ns.Shutdown()

	telemetry := &telemetryMonitor{}
	startStatsPublisher(natsPort, ns, true, telemetry)

	nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", natsPort), nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	defer nc.Close()

	sub, err := nc.SubscribeSync("mavlink.stats")
	if err != nil {
		t.Fatalf("subscribe mavlink.stats: %v", err)
	}
	defer sub.Unsubscribe()

	if err := nc.Publish("mavlink.heartbeat", []byte(`{"type":"HEARTBEAT","timestamp":12345}`)); err != nil {
		t.Fatalf("publish heartbeat: %v", err)
	}
	if err := nc.Flush(); err != nil {
		t.Fatalf("flush heartbeat: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := sub.NextMsg(1500 * time.Millisecond)
		if err != nil {
			continue
		}
		var payload struct {
			Errors []string `json:"errors"`
		}
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			t.Fatalf("decode mavlink.stats: %v", err)
		}
		if len(payload.Errors) == 0 {
			return
		}
	}

	t.Fatalf("did not observe mavlink.stats without errors after heartbeat")
}

func TestTelemetryMonitorMavlinkStatus(t *testing.T) {
	monitor := &telemetryMonitor{}

	if got, errText := monitor.mavlinkStatus(false); got != "not-configured" || errText != "" {
		t.Fatalf("disabled status = (%q, %q), want (not-configured, empty)", got, errText)
	}

	if got, errText := monitor.mavlinkStatus(true); got != "configured" || errText == "" {
		t.Fatalf("no-telemetry status = (%q, %q), want (configured, non-empty)", got, errText)
	}

	monitor.lastMavlinkTelemetryAt.Store(time.Now().Add(-5 * time.Second).UnixMilli())
	if got, errText := monitor.mavlinkStatus(true); got != "degraded" || errText == "" {
		t.Fatalf("stale status = (%q, %q), want (degraded, non-empty)", got, errText)
	}

	monitor.markTelemetryNow()
	if got, errText := monitor.mavlinkStatus(true); got != "ok" || errText != "" {
		t.Fatalf("live status = (%q, %q), want (ok, empty)", got, errText)
	}
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate tcp port: %v", err)
	}
	defer ln.Close()
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("unexpected tcp addr type %T", ln.Addr())
	}
	return addr.Port
}

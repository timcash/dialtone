package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"tailscale.com/tsnet"
)

// startTestNATSServer creates and starts an embedded NATS server for testing
func startTestNATSServer(t *testing.T, host string, port int) *server.Server {
	opts := &server.Options{
		Host: host,
		Port: port,
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("Failed to create NATS server: %v", err)
	}

	go ns.Start()

	if !ns.ReadyForConnections(10 * time.Second) {
		t.Fatal("NATS server failed to start within timeout")
	}

	return ns
}

// =============================================================================
// NATS Server Tests (Local Mode)
// =============================================================================

func TestServerStarts(t *testing.T) {
	ns := startTestNATSServer(t, "127.0.0.1", 14222)
	defer ns.Shutdown()

	if !ns.Running() {
		t.Error("Server should be running")
	}
}

func TestClientCanConnect(t *testing.T) {
	ns := startTestNATSServer(t, "127.0.0.1", 14223)
	defer ns.Shutdown()

	nc, err := nats.Connect("nats://localhost:14223")
	if err != nil {
		t.Fatalf("Failed to connect to NATS server: %v", err)
	}
	defer nc.Close()

	if !nc.IsConnected() {
		t.Error("Client should be connected")
	}
}

func TestPubSub(t *testing.T) {
	ns := startTestNATSServer(t, "127.0.0.1", 14224)
	defer ns.Shutdown()

	nc, err := nats.Connect("nats://localhost:14224")
	if err != nil {
		t.Fatalf("Failed to connect to NATS server: %v", err)
	}
	defer nc.Close()

	received := make(chan string, 1)
	_, err = nc.Subscribe("test.subject", func(msg *nats.Msg) {
		received <- string(msg.Data)
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	nc.Flush()

	testMessage := "Hello, NATS!"
	err = nc.Publish("test.subject", []byte(testMessage))
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	nc.Flush()

	select {
	case msg := <-received:
		if msg != testMessage {
			t.Errorf("Expected message %q, got %q", testMessage, msg)
		}
	case <-time.After(5 * time.Second):
		t.Error("Timeout waiting for message")
	}
}

func TestRequestReply(t *testing.T) {
	ns := startTestNATSServer(t, "127.0.0.1", 14225)
	defer ns.Shutdown()

	nc, err := nats.Connect("nats://localhost:14225")
	if err != nil {
		t.Fatalf("Failed to connect to NATS server: %v", err)
	}
	defer nc.Close()

	_, err = nc.Subscribe("service.echo", func(msg *nats.Msg) {
		nc.Publish(msg.Reply, []byte("echo: "+string(msg.Data)))
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	nc.Flush()

	reply, err := nc.Request("service.echo", []byte("test"), 5*time.Second)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	expected := "echo: test"
	if string(reply.Data) != expected {
		t.Errorf("Expected reply %q, got %q", expected, string(reply.Data))
	}
}

func TestQueueGroups(t *testing.T) {
	ns := startTestNATSServer(t, "127.0.0.1", 14226)
	defer ns.Shutdown()

	nc, err := nats.Connect("nats://localhost:14226")
	if err != nil {
		t.Fatalf("Failed to connect to NATS server: %v", err)
	}
	defer nc.Close()

	received := make(chan int, 10)

	for i := 0; i < 3; i++ {
		subscriberID := i
		_, err = nc.QueueSubscribe("work.queue", "workers", func(msg *nats.Msg) {
			received <- subscriberID
		})
		if err != nil {
			t.Fatalf("Failed to create queue subscriber %d: %v", i, err)
		}
	}

	nc.Flush()

	messageCount := 10
	for i := 0; i < messageCount; i++ {
		err = nc.Publish("work.queue", []byte("work item"))
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}
	}

	nc.Flush()

	receivedCount := 0
	timeout := time.After(5 * time.Second)
	for receivedCount < messageCount {
		select {
		case <-received:
			receivedCount++
		case <-timeout:
			t.Fatalf("Timeout: only received %d of %d messages", receivedCount, messageCount)
		}
	}
}

func TestMultipleConnections(t *testing.T) {
	ns := startTestNATSServer(t, "127.0.0.1", 14227)
	defer ns.Shutdown()

	connections := make([]*nats.Conn, 5)
	for i := 0; i < 5; i++ {
		nc, err := nats.Connect("nats://localhost:14227")
		if err != nil {
			t.Fatalf("Failed to create connection %d: %v", i, err)
		}
		connections[i] = nc
	}

	for i, nc := range connections {
		if !nc.IsConnected() {
			t.Errorf("Connection %d should be connected", i)
		}
	}

	for _, nc := range connections {
		nc.Close()
	}
}

// =============================================================================
// Proxy Tests (Core Tailscale Integration Logic)
// =============================================================================

func TestProxyConnection(t *testing.T) {
	// Start a backend NATS server
	ns := startTestNATSServer(t, "127.0.0.1", 14228)
	defer ns.Shutdown()

	// Create a proxy listener
	proxyLn, err := net.Listen("tcp", "127.0.0.1:14229")
	if err != nil {
		t.Fatalf("Failed to create proxy listener: %v", err)
	}
	defer proxyLn.Close()

	// Start proxying
	go proxyListener(proxyLn, "127.0.0.1:14228")

	// Connect through the proxy
	nc, err := nats.Connect("nats://localhost:14229")
	if err != nil {
		t.Fatalf("Failed to connect through proxy: %v", err)
	}
	defer nc.Close()

	if !nc.IsConnected() {
		t.Error("Client should be connected through proxy")
	}

	// Test pub/sub through the proxy
	received := make(chan string, 1)
	_, err = nc.Subscribe("proxy.test", func(msg *nats.Msg) {
		received <- string(msg.Data)
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	nc.Flush()

	err = nc.Publish("proxy.test", []byte("proxied message"))
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}
	nc.Flush()

	select {
	case msg := <-received:
		if msg != "proxied message" {
			t.Errorf("Expected 'proxied message', got %q", msg)
		}
	case <-time.After(5 * time.Second):
		t.Error("Timeout waiting for proxied message")
	}
}

func TestProxyMultipleConnections(t *testing.T) {
	// Start a backend NATS server
	ns := startTestNATSServer(t, "127.0.0.1", 14230)
	defer ns.Shutdown()

	// Create a proxy listener
	proxyLn, err := net.Listen("tcp", "127.0.0.1:14231")
	if err != nil {
		t.Fatalf("Failed to create proxy listener: %v", err)
	}
	defer proxyLn.Close()

	// Start proxying
	go proxyListener(proxyLn, "127.0.0.1:14230")

	// Create multiple connections through the proxy
	connections := make([]*nats.Conn, 5)
	for i := 0; i < 5; i++ {
		nc, err := nats.Connect("nats://localhost:14231")
		if err != nil {
			t.Fatalf("Failed to create connection %d: %v", i, err)
		}
		connections[i] = nc
	}
	defer func() {
		for _, nc := range connections {
			nc.Close()
		}
	}()

	// Verify all connections work
	for i, nc := range connections {
		if !nc.IsConnected() {
			t.Errorf("Connection %d should be connected", i)
		}

		// Test each connection can pub/sub
		subject := fmt.Sprintf("test.conn.%d", i)
		received := make(chan bool, 1)

		_, err := nc.Subscribe(subject, func(msg *nats.Msg) {
			received <- true
		})
		if err != nil {
			t.Fatalf("Connection %d failed to subscribe: %v", i, err)
		}
		nc.Flush()

		err = nc.Publish(subject, []byte("test"))
		if err != nil {
			t.Fatalf("Connection %d failed to publish: %v", i, err)
		}
		nc.Flush()

		select {
		case <-received:
			// Success
		case <-time.After(2 * time.Second):
			t.Errorf("Connection %d: timeout waiting for message", i)
		}
	}
}

func TestProxyBidirectionalData(t *testing.T) {
	// Create a simple echo server to test bidirectional proxy
	echoLn, err := net.Listen("tcp", "127.0.0.1:14232")
	if err != nil {
		t.Fatalf("Failed to create echo listener: %v", err)
	}
	defer echoLn.Close()

	// Echo server goroutine
	go func() {
		for {
			conn, err := echoLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c) // Echo back
			}(conn)
		}
	}()

	// Create proxy listener
	proxyLn, err := net.Listen("tcp", "127.0.0.1:14233")
	if err != nil {
		t.Fatalf("Failed to create proxy listener: %v", err)
	}
	defer proxyLn.Close()

	// Start proxying to echo server
	go proxyListener(proxyLn, "127.0.0.1:14232")

	// Connect through proxy and test echo
	conn, err := net.Dial("tcp", "127.0.0.1:14233")
	if err != nil {
		t.Fatalf("Failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	testData := "Hello, Proxy!"
	_, err = conn.Write([]byte(testData))
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}

	buf := make([]byte, len(testData))
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err = io.ReadFull(conn, buf)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	if string(buf) != testData {
		t.Errorf("Expected %q, got %q", testData, string(buf))
	}
}

// =============================================================================
// Tailscale Integration Tests (require TS_AUTHKEY)
// =============================================================================

func TestTailscaleConnection(t *testing.T) {
	// Skip if no auth key is available
	if os.Getenv("TS_AUTHKEY") == "" {
		t.Skip("Skipping Tailscale test: TS_AUTHKEY not set")
	}

	// Create temp directory for state
	stateDir, err := os.MkdirTemp("", "tsnet-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(stateDir)

	// Create tsnet server
	ts := &tsnet.Server{
		Hostname:  "nats-test",
		Dir:       stateDir,
		Ephemeral: true,
	}

	// Start tsnet
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	status, err := ts.Up(ctx)
	if err != nil {
		t.Fatalf("Failed to connect to Tailscale: %v", err)
	}
	defer ts.Close()

	// Verify we got IPs
	if len(status.TailscaleIPs) == 0 {
		t.Error("Expected at least one Tailscale IP")
	}

	t.Logf("Connected to Tailscale with IPs: %v", status.TailscaleIPs)

	// Start NATS on localhost
	ns := startTestNATSServer(t, "127.0.0.1", 14240)
	defer ns.Shutdown()

	// Create Tailscale listener
	ln, err := ts.Listen("tcp", ":4222")
	if err != nil {
		t.Fatalf("Failed to listen on Tailscale: %v", err)
	}
	defer ln.Close()

	// Start proxy
	go proxyListener(ln, "127.0.0.1:14240")

	t.Log("NATS server available on Tailscale at nats-test:4222")

	// We can't easily connect back to ourselves via Tailscale in a test,
	// but we've verified the setup works. In a real scenario, another
	// machine on the tailnet would connect.
	t.Log("Tailscale integration test passed - server is listening")
}

func TestTailscaleNATSPubSub(t *testing.T) {
	// Skip if no auth key is available
	if os.Getenv("TS_AUTHKEY") == "" {
		t.Skip("Skipping Tailscale test: TS_AUTHKEY not set")
	}

	// Create temp directory for state
	stateDir, err := os.MkdirTemp("", "tsnet-nats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(stateDir)

	// Create tsnet server
	ts := &tsnet.Server{
		Hostname:  "nats-pubsub-test",
		Dir:       stateDir,
		Ephemeral: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	status, err := ts.Up(ctx)
	if err != nil {
		t.Fatalf("Failed to connect to Tailscale: %v", err)
	}
	defer ts.Close()

	t.Logf("Connected with IPs: %v", status.TailscaleIPs)

	// Start NATS
	ns := startTestNATSServer(t, "127.0.0.1", 14241)
	defer ns.Shutdown()

	// Listen on Tailscale
	ln, err := ts.Listen("tcp", ":4222")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer ln.Close()

	go proxyListener(ln, "127.0.0.1:14241")

	// Use tsnet's Dial to connect back to ourselves via a custom dialer
	nc, err := nats.Connect("nats://nats-pubsub-test:4222",
		nats.SetCustomDialer(&tsnetDialer{ts: ts, ctx: ctx}),
	)
	if err != nil {
		t.Fatalf("Failed to create NATS connection: %v", err)
	}
	defer nc.Close()

	// Test pub/sub
	received := make(chan string, 1)
	_, err = nc.Subscribe("tailscale.test", func(msg *nats.Msg) {
		received <- string(msg.Data)
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	nc.Flush()

	err = nc.Publish("tailscale.test", []byte("Hello via Tailscale!"))
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}
	nc.Flush()

	select {
	case msg := <-received:
		if msg != "Hello via Tailscale!" {
			t.Errorf("Expected 'Hello via Tailscale!', got %q", msg)
		}
		t.Log("Successfully sent/received message via Tailscale!")
	case <-time.After(10 * time.Second):
		t.Error("Timeout waiting for message via Tailscale")
	}
}

// tsnetDialer implements nats.CustomDialer using tsnet
type tsnetDialer struct {
	ts  *tsnet.Server
	ctx context.Context
}

func (d *tsnetDialer) Dial(network, address string) (net.Conn, error) {
	return d.ts.Dial(d.ctx, network, address)
}

// =============================================================================
// StartNATSServer Function Test
// =============================================================================

func TestStartNATSServerFunction(t *testing.T) {
	ns := startNATSServer("127.0.0.1", 14250, false)
	defer ns.Shutdown()

	if !ns.Running() {
		t.Error("Server should be running")
	}

	// Test we can connect
	nc, err := nats.Connect("nats://localhost:14250")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer nc.Close()

	if !nc.IsConnected() {
		t.Error("Should be connected")
	}
}

// =============================================================================
// State Directory Test
// =============================================================================

func TestStateDirCreation(t *testing.T) {
	// Test that we can create state directories
	tempDir := filepath.Join(os.TempDir(), "tsnet-state-test")
	defer os.RemoveAll(tempDir)

	err := os.MkdirAll(tempDir, 0700)
	if err != nil {
		t.Fatalf("Failed to create state dir: %v", err)
	}

	info, err := os.Stat(tempDir)
	if err != nil {
		t.Fatalf("Failed to stat dir: %v", err)
	}

	if !info.IsDir() {
		t.Error("Should be a directory")
	}
}

// =============================================================================
// Web Server Tests
// =============================================================================

func TestTailscaleWebAccess(t *testing.T) {
	// Skip if no auth key is available
	if os.Getenv("TS_AUTHKEY") == "" {
		t.Skip("Skipping Tailscale test: TS_AUTHKEY not set")
	}

	// Create temp directory for state
	stateDir, err := os.MkdirTemp("", "tsnet-web-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(stateDir)

	hostname := "web-test"
	ts := &tsnet.Server{
		Hostname:  hostname,
		Dir:       stateDir,
		Ephemeral: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	status, err := ts.Up(ctx)
	if err != nil {
		t.Fatalf("Failed to connect to Tailscale: %v", err)
	}
	defer ts.Close()

	// Start a backend NATS server (needed for the web dashboard)
	ns := startTestNATSServer(t, "127.0.0.1", 14260)
	defer ns.Shutdown()

	// Create Tailscale listener for web
	webPort := 8080
	webLn, err := ts.Listen("tcp", fmt.Sprintf(":%d", webPort))
	if err != nil {
		t.Fatalf("Failed to listen for web: %v", err)
	}
	defer webLn.Close()

	// Get FQDN
	fqdn := hostname
	if status.Self != nil && status.Self.DNSName != "" {
		importStrings := "strings" // manual check if strings is imported
		_ = importStrings
		fqdn = status.Self.DNSName
		// strip trailing dot
		if len(fqdn) > 0 && fqdn[len(fqdn)-1] == '.' {
			fqdn = fqdn[:len(fqdn)-1]
		}
	}

	// Create web handler
	// hostname string, natsPort, webPort int, ns *server.Server, lc *tailscale.LocalClient, ips []netip.Addr
	webHandler := createWebHandler(hostname, 4222, webPort, ns, nil, status.TailscaleIPs)

	// Start web server
	go http.Serve(webLn, webHandler)

	// Create a transport that uses the tsnet dialer
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: ts.Dial,
		},
		Timeout: 10 * time.Second,
	}

	// Try to fetch the homepage using the FQDN
	url := fmt.Sprintf("http://%s:%d/", fqdn, webPort)
	t.Logf("Testing accessibility via URL: %s", url)

	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("Failed to fetch dashboard via %s: %v", fqdn, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Dialtone Dashboard") {
		t.Error("Dashboard content missing from response")
	}

	t.Log("Successfully accessed web dashboard via MagicDNS FQDN!")
}

package test

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

	"github.com/chromedp/chromedp"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	dialtone "rover/nats/src"
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
// NATS Server Tests
// =============================================================================

func TestNATS_Basic(t *testing.T) {
	ns := startTestNATSServer(t, "127.0.0.1", 14222)
	defer ns.Shutdown()

	nc, err := nats.Connect("nats://localhost:14222")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer nc.Close()

	if !nc.IsConnected() {
		t.Error("Client should be connected")
	}

	// Test Pub/Sub
	received := make(chan string, 1)
	_, err = nc.Subscribe("test.subject", func(msg *nats.Msg) {
		received <- string(msg.Data)
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	testMessage := "Hello, NATS!"
	nc.Publish("test.subject", []byte(testMessage))
	nc.Flush()

	select {
	case msg := <-received:
		if msg != testMessage {
			t.Errorf("Expected %q, got %q", testMessage, msg)
		}
	case <-time.After(5 * time.Second):
		t.Error("Timeout waiting for message")
	}
}

func TestNATS_RequestReply(t *testing.T) {
	ns := startTestNATSServer(t, "127.0.0.1", 14225)
	defer ns.Shutdown()

	nc, err := nats.Connect("nats://localhost:14225")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
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

// =============================================================================
// Proxy Logic Tests
// =============================================================================

func TestProxy_Bidirectional(t *testing.T) {
	// Create a simple echo server
	echoLn, err := net.Listen("tcp", "127.0.0.1:14232")
	if err != nil {
		t.Fatalf("Failed to create echo listener: %v", err)
	}
	defer echoLn.Close()

	go func() {
		for {
			conn, err := echoLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c)
			}(conn)
		}
	}()

	// Create proxy listener
	proxyLn, err := net.Listen("tcp", "127.0.0.1:14233")
	if err != nil {
		t.Fatalf("Failed to create proxy listener: %v", err)
	}
	defer proxyLn.Close()

	go dialtone.ProxyListener(proxyLn, "127.0.0.1:14232")

	// Test connection through proxy
	conn, err := net.Dial("tcp", "127.0.0.1:14233")
	if err != nil {
		t.Fatalf("Failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	testData := "Hello, Proxy!"
	conn.Write([]byte(testData))

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
// Local Web Dashboard Tests (Browser)
// =============================================================================

func TestLocalWebUI(t *testing.T) {
	// 1. Setup local server
	natsPort := 15222
	wsPort := 15223
	webPort := 18080

	// Use a temp dir for state
	stateDir, _ := os.MkdirTemp("", "dialtone-local-test-*")
	defer os.RemoveAll(stateDir)

	// Start NATS with WebSocket support
	opts := &server.Options{
		Host: "127.0.0.1",
		Port: natsPort,
		Websocket: server.WebsocketOpts{
			Port:  wsPort,
			NoTLS: true,
		},
	}
	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("Failed to create NATS: %v", err)
	}
	go ns.Start()
	defer ns.Shutdown()

	if !ns.ReadyForConnections(10 * time.Second) {
		t.Fatal("NATS not ready")
	}

	// Create web handler
	handler := dialtone.CreateWebHandler("localhost", natsPort, wsPort, webPort, ns, nil, nil)

	server := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", webPort),
		Handler: handler,
	}
	go server.ListenAndServe()
	defer server.Close()

	// Wait for web server
	time.Sleep(2 * time.Second)

	// 2. Automated Browser Test
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var statusText string
	err = chromedp.Run(ctx,
		chromedp.Navigate(fmt.Sprintf("http://127.0.0.1:%d", webPort)),
		chromedp.WaitVisible(`#status-indicator.status-online`, chromedp.ByQuery),
		chromedp.TextContent(`#status-text`, &statusText, chromedp.ByID),
	)

	if err != nil {
		t.Fatalf("Local UI browser test failed: %v", err)
	}

	if !strings.Contains(statusText, "CONNECTED") {
		t.Errorf("Expected status 'CONNECTED', got %q", statusText)
	}
}

// =============================================================================
// Helper Tests
// =============================================================================

func TestStateDir_Creation(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "tsnet-state-local-test")
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

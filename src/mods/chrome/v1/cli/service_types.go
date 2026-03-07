package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"sync"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
	"github.com/chromedp/chromedp"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

type serverOptions struct {
	host            string
	port            int
	natsURL         string
	natsPrefix      string
	embeddedNATS    bool
	chromeDebugPort int
	headless        bool
	initialURL      string
}

type tabInfo struct {
	Name     string `json:"name"`
	PID      int    `json:"pid"`
	Headless bool   `json:"headless"`
	TargetID string `json:"target_id"`
}

type commandResponse struct {
	OK    bool      `json:"ok"`
	Error string    `json:"error,omitempty"`
	Tab   string    `json:"tab,omitempty"`
	Tabs  []tabInfo `json:"tabs"`
}

type managedTab struct {
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	targetID string
}

type chromeServiceManager struct {
	session       *chrome.Session
	allocCtx      context.Context
	cancelAlloc   context.CancelFunc
	browserCtx    context.Context
	cancelBrowser context.CancelFunc
	browser       *chromedp.Browser
	headless      bool

	mu   sync.RWMutex
	tabs map[string]*managedTab
}

type natsBridge struct {
	nc   *nats.Conn
	subs []*nats.Subscription
	mgr  *chromeServiceManager
}

type embeddedNATSServer struct {
	server *natsserver.Server
}

func parseServerOptions(argv []string) (serverOptions, error) {
	fs := flag.NewFlagSet("chrome v1 server", flag.ContinueOnError)
	host := fs.String("host", "127.0.0.1", "Bind address")
	port := fs.Int("port", 7788, "Embedded web server port")
	natsURL := fs.String("nats-url", "nats://127.0.0.1:4222", "NATS server URL")
	natsPrefix := fs.String("nats-prefix", "chrome.v1", "NATS subject prefix")
	embeddedNATS := fs.Bool("embedded-nats", true, "Run an embedded NATS server in-process")
	chromeDebugPort := fs.Int("chrome-debug-port", 0, "Chrome remote debugging port (0 = auto)")
	headless := fs.Bool("headless", false, "Run Chrome in headless mode")
	initialURL := fs.String("initial-url", "about:blank", "Initial URL for the main tab")
	if err := fs.Parse(argv); err != nil {
		return serverOptions{}, err
	}
	if *port <= 0 || *port > 65535 {
		return serverOptions{}, fmt.Errorf("invalid port: %d", *port)
	}
	if *chromeDebugPort < 0 || *chromeDebugPort > 65535 {
		return serverOptions{}, fmt.Errorf("invalid chrome debug port: %d", *chromeDebugPort)
	}
	return serverOptions{
		host:            strings.TrimSpace(*host),
		port:            *port,
		natsURL:         strings.TrimSpace(*natsURL),
		natsPrefix:      strings.TrimSpace(*natsPrefix),
		embeddedNATS:    *embeddedNATS,
		chromeDebugPort: *chromeDebugPort,
		headless:        *headless,
		initialURL:      strings.TrimSpace(*initialURL),
	}, nil
}

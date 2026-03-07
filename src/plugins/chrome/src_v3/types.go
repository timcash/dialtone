package src_v3

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	defaultRole       = "dev"
	defaultChromePort = 19464
	defaultNATSPort   = 19465
	defaultTimeout    = 12 * time.Second
)

var errBrowserClosed = fmt.Errorf("browser is closed or unhealthy")

type commandRequest struct {
	Command string `json:"command"`
	Role    string `json:"role,omitempty"`
	URL     string `json:"url,omitempty"`
	Index   int    `json:"index,omitempty"`
}

type pageInfo struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type commandResponse struct {
	OK            bool       `json:"ok"`
	Error         string     `json:"error,omitempty"`
	ServicePID    int        `json:"service_pid"`
	BrowserPID    int        `json:"browser_pid"`
	ChromePort    int        `json:"chrome_port"`
	NATSPort      int        `json:"nats_port"`
	Role          string     `json:"role"`
	ProfileDir    string     `json:"profile_dir,omitempty"`
	ManagedTarget string     `json:"managed_target,omitempty"`
	CurrentURL    string     `json:"current_url,omitempty"`
	Tabs          []pageInfo `json:"tabs,omitempty"`
	Unhealthy     bool       `json:"unhealthy,omitempty"`
}

type daemonState struct {
	mu              sync.Mutex
	role            string
	chromePort      int
	natsPort        int
	profileDir      string
	chromePath      string
	browserPID      int
	browserWS       string
	allocCtx        context.Context
	cancelAlloc     context.CancelFunc
	tabCtx          context.Context
	cancelTab       context.CancelFunc
	managedTarget   string
	currentURL      string
	unexpectedErr   error
	intentionalStop bool
}

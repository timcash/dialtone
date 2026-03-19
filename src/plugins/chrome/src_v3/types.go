package src_v3

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	defaultRole        = "dev"
	defaultChromePort  = 19464
	defaultNATSPort    = 19465
	defaultTimeout     = 12 * time.Second
	DefaultServicePort = defaultNATSPort
)

var errBrowserClosed = fmt.Errorf("browser is closed or unhealthy")

type CommandRequest struct {
	Command   string `json:"command"`
	Role      string `json:"role,omitempty"`
	URL       string `json:"url,omitempty"`
	Index     int    `json:"index,omitempty"`
	AriaLabel string `json:"aria_label,omitempty"`
	Attr      string `json:"attr,omitempty"`
	Expected  string `json:"expected,omitempty"`
	Value     string `json:"value,omitempty"`
	Contains  string `json:"contains,omitempty"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
	Script    string `json:"script,omitempty"`
}

type PageInfo struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type CommandResponse struct {
	OK            bool       `json:"ok"`
	Error         string     `json:"error,omitempty"`
	Host          string     `json:"host,omitempty"`
	ServicePID    int        `json:"service_pid"`
	BrowserPID    int        `json:"browser_pid"`
	ChromePort    int        `json:"chrome_port"`
	NATSPort      int        `json:"nats_port"`
	Role          string     `json:"role"`
	ProfileDir    string     `json:"profile_dir,omitempty"`
	WebSocketURL  string     `json:"websocket_url,omitempty"`
	ManagedTarget string     `json:"managed_target,omitempty"`
	CurrentURL    string     `json:"current_url,omitempty"`
	StartedAt     string     `json:"started_at,omitempty"`
	LastHealthyAt string     `json:"last_healthy_at,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	Tabs          []PageInfo `json:"tabs,omitempty"`
	Unhealthy     bool       `json:"unhealthy,omitempty"`
	ConsoleLines  []string   `json:"console_lines,omitempty"`
	Value         string     `json:"value,omitempty"`
	ScreenshotB64 string     `json:"screenshot_b64,omitempty"`
}

type Session struct {
	Host          string `json:"host,omitempty"`
	Role          string `json:"role,omitempty"`
	PID           int    `json:"pid"`
	Port          int    `json:"port"`
	NATSPort      int    `json:"nats_port"`
	WebSocketURL  string `json:"websocket_url,omitempty"`
	CurrentURL    string `json:"current_url,omitempty"`
	ManagedTarget string `json:"managed_target,omitempty"`
	IsNew         bool   `json:"is_new,omitempty"`
}

type commandRequest = CommandRequest
type pageInfo = PageInfo
type commandResponse = CommandResponse

type daemonState struct {
	mu              sync.Mutex
	role            string
	hostID          string
	chromePort      int
	natsPort        int
	natsURL         string
	embeddedNATS    bool
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
	consoleLines    []string
	unexpectedErr   error
	intentionalStop bool
	startedAt       string
}

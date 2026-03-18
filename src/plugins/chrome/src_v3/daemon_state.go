package src_v3

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type persistedDaemonState struct {
	ServicePID    int    `json:"service_pid"`
	BrowserPID    int    `json:"browser_pid"`
	ChromePort    int    `json:"chrome_port"`
	NATSPort      int    `json:"nats_port"`
	Role          string `json:"role"`
	ProfileDir    string `json:"profile_dir"`
	WebSocketURL  string `json:"websocket_url,omitempty"`
	CurrentURL    string `json:"current_url,omitempty"`
	ManagedTarget string `json:"managed_target,omitempty"`
	StartedAt     string `json:"started_at,omitempty"`
	LastHealthyAt string `json:"last_healthy_at,omitempty"`
	LastError     string `json:"last_error,omitempty"`
}

func daemonStateRoot(role string) string {
	home, _ := os.UserHomeDir()
	role = strings.TrimSpace(role)
	if role == "" {
		role = defaultRole
	}
	return filepath.Join(home, ".dialtone", "chrome-src-v3", role)
}

func daemonStatePath(role string) string {
	return filepath.Join(daemonStateRoot(role), "state.json")
}

func (d *daemonState) persistedState() persistedDaemonState {
	d.mu.Lock()
	defer d.mu.Unlock()
	st := persistedDaemonState{
		ServicePID:    os.Getpid(),
		BrowserPID:    d.browserPID,
		ChromePort:    d.chromePort,
		NATSPort:      d.natsPort,
		Role:          d.role,
		ProfileDir:    d.profileDir,
		WebSocketURL:  d.browserWS,
		CurrentURL:    d.currentURL,
		ManagedTarget: d.managedTarget,
		StartedAt:     d.startedAt,
		LastHealthyAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if d.unexpectedErr != nil {
		st.LastError = d.unexpectedErr.Error()
	}
	return st
}

func (d *daemonState) persistState() {
	st := d.persistedState()
	root := daemonStateRoot(st.Role)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return
	}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(daemonStatePath(st.Role), raw, 0o644)
}


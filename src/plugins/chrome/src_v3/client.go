package src_v3

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

type Resource struct {
	PID        int    `json:"pid"`
	Role       string `json:"role,omitempty"`
	Origin     string `json:"origin,omitempty"`
	IsWindows  bool   `json:"is_windows,omitempty"`
	IsHeadless bool   `json:"is_headless,omitempty"`
}

func SendCommand(node sshv1.MeshNode, req CommandRequest) (*CommandResponse, error) {
	return sendRemoteCommand(node, req)
}

func SendCommandByHost(host string, req CommandRequest) (*CommandResponse, error) {
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(host))
	if err != nil {
		return nil, err
	}
	return SendCommand(node, req)
}

func NewSessionFromResponse(host string, resp *CommandResponse) *Session {
	if resp == nil {
		return nil
	}
	role := strings.TrimSpace(resp.Role)
	if role == "" {
		role = defaultRole
	}
	return &Session{
		Host:          strings.TrimSpace(host),
		Role:          role,
		PID:           resp.BrowserPID,
		Port:          resp.ChromePort,
		NATSPort:      resp.NATSPort,
		CurrentURL:    strings.TrimSpace(resp.CurrentURL),
		ManagedTarget: strings.TrimSpace(resp.ManagedTarget),
	}
}

func CleanupSession(session *Session) error {
	if session == nil || strings.TrimSpace(session.Host) == "" {
		return nil
	}
	_, err := SendCommandByHost(session.Host, CommandRequest{
		Command: "close",
		Role:    strings.TrimSpace(session.Role),
	})
	return err
}

func WriteSessionMetadata(path string, session *Session) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}
	payload := struct {
		Host          string `json:"host,omitempty"`
		Role          string `json:"role,omitempty"`
		PID           int    `json:"pid"`
		DebugPort     int    `json:"debug_port"`
		NATSPort      int    `json:"nats_port"`
		WebSocketURL  string `json:"websocket_url,omitempty"`
		CurrentURL    string `json:"current_url,omitempty"`
		ManagedTarget string `json:"managed_target,omitempty"`
	}{
		Host:          strings.TrimSpace(session.Host),
		Role:          strings.TrimSpace(session.Role),
		PID:           session.PID,
		DebugPort:     session.Port,
		NATSPort:      session.NATSPort,
		WebSocketURL:  strings.TrimSpace(session.WebSocketURL),
		CurrentURL:    strings.TrimSpace(session.CurrentURL),
		ManagedTarget: strings.TrimSpace(session.ManagedTarget),
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0644)
}

func NATSExample(host, role string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		host = "<host>"
	}
	role = strings.TrimSpace(role)
	if role == "" {
		role = defaultRole
	}
	subject := natsSubject(role)
	return fmt.Sprintf(
		"nats req %q '{\"command\":\"goto\",\"role\":\"%s\",\"url\":\"https://example.com\"}' --server nats://%s:%d",
		subject,
		role,
		host,
		defaultNATSPort,
	)
}

func KillDialtoneResources() error {
	return nil
}

func ListResources(includeChrome bool) ([]Resource, error) {
	return nil, nil
}

func KillResource(pid int, isWindows bool) error {
	return nil
}

package chrome

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type SessionMetadata struct {
	PID                int    `json:"pid"`
	DebugPort          int    `json:"debug_port"`
	WebSocketURL       string `json:"websocket_url"`
	DebugURL           string `json:"debug_url"`
	IsNew              bool   `json:"is_new"`
	GeneratedAtRFC3339 string `json:"generated_at_rfc3339"`
}

func BuildSessionMetadata(s *Session) *SessionMetadata {
	if s == nil {
		return nil
	}
	return &SessionMetadata{
		PID:                s.PID,
		DebugPort:          s.Port,
		WebSocketURL:       s.WebSocketURL,
		DebugURL:           DebugURLFromWebSocket(s.WebSocketURL),
		IsNew:              s.IsNew,
		GeneratedAtRFC3339: time.Now().Format(time.RFC3339),
	}
}

func WriteSessionMetadata(path string, s *Session) error {
	meta := BuildSessionMetadata(s)
	if meta == nil {
		return fmt.Errorf("missing chrome session")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func DebugURLFromWebSocket(wsURL string) string {
	wsURL = strings.TrimSpace(wsURL)
	if wsURL == "" {
		return ""
	}
	if strings.HasPrefix(wsURL, "ws://") {
		return "http://" + strings.TrimPrefix(wsURL, "ws://")
	}
	if strings.HasPrefix(wsURL, "wss://") {
		return "https://" + strings.TrimPrefix(wsURL, "wss://")
	}
	return wsURL
}

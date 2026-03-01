package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

const defaultChromeServicePort = 19444

type debugURLRequest struct {
	Role         string `json:"role"`
	Headless     bool   `json:"headless"`
	URL          string `json:"url"`
	Port         int    `json:"port"`
	Reuse        bool   `json:"reuse_existing"`
	UserDataDir  string `json:"user_data_dir"`
	DebugAddress string `json:"debug_address"`
}

type debugURLResponse struct {
	WebSocketURL string `json:"websocket_url"`
	PID          int    `json:"pid"`
	Port         int    `json:"port"`
	IsNew        bool   `json:"is_new"`
}

func requestRemoteServiceDebugURL(nodeName string, servicePort int, req debugURLRequest) (string, error) {
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(nodeName))
	if err != nil {
		return "", err
	}
	host := strings.TrimSpace(node.Host)
	if host == "" {
		return "", fmt.Errorf("mesh host empty for %s", strings.TrimSpace(nodeName))
	}
	if servicePort <= 0 {
		servicePort = defaultChromeServicePort
	}
	raw, _ := json.Marshal(req)
	client := &http.Client{Timeout: 2 * time.Second}
	httpURL := fmt.Sprintf("http://%s:%d/debug-url", host, servicePort)
	resp, err := client.Post(httpURL, "application/json", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return "", fmt.Errorf("remote chrome service %s failed: %s", httpURL, msg)
	}
	var out debugURLResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	ws := strings.TrimSpace(out.WebSocketURL)
	if ws == "" {
		return "", fmt.Errorf("remote chrome service returned empty websocket url")
	}
	if hostWS := normalizeRemoteServiceWebSocket(ws, host); hostWS != "" {
		return hostWS, nil
	}
	return ws, nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func parseBoolQuery(v string, fallback bool) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func parseIntQuery(v string, fallback int) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func normalizeRemoteServiceWebSocket(rawWS, host string) string {
	rawWS = strings.TrimSpace(rawWS)
	host = strings.TrimSpace(host)
	if rawWS == "" || host == "" {
		return ""
	}
	u, err := url.Parse(rawWS)
	if err != nil {
		return ""
	}
	port, _ := strconv.Atoi(strings.TrimSpace(u.Port()))
	if port <= 0 {
		return ""
	}
	path := strings.TrimSpace(u.Path)
	if path == "" {
		path = "/"
	}
	// Prefer direct debugger port first.
	if canDialHost(host, port, 900*time.Millisecond) {
		return fmt.Sprintf("ws://%s:%d%s", host, port, path)
	}
	// If relay convention is enabled remotely, fall back to +10000.
	if canDialHost(host, port+10000, 900*time.Millisecond) {
		return fmt.Sprintf("ws://%s:%d%s", host, port+10000, path)
	}
	return fmt.Sprintf("ws://%s:%d%s", host, port, path)
}

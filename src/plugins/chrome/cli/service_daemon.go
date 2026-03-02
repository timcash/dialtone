package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

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

type openRequest struct {
	debugURLRequest
	Fullscreen bool `json:"fullscreen"`
	Kiosk      bool `json:"kiosk"`
}

type actionRequest struct {
	debugURLRequest
	Action   string `json:"action"`
	Selector string `json:"selector"`
	Text     string `json:"text"`
}

type actionResponse struct {
	OK           bool     `json:"ok"`
	Error        string   `json:"error,omitempty"`
	WebSocketURL string   `json:"websocket_url,omitempty"`
	PID          int      `json:"pid,omitempty"`
	Port         int      `json:"port,omitempty"`
	Logs         []string `json:"logs,omitempty"`
}

func requestRemoteServiceDebugURL(nodeName string, servicePort int, req debugURLRequest) (string, error) {
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(nodeName))
	if err != nil {
		return "", err
	}
	primaryHost := strings.TrimSpace(node.Host)
	if primaryHost == "" {
		return "", fmt.Errorf("mesh host empty for %s", strings.TrimSpace(nodeName))
	}
	if servicePort <= 0 {
		servicePort = defaultChromeServicePort
	}
	if strings.EqualFold(strings.TrimSpace(node.OS), "windows") {
		body, err := postRemoteServiceViaSSH(node, servicePort, "/debug-url", req, 20)
		if err != nil {
			return "", err
		}
		var out debugURLResponse
		if err := json.Unmarshal(body, &out); err != nil {
			return "", err
		}
		ws := strings.TrimSpace(out.WebSocketURL)
		if ws == "" {
			return "", fmt.Errorf("remote chrome service returned empty websocket url")
		}
		if hostWS := normalizeRemoteServiceWebSocket(ws, "127.0.0.1"); hostWS != "" {
			return hostWS, nil
		}
		return ws, nil
	}
	hostCandidates := []string{primaryHost}
	raw, _ := json.Marshal(req)
	client := &http.Client{Timeout: 12 * time.Second}
	var resp *http.Response
	var usedHost string
	var lastErr error
	for _, candidate := range hostCandidates {
		httpURL := fmt.Sprintf("http://%s:%d/debug-url", strings.TrimSpace(candidate), servicePort)
		resp, err = client.Post(httpURL, "application/json", bytes.NewReader(raw))
		if err != nil {
			lastErr = err
			continue
		}
		usedHost = strings.TrimSpace(candidate)
		break
	}
	if resp == nil {
		if lastErr != nil {
			return "", lastErr
		}
		return "", fmt.Errorf("remote chrome service request failed")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return "", fmt.Errorf("remote chrome service host=%s port=%d failed: %s", usedHost, servicePort, msg)
	}
	var out debugURLResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	ws := strings.TrimSpace(out.WebSocketURL)
	if ws == "" {
		return "", fmt.Errorf("remote chrome service returned empty websocket url")
	}
	if hostWS := normalizeRemoteServiceWebSocket(ws, usedHost); hostWS != "" {
		return hostWS, nil
	}
	return ws, nil
}

func requestRemoteServiceOpen(nodeName string, servicePort int, req openRequest) (string, error) {
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(nodeName))
	if err != nil {
		return "", err
	}
	primaryHost := strings.TrimSpace(node.Host)
	if primaryHost == "" {
		return "", fmt.Errorf("mesh host empty for %s", strings.TrimSpace(nodeName))
	}
	if servicePort <= 0 {
		servicePort = defaultChromeServicePort
	}
	if strings.EqualFold(strings.TrimSpace(node.OS), "windows") {
		body, err := postRemoteServiceViaSSH(node, servicePort, "/open", req, 25)
		if err != nil {
			return "", err
		}
		var out debugURLResponse
		if err := json.Unmarshal(body, &out); err != nil {
			return "", err
		}
		ws := strings.TrimSpace(out.WebSocketURL)
		if ws == "" {
			return "", fmt.Errorf("remote chrome service returned empty websocket url")
		}
		if hostWS := normalizeRemoteServiceWebSocket(ws, "127.0.0.1"); hostWS != "" {
			return hostWS, nil
		}
		return ws, nil
	}
	hostCandidates := []string{primaryHost}
	raw, _ := json.Marshal(req)
	client := &http.Client{Timeout: 20 * time.Second}
	var resp *http.Response
	var usedHost string
	var lastErr error
	for _, candidate := range hostCandidates {
		httpURL := fmt.Sprintf("http://%s:%d/open", strings.TrimSpace(candidate), servicePort)
		resp, err = client.Post(httpURL, "application/json", bytes.NewReader(raw))
		if err != nil {
			lastErr = err
			continue
		}
		usedHost = strings.TrimSpace(candidate)
		break
	}
	if resp == nil {
		if lastErr != nil {
			return "", lastErr
		}
		return "", fmt.Errorf("remote chrome open request failed")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return "", fmt.Errorf("remote chrome service host=%s port=%d open failed: %s", usedHost, servicePort, msg)
	}
	var out debugURLResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	ws := strings.TrimSpace(out.WebSocketURL)
	if ws == "" {
		return "", fmt.Errorf("remote chrome service returned empty websocket url")
	}
	if hostWS := normalizeRemoteServiceWebSocket(ws, usedHost); hostWS != "" {
		return hostWS, nil
	}
	return ws, nil
}

func requestRemoteServiceAction(nodeName string, servicePort int, req actionRequest) (*actionResponse, error) {
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(nodeName))
	if err != nil {
		return nil, err
	}
	primaryHost := strings.TrimSpace(node.Host)
	if primaryHost == "" {
		return nil, fmt.Errorf("mesh host empty for %s", strings.TrimSpace(nodeName))
	}
	if servicePort <= 0 {
		servicePort = defaultChromeServicePort
	}
	if strings.EqualFold(strings.TrimSpace(node.OS), "windows") {
		body, err := postRemoteServiceViaSSH(node, servicePort, "/action", req, 35)
		if err != nil {
			return nil, err
		}
		var out actionResponse
		if err := json.Unmarshal(body, &out); err != nil {
			msg := strings.TrimSpace(string(body))
			if msg == "" {
				msg = err.Error()
			}
			return nil, fmt.Errorf("remote chrome action invalid response: %s", msg)
		}
		if !out.OK {
			msg := strings.TrimSpace(out.Error)
			if msg == "" {
				msg = "remote chrome action failed"
			}
			return nil, fmt.Errorf("remote chrome action failed: %s", msg)
		}
		return &out, nil
	}
	hostCandidates := []string{primaryHost}
	raw, _ := json.Marshal(req)
	client := &http.Client{Timeout: 30 * time.Second}
	var resp *http.Response
	var lastErr error
	for _, candidate := range hostCandidates {
		httpURL := fmt.Sprintf("http://%s:%d/action", strings.TrimSpace(candidate), servicePort)
		resp, err = client.Post(httpURL, "application/json", bytes.NewReader(raw))
		if err != nil {
			lastErr = err
			continue
		}
		break
	}
	if resp == nil {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("remote chrome action request failed")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out actionResponse
	if err := json.Unmarshal(body, &out); err != nil {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("remote chrome action invalid response: %s", msg)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(out.Error)
		if msg == "" {
			msg = strings.TrimSpace(string(body))
		}
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("remote chrome action failed: %s", msg)
	}
	return &out, nil
}

func postRemoteServiceViaSSH(node sshv1.MeshNode, servicePort int, endpoint string, payload any, timeoutSec int) ([]byte, error) {
	if timeoutSec <= 0 {
		timeoutSec = 20
	}
	if servicePort <= 0 {
		servicePort = defaultChromeServicePort
	}
	raw, _ := json.Marshal(payload)
	enc := base64.StdEncoding.EncodeToString(raw)
	ps := fmt.Sprintf(`$b='%s'; $j=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($b)); try { $r=Invoke-WebRequest -UseBasicParsing -Method POST -Uri 'http://127.0.0.1:%d%s' -ContentType 'application/json' -Body $j -TimeoutSec %d; Write-Output $r.Content; exit 0 } catch { if($_.Exception.Response -and $_.Exception.Response.GetResponseStream()){ $sr=New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); $body=$sr.ReadToEnd(); if($body){ Write-Output $body; exit 1 } }; Write-Output $_.Exception.Message; exit 1 }`, enc, servicePort, endpoint, timeoutSec)
	out, err := sshv1.RunNodeCommand(node.Name, "powershell -NoProfile -EncodedCommand "+encodePowerShellEncodedCommand(ps), sshv1.CommandOptions{})
	if err != nil {
		msg := strings.TrimSpace(out)
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("remote chrome service via ssh failed node=%s endpoint=%s: %s", node.Name, endpoint, msg)
	}
	body := []byte(strings.TrimSpace(out))
	if len(body) == 0 {
		return nil, fmt.Errorf("remote chrome service via ssh returned empty response")
	}
	return body, nil
}

func encodePowerShellEncodedCommand(script string) string {
	utf16Vals := utf16.Encode([]rune(script))
	buf := make([]byte, len(utf16Vals)*2)
	for i, v := range utf16Vals {
		binary.LittleEndian.PutUint16(buf[i*2:], v)
	}
	return base64.StdEncoding.EncodeToString(buf)
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

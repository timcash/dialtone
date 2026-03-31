package repl

import (
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

const leaderHealthSubject = "repl.leader.health"

type LeaderState struct {
	PID              int    `json:"pid"`
	NATSURL          string `json:"nats_url"`
	TSNetNATSURL     string `json:"tsnet_nats_url,omitempty"`
	Topic            string `json:"topic,omitempty"`
	Room             string `json:"room,omitempty"`
	HostName         string `json:"hostname"`
	ServerID         string `json:"server_id"`
	Version          string `json:"version"`
	StartedAt        string `json:"started_at"`
	LastHealthyAt    string `json:"last_healthy_at"`
	EmbeddedNATS     bool   `json:"embedded_nats"`
	Running          bool   `json:"running"`
	StoppedAt        string `json:"stopped_at,omitempty"`
	BootstrapHTTPURL string `json:"bootstrap_http_url,omitempty"`
	BootstrapHTTPPID int    `json:"bootstrap_http_pid,omitempty"`
}

func leaderStateDir() (string, error) {
	dir := filepath.Join(configv1.DefaultDialtoneHome(), "repl-v3")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func leaderStatePath() (string, error) {
	dir, err := leaderStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "leader.json"), nil
}

func readLeaderState() (LeaderState, error) {
	var st LeaderState
	path, err := leaderStatePath()
	if err != nil {
		return st, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return st, err
	}
	if strings.TrimSpace(st.Topic) == "" {
		st.Topic = sanitizeRoom(st.Room)
	}
	if strings.TrimSpace(st.Room) == "" {
		st.Room = sanitizeRoom(st.Topic)
	}
	return st, nil
}

func writeLeaderState(st LeaderState) error {
	path, err := leaderStatePath()
	if err != nil {
		return err
	}
	if strings.TrimSpace(st.Topic) == "" {
		st.Topic = sanitizeRoom(st.Room)
	}
	st.Room = ""
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return err
	}
	return syncLeaderRuntimeConfig(st)
}

func buildLeaderState(usedURL, tsnetURL, room, hostName, serverID string, embedded bool, startedAt time.Time) LeaderState {
	topic := sanitizeRoom(room)
	st := LeaderState{
		PID:           os.Getpid(),
		NATSURL:       leaderClientNATSURL(usedURL),
		TSNetNATSURL:  strings.TrimSpace(tsnetURL),
		Topic:         topic,
		HostName:      normalizePromptName(hostName),
		ServerID:      strings.TrimSpace(serverID),
		Version:       strings.TrimSpace(BuildVersion),
		StartedAt:     startedAt.UTC().Format(time.RFC3339Nano),
		LastHealthyAt: time.Now().UTC().Format(time.RFC3339Nano),
		EmbeddedNATS:  embedded,
		Running:       true,
	}
	if st.Topic == "" {
		st.Topic = defaultRoom
	}
	if host := strings.TrimSpace(os.Getenv("DIALTONE_REPL_BOOTSTRAP_HTTP_HOST")); host != "" {
		port := strings.TrimSpace(os.Getenv("DIALTONE_REPL_BOOTSTRAP_HTTP_PORT"))
		if port == "" {
			port = "8811"
		}
		st.BootstrapHTTPURL = fmt.Sprintf("http://%s:%s/install.sh", host, port)
	}
	return st
}

func leaderClientNATSURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" || host == "0.0.0.0" || host == "::" || host == "localhost" {
		port := parsed.Port()
		if port == "" {
			port = "4222"
		}
		parsed.Host = net.JoinHostPort("127.0.0.1", port)
		return parsed.String()
	}
	return raw
}

func writeLeaderStateHeartbeat(usedURL, tsnetURL, room, hostName, serverID string, embedded bool, startedAt time.Time) error {
	return writeLeaderState(buildLeaderState(usedURL, tsnetURL, room, hostName, serverID, embedded, startedAt))
}

func markLeaderStopped() {
	st, err := readLeaderState()
	if err != nil {
		return
	}
	if st.PID != os.Getpid() {
		return
	}
	st.Running = false
	st.StoppedAt = time.Now().UTC().Format(time.RFC3339Nano)
	st.LastHealthyAt = st.StoppedAt
	_ = writeLeaderState(st)
}

func syncLeaderRuntimeConfig(st LeaderState) error {
	managerURL := strings.TrimSpace(st.TSNetNATSURL)
	if managerURL == "" && st.Running {
		managerURL = strings.TrimSpace(st.NATSURL)
	}
	topic := sanitizeRoom(st.Topic)
	if topic == "" {
		topic = sanitizeRoom(st.Room)
	}
	if topic == "" {
		topic = defaultRoom
	}
	updates := map[string]any{
		"DIALTONE_REPL_RUNNING":         "0",
		"DIALTONE_REPL_TOPIC":           topic,
		"DIALTONE_REPL_HOSTNAME":        normalizePromptName(st.HostName),
		"DIALTONE_REPL_SERVER_ID":       strings.TrimSpace(st.ServerID),
		"DIALTONE_REPL_LAST_HEALTHY_AT": strings.TrimSpace(st.LastHealthyAt),
	}
	if strings.TrimSpace(st.StartedAt) != "" {
		updates["DIALTONE_REPL_STARTED_AT"] = strings.TrimSpace(st.StartedAt)
	}
	if st.Running {
		updates["DIALTONE_REPL_RUNNING"] = "1"
		if raw := strings.TrimSpace(st.NATSURL); raw != "" {
			updates["DIALTONE_REPL_NATS_URL"] = raw
			_ = os.Setenv("DIALTONE_REPL_NATS_URL", raw)
		}
		if raw := strings.TrimSpace(managerURL); raw != "" {
			updates["DIALTONE_REPL_MANAGER_NATS_URL"] = raw
			_ = os.Setenv("DIALTONE_REPL_MANAGER_NATS_URL", raw)
		}
		if raw := strings.TrimSpace(st.TSNetNATSURL); raw != "" {
			updates["DIALTONE_REPL_TSNET_NATS_URL"] = raw
			_ = os.Setenv("DIALTONE_REPL_TSNET_NATS_URL", raw)
		} else {
			updates["DIALTONE_REPL_TSNET_NATS_URL"] = nil
			_ = os.Unsetenv("DIALTONE_REPL_TSNET_NATS_URL")
		}
		if raw := strings.TrimSpace(st.BootstrapHTTPURL); raw != "" {
			updates["DIALTONE_REPL_BOOTSTRAP_HTTP_URL"] = raw
		} else {
			updates["DIALTONE_REPL_BOOTSTRAP_HTTP_URL"] = nil
		}
	} else {
		for _, key := range []string{
			"DIALTONE_REPL_NATS_URL",
			"DIALTONE_REPL_MANAGER_NATS_URL",
			"DIALTONE_REPL_TSNET_NATS_URL",
			"DIALTONE_REPL_BOOTSTRAP_HTTP_URL",
		} {
			updates[key] = nil
			_ = os.Unsetenv(key)
		}
	}
	for _, pair := range []struct {
		Key   string
		Value string
	}{
		{Key: "DIALTONE_REPL_RUNNING", Value: "0"},
		{Key: "DIALTONE_REPL_TOPIC", Value: topic},
		{Key: "DIALTONE_REPL_HOSTNAME", Value: normalizePromptName(st.HostName)},
		{Key: "DIALTONE_REPL_SERVER_ID", Value: strings.TrimSpace(st.ServerID)},
		{Key: "DIALTONE_REPL_LAST_HEALTHY_AT", Value: strings.TrimSpace(st.LastHealthyAt)},
		{Key: "DIALTONE_REPL_STARTED_AT", Value: strings.TrimSpace(st.StartedAt)},
	} {
		if strings.TrimSpace(pair.Value) == "" {
			_ = os.Unsetenv(pair.Key)
			continue
		}
		_ = os.Setenv(pair.Key, pair.Value)
	}
	if st.Running {
		_ = os.Setenv("DIALTONE_REPL_RUNNING", "1")
	}
	if err := configv1.UpdateRuntimeEnvFile("", updates); err != nil {
		return err
	}
	return nil
}

func leaderHealth(natsURL string, timeout time.Duration) (LeaderState, error) {
	var st LeaderState
	nc, err := nats.Connect(strings.TrimSpace(natsURL), nats.Timeout(timeout))
	if err != nil {
		return st, err
	}
	defer nc.Close()
	msg, err := nc.Request(leaderHealthSubject, []byte("health"), timeout)
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(msg.Data, &st); err != nil {
		return st, err
	}
	if strings.TrimSpace(st.Topic) == "" {
		st.Topic = sanitizeRoom(st.Room)
	}
	if strings.TrimSpace(st.Room) == "" {
		st.Room = sanitizeRoom(st.Topic)
	}
	if !st.Running || st.PID <= 0 {
		return st, fmt.Errorf("leader reported unhealthy state")
	}
	return st, nil
}

func LeaderHealthy(natsURL string, timeout time.Duration) bool {
	_, err := leaderHealth(strings.TrimSpace(natsURL), timeout)
	return err == nil
}

package repl

import (
	"encoding/json"
	"fmt"
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
	Room             string `json:"room"`
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
	repoRoot, _, err := resolveRoots()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(repoRoot, ".dialtone", "repl-v3")
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
	return st, nil
}

func writeLeaderState(st LeaderState) error {
	path, err := leaderStatePath()
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func buildLeaderState(usedURL, room, hostName, serverID string, embedded bool, startedAt time.Time) LeaderState {
	st := LeaderState{
		PID:           os.Getpid(),
		NATSURL:       strings.TrimSpace(usedURL),
		Room:          sanitizeRoom(room),
		HostName:      normalizePromptName(hostName),
		ServerID:      strings.TrimSpace(serverID),
		Version:       strings.TrimSpace(BuildVersion),
		StartedAt:     startedAt.UTC().Format(time.RFC3339Nano),
		LastHealthyAt: time.Now().UTC().Format(time.RFC3339Nano),
		EmbeddedNATS:  embedded,
		Running:       true,
	}
	if st.Room == "" {
		st.Room = defaultRoom
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

func writeLeaderStateHeartbeat(usedURL, room, hostName, serverID string, embedded bool, startedAt time.Time) error {
	return writeLeaderState(buildLeaderState(usedURL, room, hostName, serverID, embedded, startedAt))
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
	if !st.Running || st.PID <= 0 {
		return st, fmt.Errorf("leader reported unhealthy state")
	}
	return st, nil
}

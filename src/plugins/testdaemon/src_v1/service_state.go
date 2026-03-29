package testdaemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	"github.com/shirou/gopsutil/v3/process"
)

const defaultHeartbeatInterval = time.Second

type servicePaths struct {
	rootDir             string
	controlDir          string
	statePath           string
	logPath             string
	pauseHeartbeatPath  string
	shutdownRequestPath string
}

type serviceState struct {
	Name              string `json:"name,omitempty"`
	Host              string `json:"host,omitempty"`
	PID               int    `json:"pid,omitempty"`
	StartedAt         string `json:"started_at,omitempty"`
	UpdatedAt         string `json:"updated_at,omitempty"`
	LastHeartbeat     string `json:"last_heartbeat,omitempty"`
	HeartbeatInterval string `json:"heartbeat_interval,omitempty"`
	HeartbeatPaused   bool   `json:"heartbeat_paused,omitempty"`
	Running           bool   `json:"running,omitempty"`
	Health            string `json:"health,omitempty"`
	ExitCode          int    `json:"exit_code,omitempty"`
	ExitReason        string `json:"exit_reason,omitempty"`
	LogPath           string `json:"log_path,omitempty"`
}

func resolveServicePaths(name string) (servicePaths, error) {
	name = sanitizeName(name)
	if name == "" {
		return servicePaths{}, fmt.Errorf("service name is required")
	}
	base := filepath.Join(configv1.DefaultDialtoneHome(), "testdaemon", "services", name)
	controlDir := filepath.Join(base, "control")
	return servicePaths{
		rootDir:             base,
		controlDir:          controlDir,
		statePath:           filepath.Join(base, "state.json"),
		logPath:             filepath.Join(configv1.DefaultDialtoneHome(), "logs", fmt.Sprintf("testdaemon-service-%s.log", name)),
		pauseHeartbeatPath:  filepath.Join(controlDir, "heartbeats.paused"),
		shutdownRequestPath: filepath.Join(controlDir, "shutdown.requested"),
	}, nil
}

func loadServiceState(paths servicePaths) (serviceState, error) {
	raw, err := os.ReadFile(paths.statePath)
	if err != nil {
		return serviceState{}, err
	}
	var state serviceState
	if err := json.Unmarshal(raw, &state); err != nil {
		return serviceState{}, err
	}
	return normalizeServiceState(state), nil
}

func normalizeServiceState(state serviceState) serviceState {
	state.Name = strings.TrimSpace(state.Name)
	state.Host = strings.TrimSpace(state.Host)
	state.LogPath = strings.TrimSpace(state.LogPath)
	state.HeartbeatInterval = strings.TrimSpace(state.HeartbeatInterval)
	state.ExitReason = strings.TrimSpace(state.ExitReason)
	state.Health = deriveServiceHealth(state)
	return state
}

func writeServiceState(paths servicePaths, state serviceState) error {
	state = normalizeServiceState(state)
	if err := os.MkdirAll(filepath.Dir(paths.statePath), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmpPath := paths.statePath + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, paths.statePath)
}

func deriveServiceHealth(state serviceState) string {
	if !state.Running {
		return "stopped"
	}
	if state.PID <= 0 || !processAlive(state.PID) {
		return "missing"
	}
	if state.HeartbeatPaused {
		return "paused"
	}
	lastHeartbeat := parseRFC3339(state.LastHeartbeat)
	if lastHeartbeat.IsZero() {
		return "unknown"
	}
	if time.Since(lastHeartbeat) > heartbeatFreshWindow(state.HeartbeatInterval) {
		return "stale"
	}
	return "healthy"
}

func heartbeatFreshWindow(raw string) time.Duration {
	interval, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || interval <= 0 {
		interval = defaultHeartbeatInterval
	}
	return interval * 3
}

func parseRFC3339(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return false
	}
	running, err := proc.IsRunning()
	if err != nil {
		return false
	}
	return running
}

func terminateProcess(pid int) error {
	if pid <= 0 {
		return nil
	}
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return err
	}
	return proc.Terminate()
}

func killProcess(pid int) error {
	if pid <= 0 {
		return nil
	}
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return err
	}
	return proc.Kill()
}

func waitForServiceState(paths servicePaths, timeout time.Duration, predicate func(serviceState) bool) (serviceState, error) {
	if predicate == nil {
		return serviceState{}, fmt.Errorf("service predicate is required")
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		state, err := loadServiceState(paths)
		if err == nil && predicate(state) {
			return state, nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	if state, err := loadServiceState(paths); err == nil {
		return state, fmt.Errorf("timed out waiting for service %s state transition", state.Name)
	}
	return serviceState{}, fmt.Errorf("timed out waiting for service state file %s", paths.statePath)
}

func fileExists(path string) bool {
	_, err := os.Stat(strings.TrimSpace(path))
	return err == nil
}

func writeMarkerFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o644)
}

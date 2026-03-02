package cli

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type processStat struct {
	PID     int     `json:"pid"`
	Name    string  `json:"name"`
	CPU     float64 `json:"cpu"`
	MemMB   float64 `json:"mem_mb"`
	Command string  `json:"command,omitempty"`
}

type processStatsResponse struct {
	Host      string        `json:"host"`
	OS        string        `json:"os"`
	UpdatedAt string        `json:"updated_at"`
	Count     int           `json:"count"`
	Processes []processStat `json:"processes"`
}

func buildProcessStatsResponse(limit int) processStatsResponse {
	procs, err := collectTopProcesses(limit)
	if err != nil {
		procs = []processStat{{
			PID:     0,
			Name:    "error",
			CPU:     0,
			MemMB:   0,
			Command: err.Error(),
		}}
	}
	hn, _ := osHostname()
	return processStatsResponse{
		Host:      hn,
		OS:        runtime.GOOS,
		UpdatedAt: time.Now().Format(time.RFC3339),
		Count:     len(procs),
		Processes: procs,
	}
}

func deriveProcessName(fallback, command string) string {
	cmd := strings.TrimSpace(command)
	lc := strings.ToLower(cmd)
	if strings.Contains(lc, "sshd-session") {
		return "sshd-session"
	}
	if strings.Contains(lc, "sshd:") || strings.Contains(lc, "(sshd)") {
		return "sshd"
	}
	if cmd != "" {
		first := strings.Fields(cmd)
		if len(first) > 0 {
			token := strings.Trim(first[0], "\"'")
			base := strings.TrimSpace(filepath.Base(token))
			if base != "" && base != "." && base != "/" {
				return base
			}
		}
	}
	return strings.TrimSpace(fallback)
}

func normalizeProcessLimit(raw string) int {
	limit := parseIntQuery(raw, 30)
	if limit <= 0 {
		limit = 30
	}
	if limit > 200 {
		limit = 200
	}
	return limit
}

func osHostname() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "hostname")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

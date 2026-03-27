package repl

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"dialtone/dev/plugins/proc/src_v1/go/proc"
)

type managedHeartbeat struct {
	Host        string  `json:"host,omitempty"`
	Kind        string  `json:"kind,omitempty"`
	Name        string  `json:"name,omitempty"`
	Mode        string  `json:"mode,omitempty"`
	PID         int     `json:"pid,omitempty"`
	Room        string  `json:"room,omitempty"`
	Command     string  `json:"command,omitempty"`
	State       string  `json:"state,omitempty"`
	LogPath     string  `json:"log_path,omitempty"`
	StartedAt   string  `json:"started_at,omitempty"`
	LastOKAt    string  `json:"last_ok_at,omitempty"`
	UptimeSec   int64   `json:"uptime_sec,omitempty"`
	CPUPercent  float64 `json:"cpu_percent,omitempty"`
	PortCount   int     `json:"port_count,omitempty"`
	Ports       []int   `json:"ports,omitempty"`
	ExitCode    int     `json:"exit_code,omitempty"`
	ServiceName string  `json:"service_name,omitempty"`
}

func encodeManagedHeartbeat(h managedHeartbeat) ([]byte, error) {
	return json.Marshal(h)
}

func heartbeatSubject(host, mode, serviceName string, pid int) string {
	host = subjectToken(host)
	mode = subjectToken(mode)
	if mode == "" {
		mode = "foreground"
	}
	if host == "" {
		host = "local"
	}
	if strings.TrimSpace(serviceName) != "" {
		return fmt.Sprintf("repl.host.%s.heartbeat.service.%s", host, subjectToken(serviceName))
	}
	if pid > 0 {
		return fmt.Sprintf("repl.host.%s.heartbeat.%s.%d", host, mode, pid)
	}
	return fmt.Sprintf("repl.host.%s.heartbeat.%s.unknown", host, mode)
}

func buildManagedHeartbeat(host, room, mode, serviceName string, ev proc.SubtoneEvent, state string, exitCode int) managedHeartbeat {
	now := time.Now().UTC()
	h := managedHeartbeat{
		Host:        strings.TrimSpace(host),
		Kind:        "process",
		Name:        strings.TrimSpace(serviceName),
		Mode:        defaultManagedMode(mode),
		PID:         ev.PID,
		Room:        sanitizeRoom(room),
		Command:     strings.TrimSpace(strings.Join(ev.Args, " ")),
		State:       strings.TrimSpace(state),
		LogPath:     strings.TrimSpace(ev.LogPath),
		StartedAt:   ev.StartedAt.UTC().Format(time.RFC3339),
		LastOKAt:    now.Format(time.RFC3339),
		ExitCode:    exitCode,
		ServiceName: strings.TrimSpace(serviceName),
	}
	if strings.TrimSpace(serviceName) != "" {
		h.Kind = "service"
	}
	if !ev.StartedAt.IsZero() {
		uptime := now.Sub(ev.StartedAt).Round(time.Second)
		if uptime < 0 {
			uptime = 0
		}
		h.UptimeSec = int64(uptime / time.Second)
	}
	for _, snap := range listManagedFn() {
		if snap.PID != ev.PID {
			continue
		}
		h.CPUPercent = snap.CPUPercent
		h.PortCount = snap.PortCount
		break
	}
	return h
}

func subjectToken(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "._-")
	if out == "" {
		return "unknown"
	}
	return out
}

func heartbeatStateToken(active bool, exitCode int) string {
	if active {
		return "running"
	}
	if exitCode < 0 {
		return "stopped"
	}
	return "exited"
}

func heartbeatIdentifier(serviceName string, pid int) string {
	if strings.TrimSpace(serviceName) != "" {
		return strings.TrimSpace(serviceName)
	}
	if pid > 0 {
		return strconv.Itoa(pid)
	}
	return "unknown"
}

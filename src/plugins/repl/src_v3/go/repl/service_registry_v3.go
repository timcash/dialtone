package repl

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"dialtone/dev/plugins/proc/src_v1/go/proc"
)

const serviceRegistrySubject = "repl.registry.services"

type serviceRegistryRequest struct {
	Count int `json:"count,omitempty"`
}

type serviceRegistryItem struct {
	Name          string  `json:"name,omitempty"`
	Host          string  `json:"host,omitempty"`
	PID           int     `json:"pid"`
	Room          string  `json:"room,omitempty"`
	Command       string  `json:"command,omitempty"`
	Mode          string  `json:"mode,omitempty"`
	LogPath       string  `json:"log_path,omitempty"`
	StartedAt     string  `json:"started_at,omitempty"`
	LastUpdate    string  `json:"last_update,omitempty"`
	LastHeartbeat string  `json:"last_heartbeat,omitempty"`
	ExitCode      int     `json:"exit_code,omitempty"`
	Active        bool    `json:"active"`
	CPUPercent    float64 `json:"cpu_percent,omitempty"`
	PortCount     int     `json:"port_count,omitempty"`
	StartedAgo    string  `json:"started_ago,omitempty"`
}

type serviceRegistryEntry struct {
	Name          string
	Host          string
	PID           int
	Room          string
	Command       string
	Mode          string
	LogPath       string
	StartedAt     time.Time
	LastUpdate    time.Time
	LastHeartbeat time.Time
	ExitCode      int
	Active        bool
}

type serviceRegistry struct {
	mu      sync.Mutex
	limit   int
	entries map[string]*serviceRegistryEntry
}

func newServiceRegistry(limit int) *serviceRegistry {
	if limit <= 0 {
		limit = 256
	}
	return &serviceRegistry{
		limit:   limit,
		entries: map[string]*serviceRegistryEntry{},
	}
}

func (r *serviceRegistry) Started(name, room string, ev proc.TaskWorkerEvent) {
	if r == nil || ev.PID <= 0 {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	now := time.Now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[name] = &serviceRegistryEntry{
		Name:          name,
		Host:          "local",
		PID:           ev.PID,
		Room:          sanitizeRoom(room),
		Command:       strings.TrimSpace(strings.Join(ev.Args, " ")),
		Mode:          "service",
		LogPath:       strings.TrimSpace(ev.LogPath),
		StartedAt:     ev.StartedAt.UTC(),
		LastUpdate:    now,
		LastHeartbeat: now,
		Active:        true,
	}
	r.trimLocked()
}

func (r *serviceRegistry) Heartbeat(name string) {
	if r == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	now := time.Now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entries[name]
	if !ok {
		return
	}
	entry.LastUpdate = now
	entry.LastHeartbeat = now
	r.trimLocked()
}

func (r *serviceRegistry) ObserveHeartbeat(h managedHeartbeat) {
	if r == nil {
		return
	}
	name := strings.TrimSpace(h.Name)
	if name == "" {
		name = strings.TrimSpace(h.ServiceName)
	}
	if name == "" || !strings.EqualFold(strings.TrimSpace(h.Kind), "service") {
		return
	}
	host := strings.TrimSpace(h.Host)
	if host == "" {
		host = "local"
	}
	now := time.Now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entries[name]
	if !ok {
		entry = &serviceRegistryEntry{Name: name, Mode: "service"}
		r.entries[name] = entry
	}
	entry.Name = name
	entry.Host = host
	if h.PID > 0 {
		entry.PID = h.PID
	}
	if room := sanitizeRoom(h.Room); room != "" {
		entry.Room = room
	}
	if cmd := strings.TrimSpace(h.Command); cmd != "" {
		entry.Command = cmd
	}
	entry.Mode = defaultManagedMode(h.Mode)
	if logPath := strings.TrimSpace(h.LogPath); logPath != "" {
		entry.LogPath = logPath
	}
	if startedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(h.StartedAt)); err == nil {
		entry.StartedAt = startedAt
	}
	entry.LastUpdate = now
	if lastOK, err := time.Parse(time.RFC3339, strings.TrimSpace(h.LastOKAt)); err == nil {
		entry.LastHeartbeat = lastOK
	} else {
		entry.LastHeartbeat = now
	}
	entry.ExitCode = h.ExitCode
	entry.Active = strings.EqualFold(strings.TrimSpace(h.State), "running")
	r.trimLocked()
}

func (r *serviceRegistry) Exited(name string, pid, exitCode int) {
	if r == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	now := time.Now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entries[name]
	if !ok {
		entry = &serviceRegistryEntry{Name: name, PID: pid, Mode: "service"}
		r.entries[name] = entry
	}
	if pid > 0 {
		entry.PID = pid
	}
	entry.ExitCode = exitCode
	entry.Active = false
	entry.LastUpdate = now
	r.trimLocked()
}

func (r *serviceRegistry) Snapshot(count int, managed []proc.ManagedProcessSnapshot) []serviceRegistryItem {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	entries := make([]serviceRegistryEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		entries = append(entries, *entry)
	}
	r.mu.Unlock()

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastUpdate.After(entries[j].LastUpdate)
	})
	if count > 0 && len(entries) > count {
		entries = entries[:count]
	}

	managedByPID := map[int]proc.ManagedProcessSnapshot{}
	for _, snap := range managed {
		managedByPID[snap.PID] = snap
	}

	out := make([]serviceRegistryItem, 0, len(entries))
	now := time.Now()
	for _, entry := range entries {
		item := serviceRegistryItem{
			Name:     entry.Name,
			Host:     entry.Host,
			PID:      entry.PID,
			Room:     entry.Room,
			Command:  entry.Command,
			Mode:     entry.Mode,
			LogPath:  entry.LogPath,
			ExitCode: entry.ExitCode,
			Active:   entry.Active,
		}
		if !entry.StartedAt.IsZero() {
			item.StartedAt = entry.StartedAt.Format(time.RFC3339)
			uptime := now.Sub(entry.StartedAt).Round(time.Second)
			if uptime < 0 {
				uptime = 0
			}
			item.StartedAgo = uptime.String()
		}
		if !entry.LastUpdate.IsZero() {
			item.LastUpdate = entry.LastUpdate.Format(time.RFC3339)
		}
		if !entry.LastHeartbeat.IsZero() {
			item.LastHeartbeat = entry.LastHeartbeat.Format(time.RFC3339)
		}
		if snap, ok := managedByPID[entry.PID]; ok {
			item.CPUPercent = snap.CPUPercent
			item.PortCount = snap.PortCount
			if strings.TrimSpace(item.Command) == "" {
				item.Command = strings.TrimSpace(snap.Command)
			}
			if strings.TrimSpace(item.StartedAgo) == "" {
				item.StartedAgo = strings.TrimSpace(snap.StartedAgo.String())
			}
		}
		out = append(out, item)
	}
	return out
}

func (r *serviceRegistry) ActiveByName(name string) (serviceRegistryItem, bool) {
	if r == nil {
		return serviceRegistryItem{}, false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return serviceRegistryItem{}, false
	}
	items := r.Snapshot(0, listManagedFn())
	for _, item := range items {
		if item.Name == name && item.Active {
			return item, true
		}
	}
	return serviceRegistryItem{}, false
}

func (r *serviceRegistry) trimLocked() {
	if r.limit <= 0 || len(r.entries) <= r.limit {
		return
	}
	entries := make([]*serviceRegistryEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastUpdate.After(entries[j].LastUpdate)
	})
	for _, entry := range entries[r.limit:] {
		delete(r.entries, entry.Name)
	}
}

func encodeServiceRegistrySnapshot(items []serviceRegistryItem) ([]byte, error) {
	return json.Marshal(items)
}

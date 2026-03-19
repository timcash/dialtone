package repl

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"dialtone/dev/plugins/proc/src_v1/go/proc"
)

const subtoneRegistrySubject = "repl.registry.subtones"

type subtoneRegistryRequest struct {
	Count int `json:"count,omitempty"`
}

type subtoneRegistryItem struct {
	PID        int     `json:"pid"`
	Room       string  `json:"room,omitempty"`
	Command    string  `json:"command,omitempty"`
	Mode       string  `json:"mode,omitempty"`
	LogPath    string  `json:"log_path,omitempty"`
	StartedAt  string  `json:"started_at,omitempty"`
	LastUpdate string  `json:"last_update,omitempty"`
	ExitCode   int     `json:"exit_code,omitempty"`
	Active     bool    `json:"active"`
	CPUPercent float64 `json:"cpu_percent,omitempty"`
	PortCount  int     `json:"port_count,omitempty"`
	StartedAgo string  `json:"started_ago,omitempty"`
}

type subtoneRegistryEntry struct {
	PID        int
	Room       string
	Command    string
	Mode       string
	LogPath    string
	StartedAt  time.Time
	LastUpdate time.Time
	ExitCode   int
	Active     bool
}

type subtoneRegistry struct {
	mu      sync.Mutex
	limit   int
	entries map[int]*subtoneRegistryEntry
}

func newSubtoneRegistry(limit int) *subtoneRegistry {
	if limit <= 0 {
		limit = 256
	}
	return &subtoneRegistry{
		limit:   limit,
		entries: map[int]*subtoneRegistryEntry{},
	}
}

func (r *subtoneRegistry) Started(room string, mode string, ev proc.SubtoneEvent) {
	if r == nil || ev.PID <= 0 {
		return
	}
	now := time.Now().UTC()
	mode = strings.TrimSpace(mode)
	if mode == "" {
		mode = "foreground"
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[ev.PID] = &subtoneRegistryEntry{
		PID:        ev.PID,
		Room:       sanitizeRoom(room),
		Command:    strings.TrimSpace(strings.Join(ev.Args, " ")),
		Mode:       mode,
		LogPath:    strings.TrimSpace(ev.LogPath),
		StartedAt:  ev.StartedAt.UTC(),
		LastUpdate: now,
		Active:     true,
	}
	r.trimLocked()
}

func (r *subtoneRegistry) Exited(pid int, exitCode int) {
	if r == nil || pid <= 0 {
		return
	}
	now := time.Now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entries[pid]
	if !ok {
		entry = &subtoneRegistryEntry{PID: pid}
		r.entries[pid] = entry
	}
	entry.ExitCode = exitCode
	entry.Active = false
	entry.LastUpdate = now
	r.trimLocked()
}

func (r *subtoneRegistry) Heartbeat(pid int) {
	if r == nil || pid <= 0 {
		return
	}
	now := time.Now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entries[pid]
	if !ok {
		return
	}
	entry.LastUpdate = now
	r.trimLocked()
}

func (r *subtoneRegistry) Snapshot(count int, managed []proc.ManagedProcessSnapshot) []subtoneRegistryItem {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	entries := make([]subtoneRegistryEntry, 0, len(r.entries))
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

	out := make([]subtoneRegistryItem, 0, len(entries))
	now := time.Now()
	for _, entry := range entries {
		item := subtoneRegistryItem{
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

func (r *subtoneRegistry) Find(pid int) (subtoneRegistryItem, bool) {
	if r == nil || pid <= 0 {
		return subtoneRegistryItem{}, false
	}
	items := r.Snapshot(0, listManagedFn())
	for _, item := range items {
		if item.PID == pid {
			return item, true
		}
	}
	return subtoneRegistryItem{}, false
}

func (r *subtoneRegistry) trimLocked() {
	if r.limit <= 0 || len(r.entries) <= r.limit {
		return
	}
	entries := make([]*subtoneRegistryEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastUpdate.After(entries[j].LastUpdate)
	})
	for _, entry := range entries[r.limit:] {
		delete(r.entries, entry.PID)
	}
}

func encodeSubtoneRegistrySnapshot(items []subtoneRegistryItem) ([]byte, error) {
	return json.Marshal(items)
}

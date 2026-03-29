package repl

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"dialtone/dev/plugins/proc/src_v1/go/proc"
)

const taskRegistrySubject = "repl.registry.tasks"

type taskRegistryRequest struct {
	Count int `json:"count,omitempty"`
}

type taskRegistryItem struct {
	TaskID     string   `json:"task_id,omitempty"`
	PID        int      `json:"pid"`
	Host       string   `json:"host,omitempty"`
	Room       string   `json:"room,omitempty"`
	Topic      string   `json:"topic,omitempty"`
	Command    string   `json:"command,omitempty"`
	Args       []string `json:"args,omitempty"`
	Mode       string   `json:"mode,omitempty"`
	State      string   `json:"state,omitempty"`
	LogPath    string   `json:"log_path,omitempty"`
	CreatedAt  string   `json:"created_at,omitempty"`
	StartedAt  string   `json:"started_at,omitempty"`
	UpdatedAt  string   `json:"updated_at,omitempty"`
	LastUpdate string   `json:"last_update,omitempty"`
	ExitCode   int      `json:"exit_code,omitempty"`
	Active     bool     `json:"active"`
	CPUPercent float64  `json:"cpu_percent,omitempty"`
	PortCount  int      `json:"port_count,omitempty"`
	StartedAgo string   `json:"started_ago,omitempty"`
}

type taskRegistryEntry struct {
	TaskID     string
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

type taskRegistry struct {
	mu      sync.Mutex
	limit   int
	entries map[int]*taskRegistryEntry
}

func newTaskRegistry(limit int) *taskRegistry {
	if limit <= 0 {
		limit = 256
	}
	return &taskRegistry{
		limit:   limit,
		entries: map[int]*taskRegistryEntry{},
	}
}

func (r *taskRegistry) Started(taskID, room, mode, taskLogPath string, ev proc.SubtoneEvent) {
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
	r.entries[ev.PID] = &taskRegistryEntry{
		TaskID:     strings.TrimSpace(taskID),
		PID:        ev.PID,
		Room:       sanitizeRoom(room),
		Command:    strings.TrimSpace(strings.Join(ev.Args, " ")),
		Mode:       mode,
		LogPath:    strings.TrimSpace(taskLogPath),
		StartedAt:  ev.StartedAt.UTC(),
		LastUpdate: now,
		Active:     true,
	}
	r.trimLocked()
}

func (r *taskRegistry) Exited(pid int, exitCode int) {
	if r == nil || pid <= 0 {
		return
	}
	now := time.Now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entries[pid]
	if !ok {
		entry = &taskRegistryEntry{PID: pid}
		r.entries[pid] = entry
	}
	entry.ExitCode = exitCode
	entry.Active = false
	entry.LastUpdate = now
	r.trimLocked()
}

func (r *taskRegistry) Heartbeat(pid int) {
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

func (r *taskRegistry) Snapshot(count int, managed []proc.ManagedProcessSnapshot) []taskRegistryItem {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	entries := make([]taskRegistryEntry, 0, len(r.entries))
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

	out := make([]taskRegistryItem, 0, len(entries))
	now := time.Now()
	for _, entry := range entries {
		item := taskRegistryItem{
			TaskID:   entry.TaskID,
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

func (r *taskRegistry) Find(pid int) (taskRegistryItem, bool) {
	if r == nil || pid <= 0 {
		return taskRegistryItem{}, false
	}
	items := r.Snapshot(0, listManagedFn())
	for _, item := range items {
		if item.PID == pid {
			return item, true
		}
	}
	return taskRegistryItem{}, false
}

func (r *taskRegistry) FindByTaskID(taskID string) (taskRegistryItem, bool) {
	if r == nil {
		return taskRegistryItem{}, false
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return taskRegistryItem{}, false
	}
	items := r.Snapshot(0, listManagedFn())
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.TaskID), taskID) {
			return item, true
		}
	}
	return taskRegistryItem{}, false
}

func (r *taskRegistry) trimLocked() {
	if r.limit <= 0 || len(r.entries) <= r.limit {
		return
	}
	entries := make([]*taskRegistryEntry, 0, len(r.entries))
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

func encodeTaskRegistrySnapshot(items []taskRegistryItem) ([]byte, error) {
	return json.Marshal(items)
}

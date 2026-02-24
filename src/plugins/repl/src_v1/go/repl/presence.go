package repl

import (
	"sort"
	"strings"
	"sync"
	"time"
)

type presenceRow struct {
	Kind          string
	Name          string
	Room          string
	Version       string
	DaemonVersion string
	ReplVersion   string
	OS            string
	Arch          string
}

type presenceTracker struct {
	mu      sync.RWMutex
	clients map[string]presenceRow
	daemons map[string]daemonPresence
}

type daemonPresence struct {
	Row      presenceRow
	LastSeen time.Time
}

func newPresenceTracker() *presenceTracker {
	return &presenceTracker{
		clients: map[string]presenceRow{},
		daemons: map[string]daemonPresence{},
	}
}

func (p *presenceTracker) UpsertClient(user, room, version, osName, arch string) {
	user = normalizePromptName(user)
	if user == "" {
		return
	}
	room = sanitizeRoom(room)
	p.mu.Lock()
	prev := p.clients[user]
	if strings.TrimSpace(version) == "" {
		version = prev.Version
	}
	if strings.TrimSpace(osName) == "" {
		osName = prev.OS
	}
	if strings.TrimSpace(arch) == "" {
		arch = prev.Arch
	}
	p.clients[user] = presenceRow{
		Kind:    "client",
		Name:    user,
		Room:    room,
		Version: strings.TrimSpace(version),
		OS:      strings.TrimSpace(osName),
		Arch:    strings.TrimSpace(arch),
	}
	p.mu.Unlock()
}

func (p *presenceTracker) UpsertDaemon(host, room, daemonVersion, replVersion, osName, arch string, now time.Time) {
	host = normalizePromptName(host)
	if host == "" {
		return
	}
	room = sanitizeRoom(room)
	row := presenceRow{
		Kind:          "daemon",
		Name:          host,
		Room:          room,
		DaemonVersion: strings.TrimSpace(daemonVersion),
		ReplVersion:   strings.TrimSpace(replVersion),
		OS:            strings.TrimSpace(osName),
		Arch:          strings.TrimSpace(arch),
	}
	p.mu.Lock()
	p.daemons[host] = daemonPresence{Row: row, LastSeen: now}
	p.mu.Unlock()
}

func (p *presenceTracker) RemoveClient(user string) {
	user = normalizePromptName(user)
	if user == "" {
		return
	}
	p.mu.Lock()
	delete(p.clients, user)
	p.mu.Unlock()
}

func (p *presenceTracker) ClientRoom(user string) string {
	user = normalizePromptName(user)
	if user == "" {
		return ""
	}
	p.mu.RLock()
	row, ok := p.clients[user]
	p.mu.RUnlock()
	if !ok {
		return ""
	}
	return sanitizeRoom(row.Room)
}

func (p *presenceTracker) Snapshot(now time.Time, daemonTTL time.Duration) []presenceRow {
	p.mu.RLock()
	out := make([]presenceRow, 0, len(p.clients)+len(p.daemons))
	for _, row := range p.clients {
		out = append(out, presenceRow{
			Kind:    row.Kind,
			Name:    row.Name,
			Room:    sanitizeRoom(row.Room),
			Version: strings.TrimSpace(row.Version),
			OS:      strings.TrimSpace(row.OS),
			Arch:    strings.TrimSpace(row.Arch),
		})
	}
	for _, d := range p.daemons {
		if daemonTTL > 0 && now.Sub(d.LastSeen) > daemonTTL {
			continue
		}
		row := d.Row
		out = append(out, presenceRow{
			Kind:          row.Kind,
			Name:          row.Name,
			Room:          sanitizeRoom(row.Room),
			DaemonVersion: strings.TrimSpace(row.DaemonVersion),
			ReplVersion:   strings.TrimSpace(row.ReplVersion),
			OS:            strings.TrimSpace(row.OS),
			Arch:          strings.TrimSpace(row.Arch),
		})
	}
	p.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (p *presenceTracker) Rooms(primaryRoom string, now time.Time, daemonTTL time.Duration) []string {
	seen := map[string]struct{}{sanitizeRoom(primaryRoom): {}}
	p.mu.RLock()
	for _, row := range p.clients {
		r := sanitizeRoom(row.Room)
		if strings.TrimSpace(r) == "" {
			continue
		}
		seen[r] = struct{}{}
	}
	for _, d := range p.daemons {
		if daemonTTL > 0 && now.Sub(d.LastSeen) > daemonTTL {
			continue
		}
		r := sanitizeRoom(d.Row.Room)
		if strings.TrimSpace(r) == "" {
			continue
		}
		seen[r] = struct{}{}
	}
	p.mu.RUnlock()
	out := make([]string, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

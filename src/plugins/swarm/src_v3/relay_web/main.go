package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type peerEntry struct {
	IP        string    `json:"ip"`
	Port      int       `json:"port"`
	Topic     string    `json:"topic"`
	Who       string    `json:"who"`
	UserAgent string    `json:"userAgent"`
	FirstSeen time.Time `json:"firstSeen"`
	LastSeen  time.Time `json:"lastSeen"`
	Hits      int       `json:"hits"`
}

type peerStore struct {
	mu       sync.RWMutex
	peers    map[string]*peerEntry
	ttl      time.Duration
	maxPeers int
}

func newPeerStore(ttl time.Duration, maxPeers int) *peerStore {
	return &peerStore{
		peers:    make(map[string]*peerEntry),
		ttl:      ttl,
		maxPeers: maxPeers,
	}
}

func (s *peerStore) key(ip string, port int, topic string) string {
	return topic + "|" + ip + ":" + strconv.Itoa(port)
}

func (s *peerStore) touch(ip string, port int, topic, who, userAgent string) {
	now := time.Now().UTC()
	if topic == "" {
		topic = "default"
	}
	if who == "" {
		who = "anonymous"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneLocked(now)

	key := s.key(ip, port, topic)
	p, ok := s.peers[key]
	if !ok {
		if len(s.peers) >= s.maxPeers {
			s.evictOldestLocked()
		}
		s.peers[key] = &peerEntry{
			IP:        ip,
			Port:      port,
			Topic:     topic,
			Who:       who,
			UserAgent: userAgent,
			FirstSeen: now,
			LastSeen:  now,
			Hits:      1,
		}
		return
	}

	p.Port = port
	p.Topic = topic
	p.Who = who
	p.UserAgent = userAgent
	p.LastSeen = now
	p.Hits++
}

func (s *peerStore) pruneLocked(now time.Time) {
	for k, p := range s.peers {
		if now.Sub(p.LastSeen) > s.ttl {
			delete(s.peers, k)
		}
	}
}

func (s *peerStore) evictOldestLocked() {
	if len(s.peers) == 0 {
		return
	}
	var oldestKey string
	var oldest time.Time
	first := true
	for k, p := range s.peers {
		if first || p.LastSeen.Before(oldest) {
			oldestKey = k
			oldest = p.LastSeen
			first = false
		}
	}
	delete(s.peers, oldestKey)
}

func (s *peerStore) snapshot() []peerEntry {
	return s.snapshotTopic("")
}

func (s *peerStore) snapshotTopic(topic string) []peerEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneLocked(time.Now().UTC())
	out := make([]peerEntry, 0, len(s.peers))
	for _, p := range s.peers {
		if topic != "" && p.Topic != topic {
			continue
		}
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastSeen.After(out[j].LastSeen)
	})
	return out
}

func clientIP(r *http.Request) string {
	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	cf := strings.TrimSpace(r.Header.Get("CF-Connecting-IP"))
	if cf != "" {
		return cf
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func main() {
	addr := os.Getenv("RELAY_LISTEN")
	if addr == "" {
		addr = ":8080"
	}
	ttlSeconds := 120
	if v := strings.TrimSpace(os.Getenv("RELAY_TTL_SEC")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			ttlSeconds = n
		}
	}
	maxPeers := 10000
	if v := strings.TrimSpace(os.Getenv("RELAY_MAX_PEERS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxPeers = n
		}
	}

	store := newPeerStore(time.Duration(ttlSeconds)*time.Second, maxPeers)
	mux := http.NewServeMux()

	staticDir := strings.TrimSpace(os.Getenv("RELAY_STATIC_DIR"))
	if staticDir == "" {
		staticDir = "./src/plugins/swarm/src_v3/relay_web/static"
	}
	if _, err := os.Stat(staticDir); err != nil {
		staticDir = "./static"
	}
	fileServer := http.FileServer(http.Dir(staticDir))
	mux.Handle("/", fileServer)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/api/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload struct {
			Topic string `json:"topic"`
			Who   string `json:"who"`
			Port  int    `json:"port"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)

		ip := clientIP(r)
		if payload.Topic == "" {
			payload.Topic = "default"
		}
		store.touch(ip, payload.Port, strings.TrimSpace(payload.Topic), strings.TrimSpace(payload.Who), r.UserAgent())

		topicPeers := store.snapshotTopic(payload.Topic)
		others := make([]peerEntry, 0, len(topicPeers))
		for _, p := range topicPeers {
			if p.IP == ip && p.Port == payload.Port {
				continue
			}
			others = append(others, p)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    true,
			"ip":    ip,
			"topic": payload.Topic,
			"self": map[string]any{
				"ip":   ip,
				"port": payload.Port,
				"who":  payload.Who,
			},
			"peers": others,
		})
	})

	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			Who string `json:"who"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		ip := clientIP(r)
		store.touch(ip, 0, "default", strings.TrimSpace(payload.Who), r.UserAgent())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "ip": ip, "who": payload.Who})
	})

	mux.HandleFunc("/api/peers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		topic := strings.TrimSpace(r.URL.Query().Get("topic"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"now":   time.Now().UTC(),
			"topic": topic,
			"peers": store.snapshotTopic(topic),
		})
	})

	log.Printf("relay tracker listening on %s (ttl=%ds maxPeers=%d static=%s)", addr, ttlSeconds, maxPeers, staticDir)
	log.Fatal(http.ListenAndServe(addr, mux))
}

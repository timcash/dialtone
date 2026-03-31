package tap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

type busFrame struct {
	Type       string   `json:"type"`
	Scope      string   `json:"scope,omitempty"`
	Kind       string   `json:"kind,omitempty"`
	From       string   `json:"from,omitempty"`
	Target     string   `json:"target,omitempty"`
	Room       string   `json:"room,omitempty"`
	Version    string   `json:"version,omitempty"`
	OS         string   `json:"os,omitempty"`
	Arch       string   `json:"arch,omitempty"`
	ReplVer    string   `json:"repl_version,omitempty"`
	DaemonVer  string   `json:"daemon_version,omitempty"`
	Command    string   `json:"command,omitempty"`
	Args       []string `json:"args,omitempty"`
	Prefix     string   `json:"prefix,omitempty"`
	Message    string   `json:"message,omitempty"`
	PID        int      `json:"pid,omitempty"`
	LogPath    string   `json:"log_path,omitempty"`
	ExitCode   int      `json:"exit_code,omitempty"`
	Ready      bool     `json:"ready,omitempty"`
	ServerID   string   `json:"server_id,omitempty"`
	Timestamp  string   `json:"timestamp"`
}

type logRecord struct {
	Subject   string `json:"subject"`
	Level     string `json:"level"`
	Kind      string `json:"kind,omitempty"`
	Message   string `json:"message"`
	Source    string `json:"source,omitempty"`
	ElapsedS  int    `json:"elapsed_s,omitempty"`
	Timestamp string `json:"timestamp"`
}

func Run(upstream, subjectsCSV, name string, reconnectWait time.Duration, raw, showSubject, showReconnects bool) error {
	subjects := parseSubjects(subjectsCSV)
	if len(subjects) == 0 {
		return fmt.Errorf("no subjects configured")
	}

	opts := []nats.Option{
		nats.Name(strings.TrimSpace(name)),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(reconnectWait),
		nats.Timeout(1200 * time.Millisecond),
	}
	if showReconnects {
		opts = append(opts,
			nats.ConnectHandler(func(nc *nats.Conn) {
				fmt.Fprintf(os.Stderr, "TAP> connected: %s\n", nc.ConnectedUrl())
			}),
			nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
				if err != nil {
					fmt.Fprintf(os.Stderr, "TAP> disconnected: %v\n", err)
				} else {
					fmt.Fprintln(os.Stderr, "TAP> disconnected")
				}
			}),
			nats.ReconnectHandler(func(nc *nats.Conn) {
				fmt.Fprintf(os.Stderr, "TAP> reconnected: %s\n", nc.ConnectedUrl())
			}),
			nats.DiscoveredServersHandler(func(nc *nats.Conn) {
				if len(nc.DiscoveredServers()) > 0 {
					fmt.Fprintf(os.Stderr, "TAP> discovered servers: %s\n", strings.Join(nc.DiscoveredServers(), ", "))
				}
			}),
		)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go monitorHeartbeats(ctx)

	for {
		if ctx.Err() != nil {
			break
		}

		nc, err := nats.Connect(strings.TrimSpace(upstream), opts...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "TAP> connection failed: %v; retrying in %v\n", err, reconnectWait)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(reconnectWait):
				continue
			}
		}

		if nc.IsConnected() {
			fmt.Fprintf(os.Stderr, "TAP> connected: %s\n", nc.ConnectedUrl())
		} else {
			fmt.Fprintln(os.Stderr, "TAP> waiting for upstream NATS (subscriptions queued)")
		}

		subscribed := true
		for _, subject := range subjects {
			subj := subject
			_, subErr := nc.Subscribe(subj, func(msg *nats.Msg) {
				printMessage(msg, raw, showSubject)
			})
			if subErr != nil {
				fmt.Fprintf(os.Stderr, "TAP> subscribe failed for %q: %v\n", subj, subErr)
				subscribed = false
				break
			}
		}

		if !subscribed {
			nc.Close()
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(reconnectWait):
				continue
			}
		}

		if err := nc.FlushTimeout(1200 * time.Millisecond); err != nil {
			fmt.Fprintf(os.Stderr, "TAP> warning: initial nats flush pending (%v)\n", err)
		}

		// Wait for connection to close or signal
		closed := make(chan struct{})
		nc.SetClosedHandler(func(_ *nats.Conn) {
			fmt.Fprintln(os.Stderr, "TAP> connection closed; re-establishing...")
			close(closed)
		})

		select {
		case <-ctx.Done():
			nc.Close()
			return nil
		case <-closed:
			nc.Close()
			// continue loop to reconnect
		}
	}

	return nil
}

func parseSubjects(raw string) []string {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func printMessage(msg *nats.Msg, raw bool, showSubject bool) {
	subject := strings.TrimSpace(msg.Subject)
	if raw {
		line := strings.TrimSpace(string(msg.Data))
		if showSubject {
			fmt.Printf("[%s] %s\n", subject, line)
		} else {
			fmt.Println(line)
		}
		return
	}

	// Try BusFrame first (REPL protocol)
	var frame busFrame
	if err := json.Unmarshal(msg.Data, &frame); err == nil && frame.Type != "" {
		printFrame(subject, frame, showSubject)
		return
	}

	// Try LogRecord (Direct logging protocol)
	var rec logRecord
	if err := json.Unmarshal(msg.Data, &rec); err == nil && rec.Level != "" {
		printLogRecord(subject, rec, showSubject)
		return
	}

	line := strings.TrimSpace(string(msg.Data))
	if showSubject {
		fmt.Printf("[%s] %s\n", subject, line)
	} else {
		fmt.Println(line)
	}
}

func printFrame(subject string, f busFrame, showSubject bool) {
	prefix := ""
	if showSubject {
		prefix = "[" + subject + "] "
	}

	// Determine UI branding based on Scope and Kind
	brand := "DIALTONE"
	if f.Scope == "task-worker" && f.PID > 0 {
		brand = fmt.Sprintf("DIALTONE:%d", f.PID)
	} else if f.Prefix != "" {
		brand = f.Prefix
	}

	switch strings.TrimSpace(f.Type) {
	case "input":
		user := firstNonEmpty(f.From, "unknown")
		fmt.Printf("%s%s> %s\n", prefix, user, strings.TrimSpace(f.Message))
	case "chat":
		user := firstNonEmpty(f.From, "chat")
		fmt.Printf("%s%s: %s\n", prefix, user, strings.TrimSpace(f.Message))
	case "join":
		ver := ""
		if f.Version != "" {
			ver = " version=" + f.Version
		}
		fmt.Printf("%sDIALTONE> [JOIN] %s (room=%s%s)\n", prefix, firstNonEmpty(f.From, "unknown"), firstNonEmpty(f.Room, "index"), ver)
	case "left":
		fmt.Printf("%sDIALTONE> [LEFT] %s (room=%s)\n", prefix, firstNonEmpty(f.From, "unknown"), firstNonEmpty(f.Room, "index"))
	case "server":
		fmt.Printf("%sDIALTONE> %s\n", prefix, strings.TrimSpace(f.Message))
	case "error":
		fmt.Printf("%s%s> [ERROR] %s\n", prefix, brand, strings.TrimSpace(f.Message))
	case "heartbeat":
		msg := strings.TrimSpace(f.Message)
		trackHeartbeat(subject)
		if msg != "alive" {
			fmt.Printf("%sDIALTONE> [HEARTBEAT] %s\n", prefix, msg)
		}
	case "command":
		user := firstNonEmpty(f.From, "unknown")
		fmt.Printf("%s%s (CMD)> %s\n", prefix, user, strings.TrimSpace(f.Message))
	case "control":
		msg := strings.TrimSpace(f.Message)
		if msg == "" {
			msg = fmt.Sprintf("%s target=%s room=%s", f.Command, f.Target, f.Room)
		}
		fmt.Printf("%sDIALTONE> [CONTROL] %s\n", prefix, msg)
	case "daemon":
		host := firstNonEmpty(f.From, "unknown")
		fmt.Printf("%sDIALTONE> [DAEMON] %s (room=%s daemon=%s repl=%s)\n", prefix, host, f.Room, f.DaemonVer, f.ReplVer)
	case "probe":
		user := firstNonEmpty(f.From, "unknown")
		fmt.Printf("%sDIALTONE> [PROBE] %s (room=%s msg=%s)\n", prefix, user, f.Room, f.Message)
	case "line":
		// Generic line, use our determined brand
		fmt.Printf("%s%s> %s\n", prefix, brand, strings.TrimSpace(f.Message))
	default:
		if strings.TrimSpace(f.Message) != "" {
			fmt.Printf("%s[%s] %s\n", prefix, firstNonEmpty(f.Type, "frame"), strings.TrimSpace(f.Message))
		} else {
			fmt.Printf("%s[%s] from=%s room=%s target=%s command=%s\n", prefix, firstNonEmpty(f.Type, "frame"), strings.TrimSpace(f.From), strings.TrimSpace(f.Room), strings.TrimSpace(f.Target), strings.TrimSpace(f.Command))
		}
	}
}

func printLogRecord(subject string, r logRecord, showSubject bool) {
	prefix := ""
	if showSubject {
		prefix = "[" + subject + "] "
	}

	brand := "DIALTONE"
	if r.Kind == "status" {
		fmt.Printf("%s%s> %s\n", prefix, brand, strings.TrimSpace(r.Message))
		return
	}

	// For standard log records, we can still show the level/source if it's not a status message
	fmt.Printf("%s[%s|%s] %s\n", prefix, r.Level, r.Source, strings.TrimSpace(r.Message))
}

var (
	heartbeatsMu   sync.Mutex
	lastHeartbeats = make(map[string]time.Time)
	missingAlerted = make(map[string]bool)
)

func trackHeartbeat(subject string) {
	heartbeatsMu.Lock()
	defer heartbeatsMu.Unlock()
	lastHeartbeats[subject] = time.Now()
	if missingAlerted[subject] {
		fmt.Printf("[%s] DIALTONE> [HEARTBEAT RESTORED]\n", subject)
		missingAlerted[subject] = false
	}
}

func monitorHeartbeats(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			heartbeatsMu.Lock()
			now := time.Now()
			for subj, last := range lastHeartbeats {
				if !missingAlerted[subj] && now.Sub(last) > 30*time.Second {
					fmt.Printf("[%s] DIALTONE> [HEARTBEAT MISSING] (last seen %v ago)\n", subj, now.Sub(last).Truncate(time.Second))
					missingAlerted[subj] = true
				}
			}
			heartbeatsMu.Unlock()
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

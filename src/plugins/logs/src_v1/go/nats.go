package logs

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	nserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

type Record struct {
	Subject   string `json:"subject"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Source    string `json:"source,omitempty"`
	ElapsedS  int    `json:"elapsed_s,omitempty"`
	Timestamp string `json:"timestamp"`
}

type EmbeddedNATS struct {
	server *nserver.Server
	conn   *nats.Conn
}

func StartEmbeddedNATS() (*EmbeddedNATS, error) {
	opts := &nserver.Options{
		Host:   "127.0.0.1",
		Port:   -1,
		NoLog:  true,
		NoSigs: true,
	}
	srv, err := nserver.NewServer(opts)
	if err != nil {
		return nil, err
	}
	go srv.Start()
	if !srv.ReadyForConnections(5 * time.Second) {
		return nil, fmt.Errorf("embedded nats server did not become ready")
	}

	nc, err := nats.Connect(srv.ClientURL())
	if err != nil {
		srv.Shutdown()
		return nil, err
	}
	return &EmbeddedNATS{server: srv, conn: nc}, nil
}

func StartEmbeddedNATSOnURL(natsURL string) (*EmbeddedNATS, error) {
	u, err := url.Parse(strings.TrimSpace(natsURL))
	if err != nil {
		return nil, fmt.Errorf("invalid nats url %q: %w", natsURL, err)
	}
	host := u.Hostname()
	if host == "" {
		host = "127.0.0.1"
	}
	port := 4222
	if p := u.Port(); p != "" {
		parsed, perr := strconv.Atoi(p)
		if perr != nil {
			return nil, fmt.Errorf("invalid nats port %q: %w", p, perr)
		}
		port = parsed
	}

	opts := &nserver.Options{
		Host:   host,
		Port:   port,
		NoLog:  true,
		NoSigs: true,
	}
	srv, err := nserver.NewServer(opts)
	if err != nil {
		return nil, err
	}
	go srv.Start()
	if !srv.ReadyForConnections(5 * time.Second) {
		return nil, fmt.Errorf("embedded nats server did not become ready")
	}
	nc, err := nats.Connect(srv.ClientURL())
	if err != nil {
		srv.Shutdown()
		return nil, err
	}
	return &EmbeddedNATS{server: srv, conn: nc}, nil
}

func (e *EmbeddedNATS) URL() string {
	if e == nil || e.server == nil {
		return ""
	}
	return e.server.ClientURL()
}

func (e *EmbeddedNATS) Conn() *nats.Conn {
	if e == nil {
		return nil
	}
	return e.conn
}

func (e *EmbeddedNATS) Close() {
	if e == nil {
		return
	}
	if e.conn != nil {
		_ = e.conn.Drain()
		e.conn.Close()
	}
	if e.server != nil {
		e.server.Shutdown()
	}
}

type NATSLogger struct {
	conn    *nats.Conn
	subject string
}

func NewNATSLogger(conn *nats.Conn, subject string) (*NATSLogger, error) {
	if conn == nil {
		return nil, fmt.Errorf("nil nats connection")
	}
	if subject == "" {
		return nil, fmt.Errorf("subject is required")
	}
	return &NATSLogger{conn: conn, subject: subject}, nil
}

func (l *NATSLogger) Subject() string { return l.subject }

func (l *NATSLogger) Conn() *nats.Conn { return l.conn }

func (l *NATSLogger) Infof(format string, args ...any) error {
	return l.publishWithSource("INFO", fmt.Sprintf(format, args...), "", false)
}

func (l *NATSLogger) Warnf(format string, args ...any) error {
	return l.publishWithSource("WARN", fmt.Sprintf(format, args...), "", false)
}

func (l *NATSLogger) Errorf(format string, args ...any) error {
	return l.publishWithSource("ERROR", fmt.Sprintf(format, args...), "", false)
}

func (l *NATSLogger) publish(level, message string) error {
	return l.publishWithSource(level, message, "", false)
}

func (l *NATSLogger) InfofFrom(source, format string, args ...any) error {
	return l.publishWithSource("INFO", fmt.Sprintf(format, args...), source, false)
}

func (l *NATSLogger) WarnfFrom(source, format string, args ...any) error {
	return l.publishWithSource("WARN", fmt.Sprintf(format, args...), source, false)
}

func (l *NATSLogger) ErrorfFrom(source, format string, args ...any) error {
	return l.publishWithSource("ERROR", fmt.Sprintf(format, args...), source, false)
}

func (l *NATSLogger) InfofFromTest(source, format string, args ...any) error {
	return l.publishWithSource("INFO", fmt.Sprintf(format, args...), source, true)
}

func (l *NATSLogger) WarnfFromTest(source, format string, args ...any) error {
	return l.publishWithSource("WARN", fmt.Sprintf(format, args...), source, true)
}

func (l *NATSLogger) ErrorfFromTest(source, format string, args ...any) error {
	return l.publishWithSource("ERROR", fmt.Sprintf(format, args...), source, true)
}

func (l *NATSLogger) publishWithSource(level, message, source string, isTest bool) error {
	level = strings.ToUpper(strings.TrimSpace(level))
	subject := strings.TrimSpace(l.subject)
	src := strings.TrimSpace(source)
	if src == "" {
		src = callerSourceLocation()
	}
	if isTest {
		message = TestPrefix(message)
	}
	elapsed := elapsedSeconds(subject)
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	targets := buildFanoutSubjects(subject, level, message)
	for _, target := range targets {
		rec := Record{
			Subject:   target,
			Level:     level,
			Message:   message,
			Source:    src,
			ElapsedS:  elapsed,
			Timestamp: ts,
		}
		data, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		if err := l.conn.Publish(target, data); err != nil {
			return err
		}
	}
	return nil
}

func buildFanoutSubjects(subject, level, message string) []string {
	subject = strings.TrimSpace(subject)
	level = strings.ToLower(strings.TrimSpace(level))
	if level == "" {
		level = "info"
	}
	plugin := pluginToken(subject)
	seen := map[string]bool{}
	out := make([]string, 0, 8)
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}

	add(subject)
	add("logfilter.level." + level)
	if plugin != "" {
		add("logfilter.level." + level + "." + plugin)
	}

	tags := extractBracketTags(message)
	for _, tag := range tags {
		add("logfilter.tag." + tag)
		if plugin != "" {
			add("logfilter.tag." + tag + "." + plugin)
		}
		add("logfilter.level." + level + ".tag." + tag)
		if plugin != "" {
			add("logfilter.level." + level + "." + plugin + ".tag." + tag)
		}
	}
	return out
}

func pluginToken(subject string) string {
	parts := strings.Split(strings.TrimSpace(subject), ".")
	if len(parts) < 2 {
		return ""
	}
	if parts[0] != "logs" {
		return ""
	}
	if parts[1] == "test" && len(parts) >= 3 {
		suite := sanitizeSubjectFragment(parts[2])
		if suite == "" {
			return "test"
		}
		if idx := strings.Index(suite, "-"); idx > 0 {
			return suite[:idx]
		}
		return suite
	}
	return sanitizeSubjectFragment(parts[1])
}

func extractBracketTags(message string) []string {
	rest := strings.TrimSpace(message)
	out := []string{}
	seen := map[string]bool{}
	for strings.HasPrefix(rest, "[") {
		end := strings.Index(rest, "]")
		if end <= 1 {
			break
		}
		raw := strings.TrimSpace(rest[1:end])
		rest = strings.TrimSpace(rest[end+1:])
		if raw == "" {
			continue
		}
		tag := sanitizeSubjectFragment(strings.ReplaceAll(strings.ToLower(raw), " ", "-"))
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	return out
}

func sanitizeSubjectFragment(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	repl := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", "|", "-", ":", "-", ".", "-", "_", "-", "(", "", ")", "", "'", "", "\"", "")
	s = repl.Replace(s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	return s
}

func ListenToFile(conn *nats.Conn, subject, filePath string) (func() error, error) {
	if conn == nil {
		return nil, fmt.Errorf("nil nats connection")
	}
	if subject == "" {
		return nil, fmt.Errorf("subject is required")
	}
	if filePath == "" {
		return nil, fmt.Errorf("file path is required")
	}
	var mu sync.Mutex

	sub, err := conn.Subscribe(subject, func(msg *nats.Msg) {
		line := formatMessage(msg.Subject, msg.Data)
		mu.Lock()
		defer mu.Unlock()
		f, ferr := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if ferr != nil {
			return
		}
		_, _ = f.WriteString(line + "\n")
		_ = f.Close()
	})
	if err != nil {
		return nil, err
	}
	if err := conn.Flush(); err != nil {
		_ = sub.Unsubscribe()
		return nil, err
	}

	stop := func() error {
		mu.Lock()
		defer mu.Unlock()
		return sub.Unsubscribe()
	}
	return stop, nil
}

func FormatMessage(subject string, payload []byte) string {
	var rec Record
	if err := json.Unmarshal(payload, &rec); err == nil {
		level := strings.ToUpper(strings.TrimSpace(rec.Level))
		if level == "" {
			level = "INFO"
		}
		src := strings.TrimSpace(rec.Source)
		if src == "" {
			src = "unknown"
		}
		elapsed := rec.ElapsedS
		if elapsed < 0 {
			elapsed = 0
		}
		msg := strings.TrimSpace(rec.Message)
		if msg == "" {
			msg = string(payload)
		}
		return fmt.Sprintf("[T+%04ds|%s|%s] %s", elapsed, level, src, msg)
	}
	return fmt.Sprintf("[T+%04ds|INFO|unknown] subject=%s message=%s", elapsedSeconds(subject), subject, string(payload))
}

func formatMessage(subject string, payload []byte) string {
	return FormatMessage(subject, payload)
}

func ResetTopicClock(subject string) {
	ResetClock(subject)
}

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
	return l.publish("INFO", fmt.Sprintf(format, args...))
}

func (l *NATSLogger) Warnf(format string, args ...any) error {
	return l.publish("WARN", fmt.Sprintf(format, args...))
}

func (l *NATSLogger) Errorf(format string, args ...any) error {
	return l.publish("ERROR", fmt.Sprintf(format, args...))
}

func (l *NATSLogger) publish(level, message string) error {
	level = strings.ToUpper(strings.TrimSpace(level))
	subject := strings.TrimSpace(l.subject)
	src := callerSourceFile()
	elapsed := elapsedSeconds(subject)
	rec := Record{
		Subject:   subject,
		Level:     level,
		Message:   message,
		Source:    src,
		ElapsedS:  elapsed,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return l.conn.Publish(l.subject, data)
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

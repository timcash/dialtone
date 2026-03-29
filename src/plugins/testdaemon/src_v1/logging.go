package testdaemon

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

type commandSession struct {
	command   string
	service   string
	host      string
	pid       int
	startedAt time.Time
	logPath   string
	logFile   *os.File
}

func newCommandSession(command string, service string) (*commandSession, error) {
	logsDir := filepath.Join(configv1.DefaultDialtoneHome(), "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, err
	}

	name := sanitizeName(service)
	if name == "" {
		name = "command"
	}
	fileName := fmt.Sprintf(
		"testdaemon-%s-%s-%s.log",
		sanitizeName(command),
		name,
		time.Now().UTC().Format("20060102-150405.000000000"),
	)
	logPath := filepath.Join(logsDir, fileName)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}

	return &commandSession{
		command:   strings.TrimSpace(command),
		service:   strings.TrimSpace(service),
		host:      currentHostName(),
		pid:       os.Getpid(),
		startedAt: time.Now().UTC(),
		logPath:   logPath,
		logFile:   logFile,
	}, nil
}

func (s *commandSession) ActivateMirror() {
	if s == nil || s.logFile == nil {
		logs.SetOutput(os.Stdout)
		return
	}
	logs.SetOutput(io.MultiWriter(os.Stdout, s.logFile))
}

func (s *commandSession) Close() {
	logs.SetOutput(os.Stdout)
	if s == nil || s.logFile == nil {
		return
	}
	_ = s.logFile.Close()
}

func (s *commandSession) PrintIdentity() {
	if s == nil {
		return
	}
	logs.Raw("testdaemon> command=%s", s.command)
	if strings.TrimSpace(s.service) != "" {
		logs.Raw("testdaemon> service=%s", strings.TrimSpace(s.service))
	}
	logs.Raw("testdaemon> host=%s", s.host)
	logs.Raw("testdaemon> pid=%d", s.pid)
	logs.Raw("testdaemon> started_at=%s", s.startedAt.Format(time.RFC3339))
	logs.Raw("testdaemon> log_path=%s", s.logPath)
}

func withCommandSession(command string, service string, fn func(*commandSession) error) error {
	session, err := newCommandSession(command, service)
	if err != nil {
		return err
	}
	defer session.Close()
	session.ActivateMirror()
	session.PrintIdentity()
	return fn(session)
}

func currentHostName() string {
	host, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return "unknown"
	}
	return host
}

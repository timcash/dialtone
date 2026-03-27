package repl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type taskLogWriter struct {
	TaskID    string
	LogPath   string
	StartedAt time.Time
	mu        sync.Mutex
	file      *os.File
}

func newTaskLogWriter(taskID string, args []string) (*taskLogWriter, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task id is required")
	}
	logsDir, err := resolveTaskLogsDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, err
	}
	startedAt := time.Now().UTC()
	path := filepath.Join(logsDir, fmt.Sprintf("%s.log", taskID))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	w := &taskLogWriter{
		TaskID:    taskID,
		LogPath:   path,
		StartedAt: startedAt,
		file:      f,
	}
	w.writef("queued task_id=%s args=%q", taskID, args)
	return w, nil
}

func (w *taskLogWriter) Close() {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
}

func (w *taskLogWriter) LogLifecycle(format string, args ...any) {
	if w == nil {
		return
	}
	w.writef("lifecycle "+format, args...)
}

func (w *taskLogWriter) LogStatus(format string, args ...any) {
	if w == nil {
		return
	}
	w.writef("status "+format, args...)
}

func (w *taskLogWriter) LogLine(line string) {
	if w == nil {
		return
	}
	w.writef("stdout %s", strings.TrimSpace(line))
}

func (w *taskLogWriter) LogError(line string) {
	if w == nil {
		return
	}
	w.writef("stderr %s", strings.TrimSpace(line))
}

func (w *taskLogWriter) writef(format string, args ...any) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return
	}
	_, _ = fmt.Fprintf(w.file, "%s %s\n", time.Now().UTC().Format(time.RFC3339), fmt.Sprintf(format, args...))
}

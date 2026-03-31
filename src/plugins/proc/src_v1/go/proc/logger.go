package proc

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

type TaskWorkerLogger struct {
	PID        int
	LogPath    string
	StartTime  time.Time
	ErrorCount int
	ErrorLimit int
	mu         sync.Mutex
	done       chan struct{}
	file       *os.File
}

func NewTaskWorkerLogger(pid int, args []string, logDir string) (*TaskWorkerLogger, error) {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}
	startedAt := time.Now()
	name := fmt.Sprintf("task-worker-%d-%s.log", pid, startedAt.Format("20060102-150405"))
	path := filepath.Join(logDir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	logger := &TaskWorkerLogger{
		PID:        pid,
		LogPath:    path,
		StartTime:  startedAt,
		ErrorLimit: 2,
		done:       make(chan struct{}),
		file:       f,
	}
	logger.writef("started pid=%d args=%q", pid, args)
	return logger, nil
}

func (l *TaskWorkerLogger) StartHeartbeat(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				l.logHeartbeat()
			case <-l.done:
				return
			}
		}
	}()
}

func (l *TaskWorkerLogger) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	select {
	case <-l.done:
	default:
		close(l.done)
	}
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
}

func (l *TaskWorkerLogger) LogLine(line string) {
	l.writef("stdout %s", line)
}

func (l *TaskWorkerLogger) LogError(line string) {
	l.writef("stderr %s", line)

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.ErrorCount < l.ErrorLimit {
		l.ErrorCount++
	}
}

func (l *TaskWorkerLogger) logHeartbeat() {
	p, err := process.NewProcess(int32(l.PID))
	if err != nil {
		return
	}

	cpu, _ := p.CPUPercent()
	mem, _ := p.MemoryInfo()

	// Network connections
	conns, _ := net.ConnectionsPid("tcp", int32(l.PID))
	ports := len(conns)

	memUsage := uint64(0)
	if mem != nil {
		memUsage = mem.RSS
	}

	_ = cpu
	_ = memUsage
	_ = ports
	l.writef("heartbeat cpu=%.1f mem_rss=%d ports=%d", cpu, memUsage, ports)
}

func (l *TaskWorkerLogger) writef(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}
	_, _ = fmt.Fprintf(l.file, "%s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
}

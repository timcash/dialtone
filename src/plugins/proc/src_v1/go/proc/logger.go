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

type SubtoneLogger struct {
	PID        int
	LogPath    string
	StartTime  time.Time
	ErrorCount int
	ErrorLimit int
	mu         sync.Mutex
	done       chan struct{}
}

func NewSubtoneLogger(pid int, args []string, logDir string) (*SubtoneLogger, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	timestamp := time.Now().Format("20060102-150405")
	logName := fmt.Sprintf("subtone-%d-%s.log", pid, timestamp)
	logPath := filepath.Join(logDir, logName)

	// Create/truncate log file
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(f, "Command: %v\n", args)
	fmt.Fprintf(f, "Started at: %s\n", time.Now().Format(time.RFC3339))
	f.Close()

	logger := &SubtoneLogger{
		PID:        pid,
		LogPath:    logPath,
		StartTime:  time.Now(),
		ErrorLimit: 2,
		done:       make(chan struct{}),
	}

	return logger, nil
}

func (l *SubtoneLogger) StartHeartbeat(interval time.Duration) {
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

func (l *SubtoneLogger) Stop() {
	close(l.done)
}

func (l *SubtoneLogger) LogLine(line string) {
	// Append to file
	f, err := os.OpenFile(l.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		ts := time.Now().Format(time.RFC3339)
		fmt.Fprintf(f, "[%s] %s\n", ts, line)
		f.Close()
	}
}

func (l *SubtoneLogger) LogError(line string) {
	l.LogLine(line) // Log to file

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.ErrorCount < l.ErrorLimit {
		l.ErrorCount++
	}
}

func (l *SubtoneLogger) logHeartbeat() {
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
}

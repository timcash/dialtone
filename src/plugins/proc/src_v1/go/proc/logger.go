package proc

import (
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
	logger := &SubtoneLogger{
		PID:        pid,
		LogPath:    "",
		StartTime:  time.Now(),
		ErrorLimit: 2,
		done:       make(chan struct{}),
	}
	_ = args
	_ = logDir
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
	_ = line
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

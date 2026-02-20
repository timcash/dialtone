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

	// 1. Log start info to REPL
	fmt.Printf("DIALTONE:%d> Started at %s\n", pid, logger.StartTime.Format(time.RFC3339))
	fmt.Printf("DIALTONE:%d> Command: %v\n", pid, args)
	fmt.Printf("DIALTONE:%d> Log: %s\n", pid, logPath)

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
	// 4. Log cleanup
	fmt.Printf("DIALTONE:%d> Cleaning up...\n", l.PID)
}

func (l *SubtoneLogger) LogLine(line string) {
	// Append to file
	f, err := os.OpenFile(l.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		ts := time.Now().Format(time.RFC3339)
		fmt.Fprintf(f, "[%s] %s\n", ts, line)
		f.Close()
	}

	// 3. Check for errors and throttle output to REPL
	// Simple heuristic: lines starting with "Error" or containing "panic"
	// Or maybe the caller tells us if it's stderr?
	// For now, assume stderr lines are passed here? 
	// No, dev.go passes all output here.
	// We'll treat all lines as info unless they look like errors.
}

func (l *SubtoneLogger) LogError(line string) {
	l.LogLine(line) // Log to file

	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.ErrorCount < l.ErrorLimit {
		fmt.Printf("DIALTONE:%d> ERROR: %s\n", l.PID, line)
		l.ErrorCount++
		if l.ErrorCount == l.ErrorLimit {
			fmt.Printf("DIALTONE:%d> (Suppressed further errors)\n", l.PID)
		}
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

	fmt.Printf("DIALTONE:%d> [Heartbeat] CPU: %.1f%% | Mem: %s | Ports: %d\n", 
		l.PID, cpu, formatBytes(memUsage), ports)
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

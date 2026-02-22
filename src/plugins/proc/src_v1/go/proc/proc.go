package proc

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

type Process struct {
	PID       int
	Args      []string
	StartTime time.Time
}

type ManagedProcessSnapshot struct {
	PID        int
	Command    string
	StartedAgo time.Duration
	CPUPercent float64
	MemRSS     uint64
	PortCount  int
}

var (
	processRegistry = struct {
		sync.Mutex
		procs map[int]*Process
	}{
		procs: make(map[int]*Process),
	}
)

func TrackProcess(pid int, args []string) {
	processRegistry.Lock()
	defer processRegistry.Unlock()
	processRegistry.procs[pid] = &Process{
		PID:       pid,
		Args:      append([]string(nil), args...),
		StartTime: time.Now(),
	}
}

func UntrackProcess(pid int) {
	processRegistry.Lock()
	defer processRegistry.Unlock()
	delete(processRegistry.procs, pid)
}

func ListManagedProcesses() []ManagedProcessSnapshot {
	processRegistry.Lock()
	procs := make([]*Process, 0, len(processRegistry.procs))
	for _, p := range processRegistry.procs {
		procs = append(procs, p)
	}
	processRegistry.Unlock()

	out := make([]ManagedProcessSnapshot, 0, len(procs))
	for _, p := range procs {
		s := ManagedProcessSnapshot{
			PID:        p.PID,
			Command:    strings.Join(p.Args, " "),
			StartedAgo: time.Since(p.StartTime).Round(time.Second),
		}
		if s.StartedAgo < 0 {
			s.StartedAgo = 0
		}

		if gp, err := process.NewProcess(int32(p.PID)); err == nil {
			if cpu, err := gp.CPUPercent(); err == nil {
				s.CPUPercent = cpu
			}
			if mem, err := gp.MemoryInfo(); err == nil && mem != nil {
				s.MemRSS = mem.RSS
			}
			if conns, err := net.ConnectionsPid("tcp", int32(p.PID)); err == nil {
				s.PortCount = len(conns)
			}
		}
		out = append(out, s)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].PID < out[j].PID })
	return out
}

func KillManagedProcess(pid int) error {
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		return err
	}
	if err := p.Kill(); err != nil {
		return err
	}
	UntrackProcess(pid)
	return nil
}

func ListProcesses() {
	snapshots := ListManagedProcesses()
	if len(snapshots) == 0 {
		fmt.Println("No active managed processes.")
		return
	}

	fmt.Println("Active managed processes:")
	fmt.Printf("%-8s %-10s %-12s %-8s %s\n", "PID", "CPU%", "MEM", "PORTS", "COMMAND")
	for _, p := range snapshots {
		fmt.Printf("%-8d %-10.1f %-12s %-8d %s\n", p.PID, p.CPUPercent, formatBytes(p.MemRSS), p.PortCount, p.Command)
	}
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

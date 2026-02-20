package proc

import (
	"fmt"
	"sync"
	"time"
)

type Process struct {
	PID       int
	Command   string
	StartTime time.Time
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
		Command:   fmt.Sprintf("%v", args),
		StartTime: time.Now(),
	}
}

func UntrackProcess(pid int) {
	processRegistry.Lock()
	defer processRegistry.Unlock()
	delete(processRegistry.procs, pid)
}

func ListProcesses() {
	processRegistry.Lock()
	defer processRegistry.Unlock()

	if len(processRegistry.procs) == 0 {
		fmt.Println("DIALTONE> No active subtones.")
		return
	}

	fmt.Println("DIALTONE> Active Subtones:")
	fmt.Printf("%-8s %-20s %s\n", "PID", "STARTED", "COMMAND")
	for _, p := range processRegistry.procs {
		duration := time.Since(p.StartTime).Round(time.Second)
		fmt.Printf("%-8d %-20s %s\n", p.PID, duration, p.Command)
	}
}

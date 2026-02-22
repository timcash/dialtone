package proc

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type SubtoneEventType string

const (
	SubtoneEventStarted SubtoneEventType = "started"
	SubtoneEventStdout  SubtoneEventType = "stdout"
	SubtoneEventStderr  SubtoneEventType = "stderr"
	SubtoneEventExited  SubtoneEventType = "exited"
)

type SubtoneEvent struct {
	Type      SubtoneEventType
	PID       int
	Args      []string
	LogPath   string
	StartedAt time.Time
	Line      string
	ExitCode  int
}

type SubtoneEventHandler func(SubtoneEvent)

func RunSubtone(args []string) int {
	return RunSubtoneWithEvents(args, nil)
}

func RunSubtoneWithEvents(args []string, onEvent SubtoneEventHandler) int {
	emit := func(ev SubtoneEvent) {
		if onEvent != nil {
			onEvent(ev)
		}
	}

	cwd, _ := os.Getwd()
	repoRoot := cwd
	if filepath.Base(cwd) == "src" {
		repoRoot = filepath.Dir(cwd)
	}
	dialtoneSh := filepath.Join(repoRoot, "dialtone.sh")

	cmd := exec.Command(dialtoneSh, args...)
	cmd.Dir = repoRoot

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		emit(SubtoneEvent{
			Type:     SubtoneEventExited,
			PID:      0,
			Args:     append([]string(nil), args...),
			ExitCode: 1,
			Line:     fmt.Sprintf("failed to start subtone: %v", err),
		})
		return 1
	}

	pid := cmd.Process.Pid
	TrackProcess(pid, args)
	defer UntrackProcess(pid)

	logDir := filepath.Join(repoRoot, ".dialtone", "logs")
	logger, err := NewSubtoneLogger(pid, args, logDir)
	if err == nil {
		logger.StartHeartbeat(3 * time.Second)
		defer logger.Stop()
	}
	logPath := ""
	startedAt := time.Now()
	if logger != nil {
		logPath = logger.LogPath
		startedAt = logger.StartTime
	}
	emit(SubtoneEvent{
		Type:      SubtoneEventStarted,
		PID:       pid,
		Args:      append([]string(nil), args...),
		LogPath:   logPath,
		StartedAt: startedAt,
	})

	// Stream stdout (info) and stderr (errors) separately
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if logger != nil {
				logger.LogLine(line)
			}
			emit(SubtoneEvent{
				Type: SubtoneEventStdout,
				PID:  pid,
				Line: line,
			})
		}
	}()

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if logger != nil {
			logger.LogError(line)
		}
		emit(SubtoneEvent{
			Type: SubtoneEventStderr,
			PID:  pid,
			Line: line,
		})
	}

	err = cmd.Wait()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	emit(SubtoneEvent{
		Type:     SubtoneEventExited,
		PID:      pid,
		Args:     append([]string(nil), args...),
		ExitCode: exitCode,
	})
	return exitCode
}

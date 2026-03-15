package proc

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
	cwd, _ := os.Getwd()
	repoRoot := cwd
	if filepath.Base(cwd) == "src" {
		repoRoot = filepath.Dir(cwd)
	}
	dialtoneSh := filepath.Join(repoRoot, "dialtone.sh")

	internalArgs := append([]string{"--subtone-internal"}, args...)
	cmd := exec.Command(dialtoneSh, internalArgs...)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "DIALTONE_CONTEXT=repl")
	logDir := filepath.Join(repoRoot, ".dialtone", "logs")
	return runCommandWithEvents(cmd, args, logDir, onEvent)
}

func RunHostCommandWithEvents(command string, onEvent SubtoneEventHandler) int {
	command = strings.TrimSpace(command)
	if command == "" {
		if onEvent != nil {
			onEvent(SubtoneEvent{
				Type:     SubtoneEventExited,
				ExitCode: 1,
				Line:     "empty host command",
			})
		}
		return 1
	}
	var cmd *exec.Cmd
	trackArgs := []string{"host", command}
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command)
	} else {
		cmd = exec.Command("sh", "-lc", command)
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		cmd.Dir = home
	}
	logDir := filepath.Join(defaultDialtoneHome(), "logs")
	return runCommandWithEvents(cmd, trackArgs, logDir, onEvent)
}

func runCommandWithEvents(cmd *exec.Cmd, trackArgs []string, logDir string, onEvent SubtoneEventHandler) int {
	emit := func(ev SubtoneEvent) {
		if onEvent != nil {
			onEvent(ev)
		}
	}

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		emit(SubtoneEvent{
			Type:     SubtoneEventExited,
			PID:      0,
			Args:     append([]string(nil), trackArgs...),
			ExitCode: 1,
			Line:     fmt.Sprintf("failed to start subtone: %v", err),
		})
		return 1
	}

	pid := cmd.Process.Pid
	TrackProcess(pid, trackArgs)
	defer UntrackProcess(pid)

	logger, err := NewSubtoneLogger(pid, trackArgs, logDir)
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
		Args:      append([]string(nil), trackArgs...),
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
		Args:     append([]string(nil), trackArgs...),
		ExitCode: exitCode,
	})
	return exitCode
}

func defaultDialtoneHome() string {
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".dialtone")
	}
	return ".dialtone"
}

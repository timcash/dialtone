package proc

import (
	"bufio"
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type TaskWorkerEventType string

const (
	TaskWorkerEventStarted TaskWorkerEventType = "started"
	TaskWorkerEventStdout  TaskWorkerEventType = "stdout"
	TaskWorkerEventStderr  TaskWorkerEventType = "stderr"
	TaskWorkerEventExited  TaskWorkerEventType = "exited"
)

type TaskWorkerEvent struct {
	Type      TaskWorkerEventType
	PID       int
	Args      []string
	LogPath   string
	StartedAt time.Time
	Line      string
	ExitCode  int
}

type TaskWorkerEventHandler func(TaskWorkerEvent)

func RunTaskWorker(args []string) int {
	return RunTaskWorkerWithEvents(args, nil)
}

func RunTaskWorkerWithEvents(args []string, onEvent TaskWorkerEventHandler) int {
	cwd, _ := os.Getwd()
	repoRoot := cwd
	if filepath.Base(cwd) == "src" {
		repoRoot = filepath.Dir(cwd)
	}
	dialtoneSh := filepath.Join(repoRoot, "dialtone.sh")

	cmd := exec.Command(dialtoneSh, args...)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "DIALTONE_CONTEXT=repl")
	logDir := filepath.Join(defaultDialtoneHome(), "logs")
	return runCommandWithEvents(cmd, args, logDir, onEvent)
}

func RunHostCommandWithEvents(command string, onEvent TaskWorkerEventHandler) int {
	command = strings.TrimSpace(command)
	if command == "" {
		if onEvent != nil {
			onEvent(TaskWorkerEvent{
				Type:     TaskWorkerEventExited,
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

func runCommandWithEvents(cmd *exec.Cmd, trackArgs []string, logDir string, onEvent TaskWorkerEventHandler) int {
	emit := func(ev TaskWorkerEvent) {
		if onEvent != nil {
			onEvent(ev)
		}
	}

	configureManagedCommand(cmd)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		emit(TaskWorkerEvent{
			Type:     TaskWorkerEventExited,
			PID:      0,
			Args:     append([]string(nil), trackArgs...),
			ExitCode: 1,
			Line:     fmt.Sprintf("failed to start task worker: %v", err),
		})
		return 1
	}

	pid := cmd.Process.Pid
	TrackProcess(pid, trackArgs)
	defer UntrackProcess(pid)

	logger, err := NewTaskWorkerLogger(pid, trackArgs, logDir)
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
	emit(TaskWorkerEvent{
		Type:      TaskWorkerEventStarted,
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
			emit(TaskWorkerEvent{
				Type: TaskWorkerEventStdout,
				PID:  pid,
				Line: line,
			})
		}
	}()

	reportedExitCode := 0
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if code, ok := parseForwardedExitStatus(line); ok && code > 1 {
			reportedExitCode = code
		}
		if logger != nil {
			logger.LogError(line)
		}
		emit(TaskWorkerEvent{
			Type: TaskWorkerEventStderr,
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
	// `go run` exits 1 for any child failure but prints the real code as
	// `exit status N` on stderr. Preserve that task-visible exit code.
	if exitCode == 1 && reportedExitCode > 1 {
		exitCode = reportedExitCode
	}
	emit(TaskWorkerEvent{
		Type:     TaskWorkerEventExited,
		PID:      pid,
		Args:     append([]string(nil), trackArgs...),
		ExitCode: exitCode,
	})
	return exitCode
}

func defaultDialtoneHome() string {
	return configv1.DefaultDialtoneHome()
}

func parseForwardedExitStatus(line string) (int, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "exit status ") {
		return 0, false
	}
	code, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "exit status ")))
	if err != nil || code <= 0 {
		return 0, false
	}
	return code, true
}

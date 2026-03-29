package testdaemon

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func formatTimestamp(ts time.Time) string {
	return ts.UTC().Format(time.RFC3339Nano)
}

func RunBuild(args []string) error {
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return err
	}
	goBin := strings.TrimSpace(rt.GoBin)
	if goBin == "" {
		goBin = "go"
	}
	cmd := exec.Command(goBin, "build", "./plugins/testdaemon/scaffold", "./plugins/testdaemon/src_v1")
	cmd.Dir = rt.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunFormat(args []string) error {
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return err
	}
	goBin := strings.TrimSpace(rt.GoBin)
	if goBin == "" {
		goBin = "go"
	}
	cmd := exec.Command(goBin, "fmt", "./plugins/testdaemon/...")
	cmd.Dir = rt.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunTest(args []string) error {
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return err
	}
	goBin := strings.TrimSpace(rt.GoBin)
	if goBin == "" {
		goBin = "go"
	}
	cmd := exec.Command(goBin, "test", "./plugins/testdaemon/...")
	cmd.Dir = rt.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunRun(args []string) error {
	fs := flag.NewFlagSet("testdaemon-run", flag.ContinueOnError)
	mode := fs.String("mode", "once", "Run mode")
	name := fs.String("name", "once", "Fixture name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*mode) != "once" {
		return fmt.Errorf("unsupported run mode %q (expected once)", strings.TrimSpace(*mode))
	}
	return withCommandSession("run", *name, func(session *commandSession) error {
		logs.Raw("testdaemon> mode=once")
		logs.Raw("testdaemon> fixture ready name=%s host=%s pid=%d", sanitizeName(*name), session.host, session.pid)
		return nil
	})
}

func RunEmitProgress(args []string) error {
	fs := flag.NewFlagSet("testdaemon-emit-progress", flag.ContinueOnError)
	steps := fs.Int("steps", 5, "Number of progress lines")
	delay := fs.Duration("delay", 50*time.Millisecond, "Delay between progress lines")
	name := fs.String("name", "progress", "Fixture name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *steps <= 0 {
		return fmt.Errorf("steps must be positive")
	}
	return withCommandSession("emit-progress", *name, func(session *commandSession) error {
		for i := 1; i <= *steps; i++ {
			logs.Raw("testdaemon> progress %d/%d host=%s pid=%d", i, *steps, session.host, session.pid)
			time.Sleep(*delay)
		}
		logs.Raw("testdaemon> progress complete")
		return nil
	})
}

func RunSleep(args []string) error {
	fs := flag.NewFlagSet("testdaemon-sleep", flag.ContinueOnError)
	seconds := fs.Int("seconds", 1, "Seconds to sleep")
	name := fs.String("name", "sleep", "Fixture name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *seconds < 0 {
		return fmt.Errorf("seconds must be non-negative")
	}
	return withCommandSession("sleep", *name, func(session *commandSession) error {
		logs.Raw("testdaemon> sleeping seconds=%d", *seconds)
		time.Sleep(time.Duration(*seconds) * time.Second)
		logs.Raw("testdaemon> sleep complete host=%s pid=%d", session.host, session.pid)
		return nil
	})
}

func RunExitCode(args []string) error {
	fs := flag.NewFlagSet("testdaemon-exit-code", flag.ContinueOnError)
	code := fs.Int("code", 1, "Exit code")
	name := fs.String("name", "exit-code", "Fixture name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return withCommandSession("exit-code", *name, func(session *commandSession) error {
		logs.Raw("testdaemon> exiting with code=%d host=%s pid=%d", *code, session.host, session.pid)
		return &ExitStatusError{
			Code:    *code,
			Message: fmt.Sprintf("testdaemon exit-code requested code=%d", *code),
		}
	})
}

func RunPanic(args []string) error {
	return withCommandSession("panic", "panic", func(session *commandSession) error {
		logs.Raw("testdaemon> panic requested host=%s pid=%d", session.host, session.pid)
		panic("testdaemon panic requested")
	})
}

func RunCrash(args []string) error {
	return withCommandSession("crash", "crash", func(session *commandSession) error {
		logs.Raw("testdaemon> crash requested host=%s pid=%d", session.host, session.pid)
		if err := killProcess(session.pid); err != nil {
			return err
		}
		time.Sleep(2 * time.Second)
		return fmt.Errorf("crash command unexpectedly continued after self-kill")
	})
}

func RunHang(args []string) error {
	return withCommandSession("hang", "hang", func(session *commandSession) error {
		logs.Raw("testdaemon> hang requested host=%s pid=%d", session.host, session.pid)
		select {}
	})
}

func RunService(args []string) error {
	fs := flag.NewFlagSet("testdaemon-service", flag.ContinueOnError)
	mode := fs.String("mode", "status", "Service mode: start|status|stop")
	name := fs.String("name", "demo", "Service name")
	heartbeatInterval := fs.Duration("heartbeat-interval", defaultHeartbeatInterval, "Heartbeat interval")
	timeout := fs.Duration("timeout", 10*time.Second, "Wait timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}

	switch strings.TrimSpace(*mode) {
	case "start":
		return runServiceStart(*name, *heartbeatInterval, *timeout)
	case "status":
		return runServiceStatus(*name)
	case "stop":
		return runServiceStop(*name, *timeout)
	default:
		return fmt.Errorf("unsupported service mode %q (expected start|status|stop)", strings.TrimSpace(*mode))
	}
}

func runServiceStart(name string, heartbeatInterval time.Duration, timeout time.Duration) error {
	paths, err := resolveServicePaths(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(paths.controlDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(paths.logPath), 0o755); err != nil {
		return err
	}

	if state, err := loadServiceState(paths); err == nil && state.Running && processAlive(state.PID) && deriveServiceHealth(state) == "healthy" {
		printServiceSummary(state)
		return nil
	}

	_ = os.Remove(paths.pauseHeartbeatPath)
	_ = os.Remove(paths.shutdownRequestPath)

	logFile, err := os.OpenFile(paths.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(
		exe,
		"src_v1",
		"daemon",
		"--name", sanitizeName(name),
		"--heartbeat-interval", heartbeatInterval.String(),
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.Env = append(os.Environ(), "DIALTONE_TESTDAEMON_CHILD=1")
	configureDetachedCommand(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}

	state, err := waitForServiceState(paths, timeout, func(state serviceState) bool {
		return state.Running && state.PID > 0 && processAlive(state.PID)
	})
	if err != nil {
		return err
	}
	printServiceSummary(state)
	return nil
}

func runServiceStatus(name string) error {
	paths, err := resolveServicePaths(name)
	if err != nil {
		return err
	}
	state, err := loadServiceState(paths)
	if err != nil {
		if os.IsNotExist(err) {
			logs.Raw("testdaemon> service=%s", sanitizeName(name))
			logs.Raw("testdaemon> state=missing")
			return nil
		}
		return err
	}
	printServiceSummary(state)
	return nil
}

func runServiceStop(name string, timeout time.Duration) error {
	paths, err := resolveServicePaths(name)
	if err != nil {
		return err
	}
	state, err := loadServiceState(paths)
	if err != nil {
		if os.IsNotExist(err) {
			logs.Raw("testdaemon> service=%s", sanitizeName(name))
			logs.Raw("testdaemon> state=already-stopped")
			return nil
		}
		return err
	}

	if err := writeMarkerFile(paths.shutdownRequestPath); err != nil {
		return err
	}

	_, waitErr := waitForServiceState(paths, timeout, func(state serviceState) bool {
		return !state.Running
	})
	if waitErr == nil {
		finalState, _ := loadServiceState(paths)
		printServiceSummary(finalState)
		return nil
	}

	if processAlive(state.PID) {
		_ = terminateProcess(state.PID)
		time.Sleep(500 * time.Millisecond)
	}
	if processAlive(state.PID) {
		_ = killProcess(state.PID)
	}
	finalState := state
	finalState.Running = false
	finalState.UpdatedAt = formatTimestamp(time.Now())
	finalState.ExitReason = "forced-stop"
	finalState.Health = deriveServiceHealth(finalState)
	if err := writeServiceState(paths, finalState); err != nil {
		return err
	}
	printServiceSummary(finalState)
	return nil
}

func RunHeartbeat(args []string) error {
	fs := flag.NewFlagSet("testdaemon-heartbeat", flag.ContinueOnError)
	name := fs.String("name", "demo", "Service name")
	mode := fs.String("mode", "show", "Heartbeat mode: show|stop|resume")
	timeout := fs.Duration("timeout", 5*time.Second, "Wait timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	paths, err := resolveServicePaths(*name)
	if err != nil {
		return err
	}

	switch strings.TrimSpace(*mode) {
	case "show":
		state, err := loadServiceState(paths)
		if err != nil {
			return err
		}
		printHeartbeatSummary(state)
		return nil
	case "stop":
		if err := writeMarkerFile(paths.pauseHeartbeatPath); err != nil {
			return err
		}
		state, err := waitForServiceState(paths, *timeout, func(state serviceState) bool {
			return state.HeartbeatPaused
		})
		if err != nil {
			return err
		}
		printHeartbeatSummary(state)
		return nil
	case "resume":
		if err := os.Remove(paths.pauseHeartbeatPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		state, err := waitForServiceState(paths, *timeout, func(state serviceState) bool {
			if state.HeartbeatPaused {
				return false
			}
			return deriveServiceHealth(state) == "healthy"
		})
		if err != nil {
			return err
		}
		printHeartbeatSummary(state)
		return nil
	default:
		return fmt.Errorf("unsupported heartbeat mode %q (expected show|stop|resume)", strings.TrimSpace(*mode))
	}
}

func RunShutdown(args []string) error {
	fs := flag.NewFlagSet("testdaemon-shutdown", flag.ContinueOnError)
	name := fs.String("name", "demo", "Service name")
	timeout := fs.Duration("timeout", 10*time.Second, "Wait timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}

	paths, err := resolveServicePaths(*name)
	if err != nil {
		return err
	}
	if err := writeMarkerFile(paths.shutdownRequestPath); err != nil {
		return err
	}
	state, err := waitForServiceState(paths, *timeout, func(state serviceState) bool {
		return !state.Running
	})
	if err != nil {
		return err
	}
	printServiceSummary(state)
	return nil
}

func RunDaemon(args []string) error {
	fs := flag.NewFlagSet("testdaemon-daemon", flag.ContinueOnError)
	name := fs.String("name", "demo", "Service name")
	heartbeatInterval := fs.Duration("heartbeat-interval", defaultHeartbeatInterval, "Heartbeat interval")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return runDaemonLoop(*name, *heartbeatInterval)
}

func runDaemonLoop(name string, heartbeatInterval time.Duration) error {
	paths, err := resolveServicePaths(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(paths.controlDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(paths.logPath), 0o755); err != nil {
		return err
	}

	var logFile *os.File
	if strings.TrimSpace(os.Getenv("DIALTONE_TESTDAEMON_CHILD")) != "1" {
		logFile, err = os.OpenFile(paths.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		defer logFile.Close()
		logs.SetOutput(io.MultiWriter(os.Stdout, logFile))
		defer logs.SetOutput(os.Stdout)
	}

	startedAt := time.Now().UTC()
	state := serviceState{
		Name:              sanitizeName(name),
		Host:              currentHostName(),
		PID:               os.Getpid(),
		StartedAt:         formatTimestamp(startedAt),
		UpdatedAt:         formatTimestamp(startedAt),
		LastHeartbeat:     formatTimestamp(startedAt),
		HeartbeatInterval: heartbeatInterval.String(),
		HeartbeatPaused:   false,
		Running:           true,
		LogPath:           paths.logPath,
	}
	if err := writeServiceState(paths, state); err != nil {
		return err
	}
	logs.Raw("testdaemon> daemon started name=%s host=%s pid=%d started_at=%s", state.Name, state.Host, state.PID, state.StartedAt)

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	poll := time.NewTicker(250 * time.Millisecond)
	defer poll.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, serviceSignals()...)
	defer signal.Stop(sigCh)

	pausedLogged := false
	for {
		select {
		case sig := <-sigCh:
			state.Running = false
			state.UpdatedAt = formatTimestamp(time.Now())
			state.ExitReason = fmt.Sprintf("signal:%s", sig.String())
			if err := writeServiceState(paths, state); err != nil {
				return err
			}
			logs.Raw("testdaemon> daemon stopping signal=%s", sig.String())
			return nil
		case <-poll.C:
			if fileExists(paths.shutdownRequestPath) {
				state.Running = false
				state.UpdatedAt = formatTimestamp(time.Now())
				state.ExitReason = "shutdown-requested"
				if err := writeServiceState(paths, state); err != nil {
					return err
				}
				logs.Raw("testdaemon> shutdown requested name=%s pid=%d", state.Name, state.PID)
				return nil
			}
			if fileExists(paths.pauseHeartbeatPath) {
				if !pausedLogged {
					logs.Raw("testdaemon> heartbeats paused name=%s pid=%d", state.Name, state.PID)
					pausedLogged = true
				}
				state.HeartbeatPaused = true
				state.UpdatedAt = formatTimestamp(time.Now())
				if err := writeServiceState(paths, state); err != nil {
					return err
				}
				continue
			}
			if pausedLogged {
				logs.Raw("testdaemon> heartbeats resumed name=%s pid=%d", state.Name, state.PID)
				pausedLogged = false
				state.HeartbeatPaused = false
				state.UpdatedAt = formatTimestamp(time.Now())
				state.LastHeartbeat = formatTimestamp(time.Now())
				if err := writeServiceState(paths, state); err != nil {
					return err
				}
			}
		case <-ticker.C:
			if fileExists(paths.pauseHeartbeatPath) {
				state.HeartbeatPaused = true
				state.UpdatedAt = formatTimestamp(time.Now())
				if err := writeServiceState(paths, state); err != nil {
					return err
				}
				continue
			}
			now := time.Now().UTC()
			state.HeartbeatPaused = false
			state.Running = true
			state.UpdatedAt = formatTimestamp(now)
			state.LastHeartbeat = formatTimestamp(now)
			if err := writeServiceState(paths, state); err != nil {
				return err
			}
			logs.Raw("testdaemon> heartbeat name=%s host=%s pid=%d at=%s", state.Name, state.Host, state.PID, state.LastHeartbeat)
		}
	}
}

func printServiceSummary(state serviceState) {
	state = normalizeServiceState(state)
	logs.Raw("testdaemon> service=%s", state.Name)
	logs.Raw("testdaemon> host=%s", state.Host)
	logs.Raw("testdaemon> pid=%d", state.PID)
	logs.Raw("testdaemon> started_at=%s", state.StartedAt)
	logs.Raw("testdaemon> updated_at=%s", state.UpdatedAt)
	logs.Raw("testdaemon> last_heartbeat=%s", state.LastHeartbeat)
	logs.Raw("testdaemon> heartbeat_paused=%t", state.HeartbeatPaused)
	logs.Raw("testdaemon> running=%t", state.Running)
	logs.Raw("testdaemon> health=%s", deriveServiceHealth(state))
	if strings.TrimSpace(state.ExitReason) != "" {
		logs.Raw("testdaemon> exit_reason=%s", strings.TrimSpace(state.ExitReason))
	}
	logs.Raw("testdaemon> log_path=%s", state.LogPath)
}

func printHeartbeatSummary(state serviceState) {
	state = normalizeServiceState(state)
	logs.Raw("testdaemon> service=%s", state.Name)
	logs.Raw("testdaemon> pid=%d", state.PID)
	logs.Raw("testdaemon> last_heartbeat=%s", state.LastHeartbeat)
	logs.Raw("testdaemon> heartbeat_paused=%t", state.HeartbeatPaused)
	logs.Raw("testdaemon> health=%s", deriveServiceHealth(state))
}

func sanitizeName(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-_")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	return out
}

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

func runStart(args []string) error {
	opts, err := parseServerOptions(args)
	if err != nil {
		return err
	}

	existingPIDs, err := findExistingChromeV1ServicePIDs()
	if err != nil {
		return err
	}
	if len(existingPIDs) > 0 {
		return fmt.Errorf("refusing to start: existing chrome v1 service process(es) detected: %v", existingPIDs)
	}

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	pidPath := chromeServicePIDPath(repoRoot)
	if running, pid := serviceRunning(pidPath); running {
		return fmt.Errorf("chrome service already running (pid=%d)", pid)
	}

	if !canListen(opts.host, opts.port) {
		return fmt.Errorf("http port already in use: %s:%d", opts.host, opts.port)
	}

	cliRoot, err := locateCliRoot(repoRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o755); err != nil {
		return err
	}
	logPath := chromeServiceLogPath(repoRoot)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()

	serviceBinPath := filepath.Join(repoRoot, "tmp", "chrome-v1-service")
	buildCmd := exec.Command("go", "build", "-o", serviceBinPath, ".")
	buildCmd.Dir = cliRoot
	buildCmd.Stdout = logFile
	buildCmd.Stderr = logFile
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("build service binary: %w", err)
	}

	cmdArgs := []string{
		"__service-loop",
		"--host", opts.host,
		"--port", strconv.Itoa(opts.port),
		"--nats-url", opts.natsURL,
		"--nats-prefix", opts.natsPrefix,
		"--chrome-debug-port", strconv.Itoa(opts.chromeDebugPort),
		"--initial-url", opts.initialURL,
	}
	if opts.embeddedNATS {
		cmdArgs = append(cmdArgs, "--embedded-nats")
	}
	if opts.headless {
		cmdArgs = append(cmdArgs, "--headless")
	}

	cmd := exec.Command(serviceBinPath, cmdArgs...)
	cmd.Dir = repoRoot
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o644); err != nil {
		_ = cmd.Process.Kill()
		return err
	}

	if err := waitForServiceStart(cmd.Process.Pid, opts.host, opts.port, 45*time.Second); err != nil {
		_ = cmd.Process.Kill()
		_ = os.Remove(pidPath)
		return fmt.Errorf("service failed health check: %w (see log: %s)", err, logPath)
	}

	fmt.Printf("chrome service started pid=%d url=http://%s:%d nats=%s prefix=%s (log=%s)\n",
		cmd.Process.Pid, opts.host, opts.port, opts.natsURL, opts.natsPrefix, logPath)
	return nil
}

func runStop(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("chrome service stop does not accept positional arguments")
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	pidPath := chromeServicePIDPath(repoRoot)
	pid, err := readPID(pidPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("chrome service is not running")
			return nil
		}
		return err
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if processAlive(pid) {
		_ = proc.Signal(syscall.SIGKILL)
	}
	_ = os.Remove(pidPath)
	fmt.Printf("chrome service stopped pid=%d\n", pid)
	return nil
}

func runStatus(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("chrome service status does not accept positional arguments")
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	pidPath := chromeServicePIDPath(repoRoot)
	if pid, err := readPID(pidPath); err == nil && !processAlive(pid) {
		_ = os.Remove(pidPath)
	}
	if running, pid := serviceRunning(pidPath); running {
		fmt.Printf("chrome service running pid=%d\n", pid)
		return nil
	}
	fmt.Println("chrome service not running")
	return nil
}

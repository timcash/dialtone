package ops

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"dialtone/dev/browser"
	test_v2 "dialtone/dev/plugins/test"
	chrome_app "dialtone/dev/plugins/chrome/app"
)

func Dev(args []string) error {
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "robot", "src_v1")
	uiDir := filepath.Join(pluginDir, "ui")
	devLogPath := filepath.Join(pluginDir, "dev.log")
	devBrowserMetaPath := filepath.Join(pluginDir, "dev.browser.json")
	devPort := 3000
	devURL := fmt.Sprintf("http://127.0.0.1:%d", devPort)

	// Check for flags
	useRemoteRobot := false
	remoteChromeHost := ""
	for i, arg := range args {
		if arg == "--robot" {
			useRemoteRobot = true
		}
		if arg == "--chrome-debug" && i+1 < len(args) {
			remoteChromeHost = args[i+1]
		}
	}

	logFile, err := os.OpenFile(devLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open dev log at %s: %w", devLogPath, err)
	}
	defer logFile.Close()

	logOut := io.MultiWriter(os.Stdout, logFile)
	logf := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		fmt.Fprintln(logOut, msg)
	}

	logf(">> [ROBOT] Dev: src_v1")
	logf("   [DEV] Writing logs to %s", devLogPath)
	logf("   [DEV] Writing browser metadata to %s", devBrowserMetaPath)

	logf("   [DEV] Checking for existing robot dev processes...")
	cleanupExistingDev(logf, cwd)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Handle Remote Chrome Debug Bridge
	if remoteChromeHost != "" {
		logf("   [DEV] Establishing bridge to remote Chrome on %s:9222...", remoteChromeHost)
		go startRemoteChromeBridge(logf, remoteChromeHost)
	}

	// Manage Remote vs Local Backend dynamically
	bm := &BackendManager{
		ctx:            ctx,
		logf:           logf,
		logOut:         logOut,
		cwd:            cwd,
		useRemote:      useRemoteRobot,
		remoteHost:     os.Getenv("ROBOT_HOST"),
		remoteUser:     os.Getenv("ROBOT_USER"),
		failoverToMock: true,
	}

	if bm.remoteHost == "" {
		bm.remoteHost = "drone-1"
	}

	go bm.Start()

	if _, err := os.Stat(uiDir); os.IsNotExist(err) {
		return fmt.Errorf("UI directory not found: %s", uiDir)
	}

	var (
		mu               sync.Mutex
		session          *test_v2.BrowserSession
		browserBooted    bool
		restartAttemptID int
		lastRestartAt    time.Time
		backoffDuration  = 1 * time.Second
	)

	// Vite Dev Server Loop
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			restartAttemptID++

			// Exponential Backoff / Rate Limit
			if !lastRestartAt.IsZero() {
				elapsed := time.Since(lastRestartAt)
				if elapsed < 10*time.Second {
					logf("   [DEV] Restarting... (Cooldown: %v)", backoffDuration)
					time.Sleep(backoffDuration)
					// Increase backoff for next time, max 30s
					backoffDuration *= 2
					if backoffDuration > 30*time.Second {
						backoffDuration = 30 * time.Second
					}
				} else {
					// Reset backoff if it has been running well for > 10s
					backoffDuration = 1 * time.Second
				}
			}
			lastRestartAt = time.Now()

			logf("   [DEV] Cleaning up port %d...", devPort)
			_ = browser.CleanupPort(devPort)

			logf("   [DEV] Launching Vite (Attempt %d)...", restartAttemptID)

			cmd := exec.CommandContext(ctx, filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(devPort), "--strictPort", "--force")

			// Create a pipe to monitor output for CRITICAL errors
			pr, pw := io.Pipe()
			
			var (
				errorCount int
				errorMu    sync.Mutex
				lastErrorAt time.Time
			)

			// Custom multi-writer that filters common noise
			logFilter := func(p []byte) (n int, err error) {
				line := string(p)
				lower := strings.ToLower(line)
				
				// Identify common noise
				isNoise := strings.Contains(lower, "econnreset") || 
				           strings.Contains(lower, "epipe") || 
						   strings.Contains(lower, "ws proxy socket error")

				if isNoise {
					errorMu.Lock()
					errorCount++
					now := time.Now()
					// Only print heartbeat if it's been more than 10s since the last one
					if now.Sub(lastErrorAt) > 10*time.Second {
						lastErrorAt = now
						msg := fmt.Sprintf("   [DEV] Heartbeat: Background connection issues detected (%d errors). Suppressing spam...\n", errorCount)
						logOut.Write([]byte(msg))
					}
					errorMu.Unlock()
					return len(p), nil // Suppress the actual noise
				}

				if len(line) > 1000 {
					line = line[:1000] + "... [TRUNCATED]"
				}
				return logOut.Write([]byte(line))
			}
			
			multiOut := io.MultiWriter(writerFunc(logFilter), pw)
			cmd.Stdout = multiOut
			cmd.Stderr = multiOut

			if err := cmd.Start(); err != nil {
				logf("   [DEV] Failed to start vite: %v", err)
				continue
			}

			// Monitor for "CRITICAL ERROR" in output or high error volume
			go func() {
				scanner := bufio.NewScanner(pr)
				for scanner.Scan() {
					line := scanner.Text()
					lower := strings.ToLower(line)
					
					errorMu.Lock()
					currentCount := errorCount
					errorMu.Unlock()

					// Auto-restart if we hit 100 background errors (likely a hung tunnel/proxy)
					if currentCount > 100 {
						logf("   [DEV] High error volume detected (%d). Triggering graceful restart...", currentCount)
						_ = cmd.Process.Signal(os.Interrupt)
						break
					}

					if strings.Contains(lower, "failed to load config") ||
						strings.Contains(lower, "error during build") ||
						strings.Contains(lower, "syntax error") {
						logf("   [DEV] CRITICAL Vite error detected: %s", line)
						logf("   [DEV] Action required: Fix the code error above or check vite.config.ts")
						_ = cmd.Process.Signal(os.Interrupt)
						break
					}
				}
			}()

			go func(attempt int) {
				if err := test_v2.WaitForPort(devPort, 30*time.Second); err != nil {
					logf("   [DEV] Warning: vite server not ready on port %d: %v", devPort, err)
					return
				}

				mu.Lock()
				alreadyBooted := browserBooted
				mu.Unlock()
				if alreadyBooted {
					logf("   [DEV] Vite ready at %s (attempt %d); keeping existing browser session", devURL, attempt)
					return
				}

				logf("   [DEV] Vite ready at %s", devURL)
				logf("   [DEV] Opening dev URL in browser...")

				s, err := startRobotDevBrowser(logf, devURL, devBrowserMetaPath)
				if err != nil {
					logf("   [DEV] Warning: failed to launch debug browser: %v", err)
					return
				}
				mu.Lock()
				session = s
				browserBooted = true
				mu.Unlock()
			}(restartAttemptID)

			_ = cmd.Wait()
			if ctx.Err() != nil {
				return
			}
			logf("   [DEV] Vite process exited. Restarting in 1s...")
			time.Sleep(time.Second)
		}
	}()

	<-ctx.Done()
	logf("   [DEV] Shutting down...")
	
	mu.Lock()
	if session != nil {
		session.Close()
	}
	mu.Unlock()
	
	bm.stopCurrent()

	return nil
}

func startRobotDevBrowser(logf func(string, ...any), devURL, devBrowserMetaPath string) (*test_v2.BrowserSession, error) {
	logf("   [DEV] Starting browser session (role=robot-dev)...")
	
	s, err := test_v2.StartBrowser(test_v2.BrowserOptions{
		Headless:      false,
		Role:          "robot-dev",
		ReuseExisting: true,
		URL:           devURL,
		LogWriter:     nil, 
		LogPrefix:     "",
	})
	
	if err != nil {
		logf("   [DEV] Warning: managed browser start failed: %v", err)
		if openErr := openInRegularChrome(devURL); openErr != nil {
			return nil, fmt.Errorf("failed to open regular chrome fallback: %v", openErr)
		}
		return nil, nil
	}
	
	if err := chrome_app.WriteSessionMetadata(devBrowserMetaPath, s.ChromeSession()); err != nil {
		logf("   [DEV] Warning: failed to write browser metadata: %v", err)
	}

	return s, nil
}

func openInRegularChrome(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("osascript", "-e", fmt.Sprintf(`tell application "Google Chrome" to open location %q`, url)).Run()
	case "linux":
		return exec.Command("xdg-open", url).Run()
	case "windows":
		return exec.Command("cmd", "/c", "start", "", "chrome", url).Run()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func cleanupExistingDev(logf func(string, ...any), repoRoot string) {
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "ps", "tracked")
	out, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(out), "\n")
	myPID := os.Getpid()

	for _, line := range lines {
		if strings.Contains(line, "robot_dev") && strings.Contains(line, "running") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			key := fields[0]
			pidStr := fields[1]
			pid, _ := strconv.Atoi(pidStr)

			if pid == myPID || pid == os.Getppid() {
				continue
			}

			logf("   [DEV] Stopping conflicting process: %s (pid %d)", key, pid)
			stopCmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "proc", "stop", key)
			_ = stopCmd.Run()
		}
	}
}

func startRemoteChromeBridge(logf func(string, ...any), targetHost string) {
	l, err := net.Listen("tcp", "127.0.0.1:9222")
	if err != nil {
		logf("   [BRIDGE] Error: Could not listen on :9222 (maybe already in use?): %v", err)
		return
	}
	defer l.Close()

	for {
		localConn, err := l.Accept()
		if err != nil {
			return
		}

		go func(lConn net.Conn) {
			defer lConn.Close()
			remoteConn, err := net.DialTimeout("tcp", net.JoinHostPort(targetHost, "9222"), 5*time.Second)
			if err != nil {
				return
			}
			defer remoteConn.Close()

			done := make(chan struct{}, 2)
			go func() {
				io.Copy(remoteConn, lConn)
				done <- struct{}{}
			}()
			go func() {
				io.Copy(lConn, remoteConn)
				done <- struct{}{}
			}()
			<-done
		}(localConn)
	}
}

type writerFunc func([]byte) (int, error)
func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

type BackendManager struct {
	ctx            context.Context
	logf           func(string, ...any)
	logOut         io.Writer
	cwd            string
	useRemote      bool
	remoteHost     string
	remoteUser     string
	failoverToMock bool

	mu             sync.Mutex
	activeCmd      *exec.Cmd
	isMockActive   bool
	isFailover     bool
}

func (bm *BackendManager) Start() {
	if bm.useRemote {
		bm.logf("   [DEV] Remote Robot mode enabled. Attempting to connect to %s...", bm.remoteHost)
		if err := bm.startRemote(); err != nil {
			bm.logf("   [DEV] Initial remote connection failed: %v", err)
			if bm.failoverToMock {
				bm.TriggerFailover("Initial connection failure")
			}
		}
	} else {
		bm.startMock()
	}

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-bm.ctx.Done():
				return
			case <-ticker.C:
				bm.mu.Lock()
				failoverActive := bm.isFailover
				bm.mu.Unlock()

				if failoverActive {
					bm.logf("   [DEV] Recovery Probe: Checking if remote robot %s is back online...", bm.remoteHost)
					if bm.probeRemote() {
						bm.logf("   [DEV] Recovery: Remote robot is healthy. Swapping back to tunnel...")
						bm.restoreRemote()
					}
				}
			}
		}
	}()
}

func (bm *BackendManager) TriggerFailover(reason string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.isFailover || !bm.useRemote {
		return
	}

	bm.logf("   [DEV] ALERT: %s", reason)
	bm.logf("   [DEV] FAILING OVER: Stopping remote tunnel and starting local mock server.")
	
	bm.stopCurrent()
	bm.isFailover = true
	bm.startMockInternal()
}

func (bm *BackendManager) startRemote() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	return bm.startRemoteInternal()
}

func (bm *BackendManager) startRemoteInternal() error {
	target := bm.remoteHost
	if bm.remoteUser != "" {
		target = fmt.Sprintf("%s@%s", bm.remoteUser, bm.remoteHost)
	}

	cmd := exec.CommandContext(bm.ctx, "ssh", "-N", "-o", "ConnectTimeout=5", "-L", "8080:localhost:8080", "-L", "4223:localhost:4223", target)
	cmd.Stdout = bm.logOut
	cmd.Stderr = bm.logOut
	if err := cmd.Start(); err != nil {
		return err
	}
	bm.activeCmd = cmd
	bm.isMockActive = false
	bm.logf("   [DEV] SSH tunnel started (pid %d)", cmd.Process.Pid)
	return nil
}

func (bm *BackendManager) startMock() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.startMockInternal()
}

func (bm *BackendManager) startMockInternal() {
	bm.logf("   [DEV] Starting local mock backend...")
	cmd := exec.CommandContext(bm.ctx, filepath.Join(bm.cwd, "dialtone.sh"), "go", "exec", "run", "src/plugins/robot/src_v1/cmd/main.go")
	cmd.Dir = bm.cwd
	cmd.Stdout = bm.logOut
	cmd.Stderr = bm.logOut
	if err := cmd.Start(); err != nil {
		bm.logf("   [DEV] Error: Failed to start local backend: %v", err)
		return
	}
	bm.activeCmd = cmd
	bm.isMockActive = true
	bm.logf("   [DEV] Local mock backend started (pid %d)", cmd.Process.Pid)
}

func (bm *BackendManager) stopCurrent() {
	if bm.activeCmd != nil && bm.activeCmd.Process != nil {
		_ = bm.activeCmd.Process.Kill()
		_, _ = bm.activeCmd.Process.Wait()
	}
}

func (bm *BackendManager) probeRemote() bool {
	target := bm.remoteHost
	if bm.remoteUser != "" {
		target = fmt.Sprintf("%s@%s", bm.remoteUser, bm.remoteHost)
	}
	cmd := exec.Command("ssh", "-o", "ConnectTimeout=3", "-o", "BatchMode=yes", target, "true")
	err := cmd.Run()
	return err == nil
}

func (bm *BackendManager) restoreRemote() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.logf("   [DEV] RESTORING: Remote robot %s is back. Stopping mock and restarting tunnel.", bm.remoteHost)
	bm.stopCurrent()
	bm.isFailover = false
	if err := bm.startRemoteInternal(); err != nil {
		bm.logf("   [DEV] Restoration failed: %v. Falling back to mock again.", err)
		bm.isFailover = true
		bm.startMockInternal()
	}
}

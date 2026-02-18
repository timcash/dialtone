package ops

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
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

	"dialtone/cli/src/core/browser"
	test_v2 "dialtone/cli/src/libs/test_v2"
	chrome_app "dialtone/cli/src/plugins/chrome/app"
)

func Dev(args []string) error {
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "robot", "src_v1")
	uiDir := filepath.Join(pluginDir, "ui")
	devLogPath := filepath.Join(pluginDir, "dev.log")
	devBrowserMetaPath := filepath.Join(pluginDir, "dev.browser.json")
	devPort := 3000
	devURL := fmt.Sprintf("http://127.0.0.1:%d", devPort)

	// Check for --robot flag
	useRemoteRobot := false
	for _, arg := range args {
		if arg == "--robot" {
			useRemoteRobot = true
			break
		}
	}

	logFile, err := os.OpenFile(devLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open dev log at %s: %w", devLogPath, err)
	}
	defer logFile.Close()

	logOut := io.MultiWriter(os.Stdout, logFile)
	logf := func(format string, args ...any) {
		fmt.Fprintf(logOut, format+"\n", args...)
	}

	        logf(">> [ROBOT] Dev: src_v1")

	        logf("   [DEV] Writing logs to %s", devLogPath)

	        logf("   [DEV] Writing browser metadata to %s", devBrowserMetaPath)

	

	        // NEW: Cleanup existing robot dev processes

	        logf("   [DEV] Checking for existing robot dev processes...")

	        cleanupExistingDev(logf, cwd)

	

	        ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	
	defer stop()

	// Handle Remote Robot Tunnel or Local Backend
	var backendCmd *exec.Cmd
	var tunnelCmd *exec.Cmd

	if useRemoteRobot {
		host := os.Getenv("ROBOT_HOST")
		user := os.Getenv("ROBOT_USER")
		if host == "" {
			host = "drone-1"
		}
		
		target := host
		if user != "" {
			target = fmt.Sprintf("%s@%s", user, host)
		}

		logf("   [DEV] Remote Robot mode enabled. Connecting to %s...", host)
		
		// Start SSH Tunnel
		// -L 8080:localhost:8080 for API
		// -L 4223:localhost:4223 for NATS WS
		tunnelCmd = exec.CommandContext(ctx, "ssh", "-N", "-L", "8080:localhost:8080", "-L", "4223:localhost:4223", target)
		tunnelCmd.Stdout = logOut
		tunnelCmd.Stderr = logOut
		if err := tunnelCmd.Start(); err != nil {
			return fmt.Errorf("failed to start ssh tunnel: %w", err)
		}
		logf("   [DEV] SSH tunnel started (pid %d)", tunnelCmd.Process.Pid)
		
		go func() {
			if err := tunnelCmd.Wait(); err != nil {
				if ctx.Err() == nil {
					logf("   [DEV] Warning: SSH tunnel exited unexpectedly: %v", err)
					stop()
				}
			}
		}()
		
		time.Sleep(2 * time.Second)

	} else {
		logf("   [DEV] Starting local mock backend...")
		backendCmd = exec.CommandContext(ctx, filepath.Join(cwd, "dialtone.sh"), "go", "exec", "run", "src/plugins/robot/src_v1/cmd/main.go")
		backendCmd.Dir = cwd
		backendCmd.Stdout = logOut
		backendCmd.Stderr = logOut
		if err := backendCmd.Start(); err != nil {
			return fmt.Errorf("failed to start local backend: %w", err)
		}
		logf("   [DEV] Local backend started (pid %d)", backendCmd.Process.Pid)
	}

	if _, err := os.Stat(uiDir); os.IsNotExist(err) {
		return fmt.Errorf("UI directory not found: %s", uiDir)
	}

	var (
		mu               sync.Mutex
		session          *test_v2.BrowserSession
		browserBooted    bool
		restartAttemptID int
	)

	        // Vite Dev Server Loop

	        go func() {

	                for {

	                        if ctx.Err() != nil {

	                                return

	                        }

	                        restartAttemptID++

	                        logf("   [DEV] Cleaning up port %d...", devPort)

	                        _ = browser.CleanupPort(devPort)

	

	                        logf("   [DEV] Running vite dev... (attempt %d)", restartAttemptID)

	

	                        cmd := exec.CommandContext(ctx, filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(devPort), "--strictPort", "--force")

	                        

	                        // Create a pipe to monitor output for errors

	                        pr, pw := io.Pipe()

	                        multiOut := io.MultiWriter(logOut, pw)

	                        cmd.Stdout = multiOut

	                        cmd.Stderr = multiOut

	

	                        if err := cmd.Start(); err != nil {

	                                logf("   [DEV] Failed to start vite: %v", err)

	                                time.Sleep(2 * time.Second)

	                                continue

	                        }

	

	                        // Monitor for "error" in output to trigger restart

	                        go func() {

	                                scanner := bufio.NewScanner(pr)

	                                for scanner.Scan() {

	                                        line := scanner.Text()

	                                        if strings.Contains(strings.ToLower(line), "error") && 

	                                           !strings.Contains(line, "node_modules") { // Ignore some noise

	                                                logf("   [DEV] Vite error detected, triggering restart...")

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
	
	if backendCmd != nil && backendCmd.Process != nil {
		_ = backendCmd.Process.Kill()
	}
	if tunnelCmd != nil && tunnelCmd.Process != nil {
		_ = tunnelCmd.Process.Kill()
	}

	return nil
}

func startRobotDevBrowser(logf func(string, ...any), devURL, devBrowserMetaPath string) (*test_v2.BrowserSession, error) {
	logf("   [DEV] Starting browser session (role=robot-dev)...")
	
	// We use test_v2.StartBrowser which handles finding or launching Chrome
	// and connecting via CDP.
	s, err := test_v2.StartBrowser(test_v2.BrowserOptions{
		Headless:      false,
		Role:          "robot-dev",
		ReuseExisting: true,
		URL:           devURL,
		LogWriter:     nil, // Clean dev logs
		LogPrefix:     "",
	})
	
	if err != nil {
		// Fallback to regular chrome if managed fails?
		// But StartBrowser usually tries pretty hard.
		// If it fails, maybe we just try to open regular chrome as last resort.
		logf("   [DEV] Warning: managed browser start failed: %v", err)
		if openErr := openInRegularChrome(devURL); openErr != nil {
			return nil, fmt.Errorf("failed to open regular chrome fallback: %v", openErr)
		}
		return nil, nil
	}
	
	// Write metadata for attach support
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

func hasReachableDevtoolsWebSocket(port int) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func isRegularChromeLikelyRunning() bool {
	if runtime.GOOS == "darwin" {
		return exec.Command("pgrep", "-x", "Google Chrome").Run() == nil
	}
	if runtime.GOOS == "linux" {
		// Simple check
		out, _ := exec.Command("pgrep", "-x", "chrome").Output()
		return len(out) > 0
	}
	return false
}

func cleanupExistingDev(logf func(string, ...any), repoRoot string) {
	// Use dialtone.sh ps tracked to find existing robot_dev processes
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

			// Don't kill ourselves or our parent shell
			if pid == myPID || pid == os.Getppid() {
				continue
			}

			logf("   [DEV] Stopping conflicting process: %s (pid %d)", key, pid)
			stopCmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "proc", "stop", key)
			_ = stopCmd.Run()
		}
	}
}

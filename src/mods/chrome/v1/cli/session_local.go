package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type SessionOptions struct {
	RequestedPort int
	GPU           bool
	Headless      bool
	Kiosk         bool
	TargetURL     string
	Role          string
	ReuseExisting bool
	UserDataDir   string
	DebugAddress  string
}

type Session struct {
	PID          int
	Port         int
	WebSocketURL string
	IsNew        bool
	UserDataDir  string
}

func StartSession(opts SessionOptions) (*Session, error) {
	port := opts.RequestedPort
	if port == 0 {
		var err error
		port, err = findFreePort()
		if err != nil {
			return nil, fmt.Errorf("allocate chrome debug port: %w", err)
		}
	}

	chromePath, err := findChromePath()
	if err != nil {
		return nil, err
	}

	debugAddress := strings.TrimSpace(opts.DebugAddress)
	if debugAddress == "" {
		debugAddress = "127.0.0.1"
	}
	targetURL := strings.TrimSpace(opts.TargetURL)
	if targetURL == "" {
		targetURL = "about:blank"
	}
	role := strings.TrimSpace(opts.Role)
	if role == "" {
		role = "chrome-v1-service"
	}

	userDataDir := strings.TrimSpace(opts.UserDataDir)
	if userDataDir == "" {
		userDataDir = filepath.Join(os.TempDir(), "dialtone", "chrome-v1", role, strconv.Itoa(port))
	}
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create chrome profile dir: %w", err)
	}

	args := []string{
		"--remote-debugging-port=" + strconv.Itoa(port),
		"--remote-debugging-address=" + debugAddress,
		"--remote-allow-origins=*",
		"--user-data-dir=" + userDataDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--dialtone-role=" + role,
	}
	if opts.Headless {
		args = append(args, "--headless=new")
	}
	if !opts.GPU {
		args = append(args, "--disable-gpu")
	}
	if opts.Kiosk {
		args = append(args, "--kiosk")
	}
	args = append(args, targetURL)

	cmd := exec.Command(chromePath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start chrome: %w", err)
	}

	wsURL, err := waitForWebSocketURL(port, 20*time.Second)
	if err != nil {
		_ = killPID(cmd.Process.Pid)
		return nil, fmt.Errorf("wait for devtools: %w", err)
	}

	return &Session{
		PID:          cmd.Process.Pid,
		Port:         port,
		WebSocketURL: wsURL,
		IsNew:        true,
		UserDataDir:  userDataDir,
	}, nil
}

func CleanupSession(sess *Session) error {
	if sess == nil {
		return nil
	}
	if sess.PID > 0 {
		_ = killPID(sess.PID)
	}
	return nil
}

func findChromePath() (string, error) {
	candidates := []string{}
	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates,
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		)
	case "linux":
		candidates = append(candidates, "google-chrome", "chromium", "chromium-browser")
	case "windows":
		candidates = append(candidates,
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		)
	}
	for _, candidate := range candidates {
		if filepath.IsAbs(candidate) {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
			continue
		}
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("chrome executable not found")
}

func findFreePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func waitForWebSocketURL(port int, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	versionURL := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	client := &http.Client{Timeout: 800 * time.Millisecond}
	for time.Now().Before(deadline) {
		resp, err := client.Get(versionURL) //nolint:gosec
		if err == nil {
			var payload struct {
				WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
			}
			decodeErr := json.NewDecoder(resp.Body).Decode(&payload)
			_ = resp.Body.Close()
			if decodeErr == nil && strings.TrimSpace(payload.WebSocketDebuggerURL) != "" {
				return payload.WebSocketDebuggerURL, nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return "", fmt.Errorf("timed out waiting for chrome devtools on port %d", port)
}

func killPID(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = proc.Signal(syscall.SIGKILL)
	return nil
}

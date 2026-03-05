package main

import (
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestChromeCLISteps(t *testing.T) {
	// 1. Cleanup any existing Dialtone Chrome
	runDialtone(t, "chrome", "src_v1", "kill", "all")

	// 2. Start a new local instance
	out := runDialtone(t, "chrome", "src_v1", "new", "about:blank", "--role", "test")
	pid, port := parseNewOutput(t, out)
	t.Logf("Started Chrome: PID=%d, Port=%d", pid, port)

	// 3. Verify it shows up in list
	out = runDialtone(t, "chrome", "src_v1", "list")
	if !strings.Contains(out, strconv.Itoa(pid)) {
		t.Errorf("list output missing PID %d:\n%s", pid, out)
	}
	if !strings.Contains(out, "Role=test") {
		t.Errorf("list output missing Role=test:\n%s", out)
	}

	// 4. Start the service daemon
	servicePort := 19555
	daemonCmd := startDialtoneDaemon(t, servicePort)
	defer daemonCmd.Process.Kill()

	// Wait for daemon to be ready
	waitForDaemon(t, servicePort)

	// 5. Use goto via daemon
	runDialtone(t, "chrome", "src_v1", "goto", "https://example.com")
	
	// 6. Verify URL
	out = runDialtone(t, "chrome", "src_v1", "get-url")
	if !strings.Contains(out, "example.com") {
		t.Errorf("get-url expected example.com, got: %s", out)
	}

	// 7. Test single-tab policy (goto should not create new processes)
	runDialtone(t, "chrome", "src_v1", "goto", "https://google.com")
	out = runDialtone(t, "chrome", "src_v1", "list")
	// Count Dialtone instances
	matches := regexp.MustCompile(`PID=\d+ Role=test`).FindAllString(out, -1)
	if len(matches) > 1 {
		t.Errorf("expected 1 test instance, found %d:\n%s", len(matches), out)
	}

	// 8. Final verify URL
	out = runDialtone(t, "chrome", "src_v1", "get-url")
	if !strings.Contains(out, "google.com") {
		t.Errorf("get-url expected google.com, got: %s", out)
	}
}

func runDialtone(t *testing.T, args ...string) string {
	script := "C:\\Users\\timca\\dialtone\\dialtone.ps1"
	fullArgs := append([]string{"-NoProfile", "-Command", script}, args...)
	cmd := exec.Command("powershell.exe", fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dialtone %v failed: %v\nOutput: %s", args, err, string(out))
	}
	return string(out)
}

func startDialtoneDaemon(t *testing.T, port int) *exec.Cmd {
	script := "C:\\Users\\timca\\dialtone\\dialtone.ps1"
	fullArgs := []string{"-NoProfile", "-Command", script, "chrome", "src_v1", "service-daemon", "--listen-port", strconv.Itoa(port)}
	cmd := exec.Command("powershell.exe", fullArgs...)
	err := cmd.Start()
	if err != nil {
		t.Fatalf("failed to start service-daemon: %v", err)
	}
	return cmd
}

func waitForDaemon(t *testing.T, port int) {
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	timeout := time.After(10 * time.Second)
	tick := time.Tick(500 * time.Millisecond)
	for {
		select {
		case <-timeout:
			t.Fatal("timed out waiting for service-daemon health check")
		case <-tick:
			resp, err := http.Get(url)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return
				}
			}
		}
	}
}

func parseNewOutput(t *testing.T, out string) (int, int) {
	// Started Chrome PID=18164 Port=55244 WS=ws://...
	re := regexp.MustCompile(`PID=(\d+) Port=(\d+)`)
	m := re.FindStringSubmatch(out)
	if len(m) < 3 {
		t.Fatalf("failed to parse PID/Port from new output:\n%s", out)
	}
	pid, _ := strconv.Atoi(m[1])
	port, _ := strconv.Atoi(m[2])
	return pid, port
}

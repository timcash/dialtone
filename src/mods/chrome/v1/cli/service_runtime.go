package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func chromeServicePIDPath(repoRoot string) string {
	return filepath.Join(repoRoot, "tmp", "chrome-v1.pid")
}

func chromeServiceLogPath(repoRoot string) string {
	return filepath.Join(repoRoot, "tmp", "chrome-v1.log")
}

func readPID(path string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return 0, fmt.Errorf("invalid pid file %s: %w", path, err)
	}
	return pid, nil
}

func serviceRunning(pidPath string) (bool, int) {
	pid, err := readPID(pidPath)
	if err != nil {
		return false, 0
	}
	if !processAlive(pid) {
		return false, 0
	}
	return true, pid
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

func findExistingChromeV1ServicePIDs() ([]int, error) {
	out, err := exec.Command("ps", "-axo", "pid=,command=").Output()
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}
	lines := strings.Split(string(out), "\n")
	seen := map[int]struct{}{}
	for _, line := range lines {
		row := strings.TrimSpace(line)
		if row == "" {
			continue
		}
		fields := strings.Fields(row)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 0 {
			continue
		}
		cmd := strings.Join(fields[1:], " ")
		if !strings.Contains(cmd, "chrome-v1-service __service-loop") && !strings.Contains(cmd, "/cli __service-loop") {
			continue
		}
		if !processAlive(pid) {
			continue
		}
		seen[pid] = struct{}{}
	}
	pids := make([]int, 0, len(seen))
	for pid := range seen {
		pids = append(pids, pid)
	}
	sort.Ints(pids)
	return pids, nil
}

func canListen(host string, port int) bool {
	ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func waitForServiceStart(pid int, host string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://%s:%d/healthz", host, port)
	for time.Now().Before(deadline) {
		if pid > 0 && !processAlive(pid) {
			return fmt.Errorf("service process exited before %s became healthy", url)
		}
		resp, err := http.Get(url) //nolint:gosec
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s", url)
}

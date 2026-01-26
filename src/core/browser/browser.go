package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// CleanupPort attempts to kill any process listening on the specified port.
func CleanupPort(port int) error {
	switch runtime.GOOS {
	case "darwin":
		// macOS: Use lsof to find and kill the process
		cmd := exec.Command("lsof", "-ti", fmt.Sprintf(":%d", port))
		out, _ := cmd.Output()
		pids := strings.Fields(string(out))
		for _, pid := range pids {
			fmt.Printf("Cleaning up stale process on port %d (PID: %s)...\n", port, pid)
			exec.Command("kill", "-9", pid).Run()
		}
	case "linux":
		if isWSL() {
			cmd := exec.Command("cmd.exe", "/c", fmt.Sprintf("netstat -ano | findstr :%d", port))
			out, _ := cmd.CombinedOutput()
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				if strings.Contains(line, "LISTENING") {
					parts := strings.Fields(line)
					if len(parts) > 0 {
						pid := parts[len(parts)-1]
						fmt.Printf("Cleaning up stale process on port %d (PID: %s)...\n", port, pid)
						exec.Command("taskkill.exe", "/F", "/PID", pid).Run()
					}
				}
			}
		} else {
			exec.Command("fuser", "-k", fmt.Sprintf("%d/tcp", port)).Run()
		}
	}
	return nil
}

// KillProcessesByName kills all processes with the given name exactly.
func KillProcessesByName(name string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin", "linux":
		// Match exact name only, no -f (full command line) to avoid accidental kills
		cmd = exec.Command("pkill", "-x", name)
	case "windows":
		cmd = exec.Command("taskkill", "/F", "/IM", name+".exe")
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	return cmd.Run()
}

// KillProcessesByPattern kills all processes matching a pattern in their full command line.
// USE WITH CAUTION.
func KillProcessesByPattern(pattern string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin", "linux":
		cmd = exec.Command("pkill", "-9", "-f", pattern)
	case "windows":
		// No direct equivalent for -f in taskkill, but we can use wmic or tasklist pipe
		return fmt.Errorf("KillProcessesByPattern not implemented for Windows")
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	return cmd.Run()
}

func isWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

// GetChildPIDs returns a list of PIDs that are children of the given parent PID.
func GetChildPIDs(parentPID int) ([]int, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin", "linux":
		cmd = exec.Command("pgrep", "-P", fmt.Sprintf("%d", parentPID))
	default:
		return nil, nil // Not implemented for Windows yet
	}
	out, _ := cmd.Output()
	var pids []int
	for _, field := range strings.Fields(string(out)) {
		var pid int
		fmt.Sscanf(field, "%d", &pid)
		if pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

// FindChromePath looks for Chrome/Chromium in common locations based on the OS.
func FindChromePath() string {
	switch runtime.GOOS {
	case "darwin":
		paths := []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			os.Getenv("HOME") + "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	case "windows":
		programFiles := os.Getenv("ProgramFiles")
		if programFiles == "" {
			programFiles = `C:\Program Files`
		}
		programFilesX86 := os.Getenv("ProgramFiles(x86)")
		if programFilesX86 == "" {
			programFilesX86 = `C:\Program Files (x86)`
		}
		localAppData := os.Getenv("LocalAppData")

		paths := []string{
			filepath.Join(programFiles, `Google\Chrome\Application\chrome.exe`),
			filepath.Join(programFilesX86, `Google\Chrome\Application\chrome.exe`),
		}
		if localAppData != "" {
			paths = append(paths, filepath.Join(localAppData, `Google\Chrome\Application\chrome.exe`))
		}

		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	case "linux":
		paths := []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/google-chrome-beta",
			"/usr/bin/google-chrome-unstable",
			"/usr/bin/chromium-browser",
			"/usr/bin/chromium",
			"/usr/bin/brave-browser",
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}

		// Fallback for WSL (Windows Host)
		wslPaths := []string{
			"/mnt/c/Program Files/Google/Chrome/Application/chrome.exe",
			"/mnt/c/Program Files (x86)/Google/Chrome/Application/chrome.exe",
		}
		for _, p := range wslPaths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}

	return ""
}

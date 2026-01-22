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
// This is primarily used to clear stale Chrome/Chromium debug sessions.
func CleanupPort(port int) error {
	if runtime.GOOS == "linux" {
		// On WSL/Linux, we might need to kill Windows processes if we're using the Windows host Chrome
		// We'll try to find the PID using netstat via cmd.exe if on WSL
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
			// Native Linux (fuser or lsof)
			exec.Command("fuser", "-k", fmt.Sprintf("%d/tcp", port)).Run()
		}
	}
	return nil
}

func isWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
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

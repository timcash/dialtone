package browser

import (
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

// resolveWinBin returns the path to a Windows binary, falling back to absolute paths on WSL.
func resolveWinBin(binName string, fallbackPath string) string {
	if _, err := exec.LookPath(binName); err == nil {
		return binName
	}
	if IsWSL() {
		// Common Windows System32 paths for WSL
		fullPath := "/mnt/c/Windows/System32/" + binName
		if binName == "powershell.exe" {
			fullPath = "/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
		}
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath
		}
		// Try provided fallback if any
		if fallbackPath != "" {
			if _, err := os.Stat(fallbackPath); err == nil {
				return fallbackPath
			}
		}
	}
	return binName
}

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
		if IsWSL() {
			cmdBin := resolveWinBin("cmd.exe", "")
			tkBin := resolveWinBin("taskkill.exe", "")
			cmd := exec.Command(cmdBin, "/c", fmt.Sprintf("netstat -ano | findstr :%d", port))
			out, _ := cmd.CombinedOutput()
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				if strings.Contains(line, "LISTENING") {
					parts := strings.Fields(line)
					if len(parts) > 0 {
						pid := parts[len(parts)-1]
						fmt.Printf("Cleaning up stale process on port %d (PID: %s)...\n", port, pid)
						exec.Command(tkBin, "/F", "/PID", pid).Run()
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

func IsWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

// IsPortOpen checks if a TCP port is open on localhost.
func IsPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
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

// ChromeProcess represents a running Chrome/Chromium process.
type ChromeProcess struct {
	PID        int
	PPID       int
	Command    string
	IsWindows  bool    // True if it's a Windows process (for WSL)
	MemoryMB   float64 // RSS in MB
	CPUPerc    float64 // CPU percentage
	ChildCount int     // Number of child processes
	IsHeadless bool    // True if started with --headless
	DebugPort  int     // remote-debugging-port
	GPUEnabled bool    // true unless --disable-gpu is present
	Origin     string  // "Dialtone" or "Other"
}

// ListChromeProcesses returns a list of Chrome-related processes.
// if showAll is false, it filters out sub-processes (renderers, etc.)
func ListChromeProcesses(showAll bool) ([]ChromeProcess, error) {
	var results []ChromeProcess

	// 1. Check native processes (Linux/macOS)
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		pids, err := process.Pids()
		if err == nil {
			for _, pid := range pids {
				p, err := process.NewProcess(pid)
				if err != nil {
					continue
				}

				name, _ := p.Name()
				cmdline, _ := p.Cmdline()

				if name == "" || cmdline == "" {
					continue
				}

				isChrome := (strings.Contains(strings.ToLower(name), "chrome") ||
					strings.Contains(strings.ToLower(name), "chromium") ||
					strings.Contains(strings.ToLower(cmdline), "chrome") ||
					strings.Contains(strings.ToLower(cmdline), "chromium")) &&
					!strings.Contains(cmdline, "chrome list") &&
					!strings.Contains(cmdline, "chrome new")

				if isChrome {
					ppid, _ := p.Ppid()
					mem, _ := p.MemoryInfo()
					cpu, _ := p.CPUPercent()
					children, _ := p.Children()

					memoryMB := 0.0
					if mem != nil {
						memoryMB = float64(mem.RSS) / 1024 / 1024
					}

					results = append(results, ChromeProcess{
						PID:        int(pid),
						PPID:       int(ppid),
						Command:    cmdline,
						IsWindows:  false,
						MemoryMB:   memoryMB,
						CPUPerc:    cpu,
						ChildCount: len(children),
						IsHeadless: strings.Contains(strings.ToLower(cmdline), "--headless"),
						DebugPort:  extractDebugPort(cmdline),
						GPUEnabled: !strings.Contains(strings.ToLower(cmdline), "--disable-gpu"),
						Origin:     detectOrigin(cmdline),
					})
				}
			}
		}
	}

	// 2. Check Windows processes if on WSL
	if runtime.GOOS == "linux" && IsWSL() {
		// Get detailed info for filtering and headless detection
		script := `Get-CimInstance Win32_Process -Filter "Name = 'chrome.exe' OR Name = 'msedge.exe'" | Select-Object ProcessId, CommandLine | ConvertTo-Csv -NoTypeInformation`
		psBin := resolveWinBin("powershell.exe", "")
		cmd := exec.Command(psBin, "-Command", script)
		out, _ := cmd.Output()
		if len(out) > 0 {
			reader := csv.NewReader(strings.NewReader(strings.ReplaceAll(string(out), "\r\n", "\n")))
			reader.FieldsPerRecord = -1
			_, _ = reader.Read() // Skip header
			for {
				record, err := reader.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					continue
				}

				if len(record) >= 2 {
					pidStr := record[0]
					cmdline := record[1]

					var pid int
					fmt.Sscanf(pidStr, "%d", &pid)

					if pid > 0 {
						// Filter out dialtone commands to avoid noise
						if strings.Contains(cmdline, "chrome list") || strings.Contains(cmdline, "chrome new") {
							continue
						}

						results = append(results, ChromeProcess{
							PID:        pid,
							Command:    cmdline,
							IsWindows:  true,
							IsHeadless: strings.Contains(strings.ToLower(cmdline), "--headless"),
							DebugPort:  extractDebugPort(cmdline),
							GPUEnabled: !strings.Contains(strings.ToLower(cmdline), "--disable-gpu"),
							Origin:     detectOrigin(cmdline),
						})
					}
				}
			}
		}
	}

	return results, nil
}

func extractDebugPort(cmdline string) int {
	re := regexp.MustCompile(`--remote-debugging-port=(\d+)`)
	matches := re.FindStringSubmatch(cmdline)
	if len(matches) > 1 {
		var port int
		fmt.Sscanf(matches[1], "%d", &port)
		return port
	}
	return 0
}

func detectOrigin(cmdline string) string {
	if strings.Contains(cmdline, "--dialtone-origin=true") || strings.Contains(cmdline, "dialtone-chrome-port-") {
		return "Dialtone"
	}
	return "Other"
}

// KillProcessByPID kills a process by its PID.
func KillProcessByPID(pid int, isWindows bool) error {
	if isWindows && runtime.GOOS == "linux" && IsWSL() {
		// taskkill.exe is for the real Windows PID
		tkBin := resolveWinBin("taskkill.exe", "")
		return exec.Command(tkBin, "/F", "/PID", fmt.Sprintf("%d", pid)).Run()
	}

	// For native Linux/macOS or interop proxies in WSL, use normal kill
	switch runtime.GOOS {
	case "windows":
		return exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid)).Run()
	default:
		return exec.Command("kill", "-9", fmt.Sprintf("%d", pid)).Run()
	}
}

// KillAllChromeProcesses kills all chrome and msedge processes.
func KillAllChromeProcesses() error {
	if runtime.GOOS == "linux" && IsWSL() {
		// Kill Windows processes
		psBin := resolveWinBin("powershell.exe", "")
		_ = exec.Command(psBin, "-Command", "Get-Process chrome, msedge -ErrorAction SilentlyContinue | Stop-Process -Force").Run()
		// Also kill native Linux processes if any
	}

	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("taskkill", "/F", "/IM", "chrome.exe").Run()
		_ = exec.Command("taskkill", "/F", "/IM", "msedge.exe").Run()
	case "darwin", "linux":
		_ = exec.Command("pkill", "-9", "chrome").Run()
		_ = exec.Command("pkill", "-9", "chromium").Run()
		_ = exec.Command("pkill", "-9", "msedge").Run()
	}
	return nil
}

// KillDialtoneChromeProcesses kills only processes started by this tool.
func KillDialtoneChromeProcesses() error {
	procs, err := ListChromeProcesses(true)
	if err != nil {
		return err
	}
	for _, p := range procs {
		if p.Origin == "Dialtone" {
			_ = KillProcessByPID(p.PID, p.IsWindows)
		}
	}
	return nil
}
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

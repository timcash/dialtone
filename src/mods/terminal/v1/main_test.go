package main

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBuildTerminalCommandKeepsInteractiveShell(t *testing.T) {
	cmd := buildTerminalCommand("false", "")

	if !strings.Contains(cmd, "false; bind 'set bell-style none' >/dev/null 2>&1; exec ${SHELL:-/bin/bash} -i") {
		t.Fatalf("terminal command does not force interactive shell handoff: %q", cmd)
	}
}

func TestBuildLocalTypedCommand(t *testing.T) {
	cmd := buildLocalTypedCommand("echo hi", "/tmp/repo")
	if cmd != "cd '/tmp/repo' && echo hi" {
		t.Fatalf("unexpected local typed command: %q", cmd)
	}
}

func TestBuildLocalLauncherScriptCommandIncludesInteractiveCommand(t *testing.T) {
	repoRoot, err := locateRepoRoot()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}

	cmd, err := buildLocalLauncherScriptCommand(
		repoRoot,
		"/mnt/c/Windows/System32/wsl.exe",
		"",
		"",
		"",
		"echo hi",
	)
	if err != nil {
		t.Fatalf("build launcher command: %v", err)
	}

	if !strings.Contains(cmd, "Start-DialtoneLocalTerminal") {
		t.Fatalf("launcher command missing function call: %q", cmd)
	}
	if !strings.Contains(cmd, "-WslPath 'C:\\Windows\\System32\\wsl.exe'") {
		t.Fatalf("launcher command missing resolved wsl path: %q", cmd)
	}
	if !strings.Contains(cmd, "-CommandText 'echo hi'") {
		t.Fatalf("launcher command missing typed command text: %q", cmd)
	}
	if !strings.Contains(cmd, "-LogPath 'C:\\Users\\Public\\dialtone-typing-terminal.log'") {
		t.Fatalf("launcher command missing default windows log path: %q", cmd)
	}
}

func TestLauncherScriptUsesQueueReuse(t *testing.T) {
	repoRoot, err := locateRepoRoot()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}

	path := repoRoot + "/src/mods/terminal/v1/launch_local_terminal.ps1"
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read script: %v", err)
	}
	text := string(body)

	if !strings.Contains(text, ".queue.txt") {
		t.Fatalf("script missing queue file")
	}
	if !strings.Contains(text, ".state.json") {
		t.Fatalf("script missing state file for window reuse")
	}
	if !strings.Contains(text, "Invoke-Expression $cmd") {
		t.Fatalf("script does not execute queued commands")
	}
	if !strings.Contains(text, "Start-Process -FilePath 'C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe'") {
		t.Fatalf("script does not launch powershell window")
	}
}

func TestLaunchesVisibleLocalTerminal(t *testing.T) {
	if os.Getenv("DIALTONE_OPEN_TERMINAL_TEST") != "1" {
		t.Skip("set DIALTONE_OPEN_TERMINAL_TEST=1 to run visible terminal launch test")
	}
	if runtime.GOOS != "windows" && os.Getenv("WSL_DISTRO_NAME") == "" {
		t.Skip("requires Windows or WSL")
	}

	const launchLog = "/mnt/c/Users/Public/dialtone-typing-terminal.log"
	const stateLog = "/mnt/c/Users/Public/dialtone-typing-terminal.log.state.json"
	const queueLog = "/mnt/c/Users/Public/dialtone-typing-terminal.log.queue.txt"

	_ = os.Remove(stateLog)
	_ = os.Remove(queueLog)

	if err := runTerminal([]string{"--command", "Write-Host 'DIALTONE_VISUAL_TEST'"}); err != nil {
		t.Fatalf("runTerminal: %v", err)
	}

	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(stateLog); err == nil {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if _, err := os.Stat(stateLog); err != nil {
		t.Fatalf("state file not created: %v", err)
	}

	visible, err := countVisibleDialtonePowerShellWindows()
	if err != nil {
		t.Fatalf("count visible windows: %v", err)
	}
	if visible < 1 {
		launchTail := ""
		if data, readErr := os.ReadFile(launchLog); readErr == nil {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) > 8 {
				lines = lines[len(lines)-8:]
			}
			launchTail = strings.Join(lines, "\n")
		}
		t.Fatalf("expected at least one visible DialtoneTyping window, got %d\nlaunch log tail:\n%s", visible, launchTail)
	}
}

func countVisibleDialtonePowerShellWindows() (int, error) {
	ps := resolvePowerShellPath(defaultPowerShellPath())
	if strings.TrimSpace(ps) == "" {
		return 0, os.ErrNotExist
	}
	cmd := exec.Command(
		ps,
		"-NoProfile",
		"-Command",
		"(Get-Process -Name powershell -ErrorAction SilentlyContinue | Where-Object { $_.MainWindowHandle -ne 0 -and $_.MainWindowTitle -like 'DialtoneTyping*' }).Count",
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	text := strings.TrimSpace(bytes.NewBuffer(out).String())
	if text == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(text)
	if err != nil {
		return 0, err
	}
	return n, nil
}

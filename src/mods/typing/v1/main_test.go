package main

import (
	"os"
	"strings"
	"testing"
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

	path := repoRoot + "/src/mods/typing/v1/launch_local_terminal.ps1"
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

package wslv3

import (
	"strings"
	"testing"
)

func TestTerminalWindowTitle(t *testing.T) {
	got := terminalWindowTitle(" Ubuntu-24.04 ")
	if got != "Dialtone WSL - Ubuntu-24.04" {
		t.Fatalf("unexpected title: %q", got)
	}
}

func TestTerminalBootstrapScript(t *testing.T) {
	got := terminalBootstrapScriptWithConfig("Ubuntu-24.04", terminalBootstrapConfig{
		RepoRoot:      "/home/user/dialtone",
		ChromeHost:    "legion",
		ChromeRole:    "dev",
		ChromeEnabled: true,
		TerminalTMUX:  "dialtone",
		CADEnabled:    true,
		CADTMUX:       "dialtone-cad",
		CADPort:       8081,
	})
	wantParts := []string{
		"Dialtone WSL terminal",
		"Repo: %s",
		"Distro: %s",
		"tmux session: %s",
		"CAD session: %s (http://127.0.0.1:%s)",
		"Run ./dialtone.sh to enter the dialtone> repl.",
		"Commands sent with .\\dialtone.ps1 tmux send land in this exact tmux session.",
		"Type exit to close this terminal.",
		"CAD stays alive in a dedicated tmux session",
		"curl -fsS http://127.0.0.1:8081/health",
		"tmux attach -t %s",
		"terminal_session='dialtone'",
		"cad_session='dialtone-cad'",
		"cad_port=8081",
		"chrome_host='legion'",
		"chrome_role='dev'",
		"Chrome warmup target: %s role=%s",
		"Chrome warmup log: %s",
		"CAD warmup started in tmux session",
		"Terminal is ready in the repo root and attached to the shared tmux session.",
		"tmux new-session -d -s \"$terminal_session\"",
		"wsl-terminal-ubuntu-24-04-dialtone-init.sh",
		"dialtone.ps1 tmux status -Session %s -Distro %s -Cwd %s",
	}
	for _, part := range wantParts {
		if !strings.Contains(got, part) {
			t.Fatalf("bootstrap script missing %q: %s", part, got)
		}
	}
}

func TestTerminalAttachArgs(t *testing.T) {
	got := strings.Join(terminalAttachArgs("Ubuntu-24.04", terminalBootstrapConfig{
		RepoRoot:     "/home/user/dialtone",
		TerminalTMUX: "dialtone",
	}), "\n")
	wantParts := []string{
		"-d",
		"Ubuntu-24.04",
		"--cd",
		"/home/user/dialtone",
		"tmux",
		"attach-session",
		"-t",
		"dialtone",
	}
	for _, part := range wantParts {
		if !strings.Contains(got, part) {
			t.Fatalf("attach args missing %q: %s", part, got)
		}
	}
}

func TestShellSingleQuoteEscapesSingleQuotes(t *testing.T) {
	got := shellSingleQuote("dev'user")
	if got != `'dev'"'"'user'` {
		t.Fatalf("unexpected quoted string: %q", got)
	}
}

func TestTerminalChromeWarmupLogName(t *testing.T) {
	got := terminalChromeWarmupLogName("Legion Host", "dev/role")
	if got != "wsl-terminal-chrome-legion-host-dev-role.log" {
		t.Fatalf("unexpected warmup log name: %q", got)
	}
}

func TestTerminalChromeWarmupStampName(t *testing.T) {
	got := terminalChromeWarmupStampName("Legion Host", "dev/role")
	if got != "wsl-terminal-chrome-legion-host-dev-role.stamp" {
		t.Fatalf("unexpected warmup stamp name: %q", got)
	}
}

func TestTerminalChromeWarmupScript(t *testing.T) {
	got := terminalChromeWarmupScript(terminalBootstrapConfig{
		RepoRoot:   "/home/user/dialtone",
		ChromeHost: "legion",
		ChromeRole: "dev",
	})
	wantParts := []string{
		"wsl-terminal-chrome-legion-dev.log",
		"wsl-terminal-chrome-legion-dev.stamp",
		"./dialtone.sh chrome src_v3 deploy --host",
		"--service",
		"cd '/home/user/dialtone'",
	}
	for _, part := range wantParts {
		if !strings.Contains(got, part) {
			t.Fatalf("warmup script missing %q: %s", part, got)
		}
	}
}

func TestWindowsPathToWSLPath(t *testing.T) {
	got := windowsPathToWSLPath(`C:\Users\timca\dialtone`)
	if got != "/mnt/c/Users/timca/dialtone" {
		t.Fatalf("unexpected WSL path: %q", got)
	}
}

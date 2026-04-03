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
		RepoRoot:   "/home/user/dialtone",
		ChromeHost: "legion",
		ChromeRole: "dev",
	})
	wantParts := []string{
		"Dialtone WSL terminal",
		"Repo: %s",
		"Run ./dialtone.sh to enter the dialtone> repl.",
		"Type exit to close this terminal.",
		"chrome_host='legion'",
		"chrome_role='dev'",
		"./dialtone.sh chrome src_v3 deploy --host",
		"--service",
		"Chrome warmup queued on",
		"exec bash -li",
	}
	for _, part := range wantParts {
		if !strings.Contains(got, part) {
			t.Fatalf("bootstrap script missing %q: %s", part, got)
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

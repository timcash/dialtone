package tmuxcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBinaryPrefersDialtoneEnv(t *testing.T) {
	t.Setenv("DIALTONE_TMUX_BIN", "/usr/local/bin/tmux-host")
	if got := Binary(); got != "/usr/local/bin/tmux-host" {
		t.Fatalf("unexpected tmux binary: %q", got)
	}
}

func TestArgsIncludeRepoRootConfigWhenPresent(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, ".tmux.conf")
	if err := os.WriteFile(configPath, []byte("set -g mouse on\n"), 0o644); err != nil {
		t.Fatalf("write .tmux.conf: %v", err)
	}
	got := Args(repoRoot, "list-sessions")
	want := []string{"-f", configPath, "list-sessions"}
	if len(got) != len(want) {
		t.Fatalf("unexpected arg count: %#v", got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("unexpected args: %#v", got)
		}
	}
}

func TestShellCommandIncludesRepoRootConfig(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, ".tmux.conf")
	if err := os.WriteFile(configPath, []byte("set -g mouse on\n"), 0o644); err != nil {
		t.Fatalf("write .tmux.conf: %v", err)
	}
	command := ShellCommand(repoRoot, "new-session", "-A", "-s", "codex-view")
	if !strings.Contains(command, configPath) {
		t.Fatalf("expected shell command to include config path, got %q", command)
	}
	if !strings.Contains(command, "codex-view") {
		t.Fatalf("expected shell command to include session name, got %q", command)
	}
}

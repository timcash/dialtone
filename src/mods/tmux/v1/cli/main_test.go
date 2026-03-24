package main

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestTmuxBinaryPrefersDialtoneEnv(t *testing.T) {
	t.Setenv("DIALTONE_TMUX_BIN", "/usr/local/bin/tmux-host")
	if got := tmuxBinary(); got != "/usr/local/bin/tmux-host" {
		t.Fatalf("unexpected tmux binary: %q", got)
	}
}

func TestTmuxBinaryFallsBackToTmux(t *testing.T) {
	t.Setenv("DIALTONE_TMUX_BIN", "")
	if got := tmuxBinary(); got != "tmux" {
		t.Fatalf("unexpected fallback tmux binary: %q", got)
	}
}

func TestPersistedTargetRoundTripUsesSQLiteState(t *testing.T) {
	repoRoot := t.TempDir()
	if err := storePersistedTarget(repoRoot, "codex-view:0:0"); err != nil {
		t.Fatalf("storePersistedTarget returned error: %v", err)
	}
	value, err := loadPersistedTarget(repoRoot)
	if err != nil {
		t.Fatalf("loadPersistedTarget returned error: %v", err)
	}
	if value != "codex-view:0:0" {
		t.Fatalf("unexpected persisted target: %q", value)
	}
}

func TestClearPersistedTargetRemovesSQLiteState(t *testing.T) {
	repoRoot := t.TempDir()
	if err := storePersistedTarget(repoRoot, "codex-view:0:0"); err != nil {
		t.Fatalf("storePersistedTarget returned error: %v", err)
	}
	if err := clearPersistedTarget(repoRoot); err != nil {
		t.Fatalf("clearPersistedTarget returned error: %v", err)
	}
	_, err := loadPersistedTarget(repoRoot)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist after clear, got %v", err)
	}
}

func TestPersistedPromptTargetRoundTripUsesSQLiteState(t *testing.T) {
	repoRoot := t.TempDir()
	if err := storePersistedPromptTarget(repoRoot, "codex-view:0:0"); err != nil {
		t.Fatalf("storePersistedPromptTarget returned error: %v", err)
	}
	value, err := loadPersistedPromptTarget(repoRoot)
	if err != nil {
		t.Fatalf("loadPersistedPromptTarget returned error: %v", err)
	}
	if value != "codex-view:0:0" {
		t.Fatalf("unexpected persisted prompt target: %q", value)
	}
}

func TestClearPersistedPromptTargetRemovesSQLiteState(t *testing.T) {
	repoRoot := t.TempDir()
	if err := storePersistedPromptTarget(repoRoot, "codex-view:0:0"); err != nil {
		t.Fatalf("storePersistedPromptTarget returned error: %v", err)
	}
	if err := clearPersistedPromptTarget(repoRoot); err != nil {
		t.Fatalf("clearPersistedPromptTarget returned error: %v", err)
	}
	_, err := loadPersistedPromptTarget(repoRoot)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist after prompt clear, got %v", err)
	}
}

func TestBoolEnvValue(t *testing.T) {
	if got := boolEnvValue(true); got != "1" {
		t.Fatalf("unexpected true env value: %q", got)
	}
	if got := boolEnvValue(false); got != "0" {
		t.Fatalf("unexpected false env value: %q", got)
	}
}

func TestRunShellRejectsPositionalArgs(t *testing.T) {
	err := runShell([]string{"extra"})
	if err == nil {
		t.Fatalf("expected positional args to be rejected")
	}
	if !strings.Contains(err.Error(), "shell does not accept positional arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTmuxCLIUsageIncludesContractAndRuntimeCommands(t *testing.T) {
	output := captureTmuxStdout(t, printUsage)
	for _, want := range []string{"install", "build", "format", "test", "read", "write", "shell", "target"} {
		if !strings.Contains(output, want) {
			t.Fatalf("usage missing %q: %s", want, output)
		}
	}
}

func TestTmuxParseFormatArgsKeepsBlankDirEmpty(t *testing.T) {
	got, err := parseFormatArgs(nil)
	if err != nil {
		t.Fatalf("parseFormatArgs returned error: %v", err)
	}
	if got != "" {
		t.Fatalf("parseFormatArgs(nil) = %q, want empty string so format defaults to src/mods/tmux/v1", got)
	}
}

func captureTmuxStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()
	done := make(chan string, 1)
	go func() {
		var buf [4096]byte
		var out strings.Builder
		for {
			n, readErr := r.Read(buf[:])
			if n > 0 {
				out.Write(buf[:n])
			}
			if readErr != nil {
				done <- out.String()
				return
			}
		}
	}()
	fn()
	_ = w.Close()
	return <-done
}

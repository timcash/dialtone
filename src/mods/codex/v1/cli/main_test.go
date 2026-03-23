package main

import (
	"errors"
	"strings"
	"testing"
)

func TestBuildStartCommandUsesRepoShellAndInlineCodexLaunch(t *testing.T) {
	cmd := buildStartCommand("/tmp/dialtone", "default", "medium", "gpt-5.4")
	if !strings.Contains(cmd, "cd '/tmp/dialtone'") {
		t.Fatalf("missing repo cd: %q", cmd)
	}
	if !strings.Contains(cmd, "develop '.#default'") {
		t.Fatalf("missing nix develop shell: %q", cmd)
	}
	if !strings.Contains(cmd, "clear; printf") {
		t.Fatalf("missing clear before startup banner: %q", cmd)
	}
	if !strings.Contains(cmd, "command -v codex") {
		t.Fatalf("missing codex lookup: %q", cmd)
	}
	if !strings.Contains(cmd, "npx --yes @openai/codex") {
		t.Fatalf("missing npx fallback: %q", cmd)
	}
	if !strings.Contains(cmd, "exec env CI=1 codex -c") {
		t.Fatalf("missing codex exec prefix: %q", cmd)
	}
	if !strings.Contains(cmd, "check_for_update_on_startup=false") {
		t.Fatalf("missing update-check override: %q", cmd)
	}
	if !strings.Contains(cmd, "-m '\"'\"'gpt-5.4'\"'\"' -a never -s danger-full-access") {
		t.Fatalf("missing model and sandbox args: %q", cmd)
	}
}

func TestShellQuoteEscapesSingleQuotes(t *testing.T) {
	quoted := shellQuote("/tmp/dialtone's")
	if quoted != "'/tmp/dialtone'\"'\"'s'" {
		t.Fatalf("unexpected quote result: %q", quoted)
	}
}

func TestBuildCodexExecCommandIncludesFallback(t *testing.T) {
	cmd := buildCodexExecCommand("gpt-5.4")
	if !strings.Contains(cmd, "-c 'check_for_update_on_startup=false'") {
		t.Fatalf("missing update-check override: %q", cmd)
	}
	if !strings.Contains(cmd, "exec env CI=1 codex") {
		t.Fatalf("missing direct codex exec: %q", cmd)
	}
	if !strings.Contains(cmd, "exec env CI=1 npx --yes @openai/codex") {
		t.Fatalf("missing npx exec: %q", cmd)
	}
}

func TestNormalizePaneTargetAcceptsColonAndDotForms(t *testing.T) {
	if got := normalizePaneTarget("codex-view:0:1"); got != "codex-view:0.1" {
		t.Fatalf("unexpected normalized colon target: %q", got)
	}
	if got := normalizePaneTarget("codex-view:0.1"); got != "codex-view:0.1" {
		t.Fatalf("unexpected normalized dot target: %q", got)
	}
}

func TestTmuxOutputWithRetryRetriesTransientServerExit(t *testing.T) {
	t.Cleanup(func() {
		tmuxOutputRunner = tmuxOutput
		tmuxRetrySleep = defaultTmuxRetrySleep
	})

	attempts := 0
	tmuxOutputRunner = func(args ...string) (string, error) {
		attempts++
		if attempts == 1 {
			return "", errors.New("tmux clear-history -t codex-view:0.0 failed: server exited unexpectedly")
		}
		return "ok", nil
	}
	tmuxRetrySleep = func() {}

	out, err := tmuxOutputWithRetry("clear-history", "-t", "codex-view:0.0")
	if err != nil {
		t.Fatalf("tmuxOutputWithRetry returned error: %v", err)
	}
	if out != "ok" {
		t.Fatalf("unexpected output: %q", out)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestTmuxOutputWithRetryReturnsNonTransientErrorImmediately(t *testing.T) {
	t.Cleanup(func() {
		tmuxOutputRunner = tmuxOutput
		tmuxRetrySleep = defaultTmuxRetrySleep
	})

	attempts := 0
	wantErr := errors.New("tmux clear-history -t codex-view:0.0 failed: can't find pane")
	tmuxOutputRunner = func(args ...string) (string, error) {
		attempts++
		return "", wantErr
	}
	tmuxRetrySleep = func() {}

	_, err := tmuxOutputWithRetry("clear-history", "-t", "codex-view:0.0")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped original error, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestRespawnPaneIgnoresClearHistoryFailureWhenRespawnSucceeds(t *testing.T) {
	t.Cleanup(func() {
		tmuxOutputRunner = tmuxOutput
		tmuxRetrySleep = defaultTmuxRetrySleep
	})

	ops := make([]string, 0, 2)
	tmuxOutputRunner = func(args ...string) (string, error) {
		ops = append(ops, strings.Join(args, " "))
		switch args[0] {
		case "clear-history":
			return "", errors.New("tmux clear-history -t codex-view:0.0 failed: can't find pane")
		case "respawn-pane":
			return "", nil
		default:
			return "", nil
		}
	}
	tmuxRetrySleep = func() {}

	if err := respawnPane("codex-view:0.0", "printf ok"); err != nil {
		t.Fatalf("respawnPane returned error: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("expected 2 tmux operations, got %d: %+v", len(ops), ops)
	}
	if !strings.HasPrefix(ops[0], "clear-history -t codex-view:0.0") {
		t.Fatalf("unexpected first tmux operation: %q", ops[0])
	}
	if !strings.HasPrefix(ops[1], "respawn-pane -k -t codex-view:0.0") {
		t.Fatalf("unexpected second tmux operation: %q", ops[1])
	}
}

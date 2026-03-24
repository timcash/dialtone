package main

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestCodexTestPackagesCoverDirectControlPlaneContract(t *testing.T) {
	got := codexTestPackages()
	want := []string{
		"./mods/codex/v1/...",
		"./mods/shared/dispatch",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected codex test packages\nwant:\n%s\n\ngot:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
}

func TestBuildStartCommandUsesRepoShellAndInlineCodexLaunch(t *testing.T) {
	cmd := buildStartCommand("/tmp/dialtone", "default", "medium", "gpt-5.4")
	if !strings.Contains(cmd, "cd '/tmp/dialtone'") {
		t.Fatalf("missing repo cd: %q", cmd)
	}
	if !strings.Contains(cmd, "develop '.#default'") {
		t.Fatalf("missing nix develop shell: %q", cmd)
	}
	if !strings.Contains(cmd, "DIALTONE_NIX_SHELL_BANNER=0") {
		t.Fatalf("missing shell banner suppression: %q", cmd)
	}
	if !strings.Contains(cmd, "--no-warn-dirty") {
		t.Fatalf("missing dirty-tree warning suppression: %q", cmd)
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
	if !strings.Contains(cmd, "model_reasoning_effort=\"medium\"") {
		t.Fatalf("missing requested model reasoning effort: %q", cmd)
	}
	if !strings.Contains(cmd, "plan_mode_reasoning_effort=\"medium\"") {
		t.Fatalf("missing requested plan mode reasoning effort: %q", cmd)
	}
	if !strings.Contains(cmd, "-m '\"'\"'gpt-5.4'\"'\"' -a never -s danger-full-access") {
		t.Fatalf("missing model and sandbox args: %q", cmd)
	}
}

func TestCodexCLIUsageIncludesContractAndRuntimeCommands(t *testing.T) {
	output := captureCodexStdout(t, printUsage)
	for _, want := range []string{"install", "build", "format", "test", "start", "status"} {
		if !strings.Contains(output, want) {
			t.Fatalf("usage missing %q: %s", want, output)
		}
	}
	if !strings.Contains(output, "Run go test for codex v1 plus direct-routing helpers") {
		t.Fatalf("usage missing strengthened codex test description: %s", output)
	}
}

func TestCodexParseFormatArgsKeepsBlankDirEmpty(t *testing.T) {
	got, err := parseFormatArgs(nil)
	if err != nil {
		t.Fatalf("parseFormatArgs returned error: %v", err)
	}
	if got != "" {
		t.Fatalf("parseFormatArgs(nil) = %q, want empty string so format defaults to src/mods/codex/v1", got)
	}
}

func TestShellQuoteEscapesSingleQuotes(t *testing.T) {
	quoted := shellQuote("/tmp/dialtone's")
	if quoted != "'/tmp/dialtone'\"'\"'s'" {
		t.Fatalf("unexpected quote result: %q", quoted)
	}
}

func TestBuildCodexExecCommandIncludesFallback(t *testing.T) {
	cmd := buildCodexExecCommand("gpt-5.4", "medium")
	if !strings.Contains(cmd, "-c 'check_for_update_on_startup=false'") {
		t.Fatalf("missing update-check override: %q", cmd)
	}
	if !strings.Contains(cmd, "-c 'model_reasoning_effort=\"medium\"'") {
		t.Fatalf("missing model reasoning effort override: %q", cmd)
	}
	if !strings.Contains(cmd, "-c 'plan_mode_reasoning_effort=\"medium\"'") {
		t.Fatalf("missing plan mode reasoning effort override: %q", cmd)
	}
	if !strings.Contains(cmd, "exec env CI=1 codex") {
		t.Fatalf("missing direct codex exec: %q", cmd)
	}
	if !strings.Contains(cmd, "exec env CI=1 npx --yes @openai/codex") {
		t.Fatalf("missing npx exec: %q", cmd)
	}
}

func TestBuildCodexExecCommandDefaultsBlankReasoningToMedium(t *testing.T) {
	cmd := buildCodexExecCommand("gpt-5.4", "")
	if !strings.Contains(cmd, "model_reasoning_effort=\"medium\"") {
		t.Fatalf("expected blank reasoning to default to medium: %q", cmd)
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

func captureCodexStdout(t *testing.T, fn func()) string {
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

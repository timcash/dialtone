package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dialtone/dev/internal/modstate"
	"dialtone/dev/mods/shared/sqlitestate"
)

func TestRunStartRejectsPositionalArgs(t *testing.T) {
	err := runStart([]string{"extra"})
	if err == nil {
		t.Fatalf("expected positional args to be rejected")
	}
	if !strings.Contains(err.Error(), "does not accept positional arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunStartRejectsBlankSession(t *testing.T) {
	err := runStart([]string{"--session", "   "})
	if err == nil {
		t.Fatalf("expected blank session to be rejected")
	}
	if !strings.Contains(err.Error(), "--session is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunStartRejectsNonPositiveWaitSeconds(t *testing.T) {
	err := runStart([]string{"--wait-seconds", "0"})
	if err == nil {
		t.Fatalf("expected non-positive wait-seconds to be rejected")
	}
	if !strings.Contains(err.Error(), "--wait-seconds must be positive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSplitVerticalRejectsPositionalArgs(t *testing.T) {
	err := runSplitVertical([]string{"extra"})
	if err == nil {
		t.Fatalf("expected positional args to be rejected")
	}
	if !strings.Contains(err.Error(), "does not accept positional arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSplitVerticalRejectsBlankSession(t *testing.T) {
	err := runSplitVertical([]string{"--session", "   "})
	if err == nil {
		t.Fatalf("expected blank session to be rejected")
	}
	if !strings.Contains(err.Error(), "--session is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSplitVerticalRejectsNonPositiveWaitSeconds(t *testing.T) {
	err := runSplitVertical([]string{"--wait-seconds", "0"})
	if err == nil {
		t.Fatalf("expected non-positive wait-seconds to be rejected")
	}
	if !strings.Contains(err.Error(), "--wait-seconds must be positive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPromptRejectsMissingText(t *testing.T) {
	err := runPrompt(nil)
	if err == nil {
		t.Fatalf("expected missing prompt text to be rejected")
	}
	if !strings.Contains(err.Error(), "prompt requires text") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunDemoProtocolRejectsPositionalArgs(t *testing.T) {
	err := runDemoProtocol([]string{"extra"})
	if err == nil {
		t.Fatalf("expected positional args to be rejected")
	}
	if !strings.Contains(err.Error(), "does not accept positional arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunDemoProtocolRejectsNonPositiveWaitSeconds(t *testing.T) {
	err := runDemoProtocol([]string{"--wait-seconds", "0"})
	if err == nil {
		t.Fatalf("expected non-positive wait-seconds to be rejected")
	}
	if !strings.Contains(err.Error(), "--wait-seconds must be positive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClassifyQueueDetectsLoopingAndStuck(t *testing.T) {
	now := time.Now().UTC()
	looping := []modstate.QueueRecord{
		{CommandText: "mods v1 db sync", Target: "codex-view:0:1", Status: "done", CreatedAt: now.Format(time.RFC3339)},
		{CommandText: "mods v1 db sync", Target: "codex-view:0:1", Status: "done", CreatedAt: now.Add(-time.Second).Format(time.RFC3339)},
		{CommandText: "mods v1 db sync", Target: "codex-view:0:1", Status: "done", CreatedAt: now.Add(-2 * time.Second).Format(time.RFC3339)},
	}
	if got := classifyQueue(looping, 30*time.Second); got != "looping" {
		t.Fatalf("expected looping classification, got %q", got)
	}
	stuck := []modstate.QueueRecord{
		{CommandText: "mods v1 db test-run", Target: "codex-view:0:1", Status: "running", StartedAt: now.Add(-time.Minute).Format(time.RFC3339)},
	}
	if got := classifyQueue(stuck, 10*time.Second); got != "stuck" {
		t.Fatalf("expected stuck classification, got %q", got)
	}
}

func TestLoadStateTargetReadsSQLiteState(t *testing.T) {
	repoRoot := t.TempDir()
	dbPath := filepath.Join(repoRoot, ".dialtone", "state.sqlite")
	t.Setenv("DIALTONE_STATE_DB", dbPath)
	db, err := modstate.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.TmuxPromptTargetKey, "codex-view:0:0"); err != nil {
		t.Fatalf("UpsertStateValue returned error: %v", err)
	}
	value, err := loadStateTarget(repoRoot, sqlitestate.TmuxPromptTargetKey)
	if err != nil {
		t.Fatalf("loadStateTarget returned error: %v", err)
	}
	if value != "codex-view:0:0" {
		t.Fatalf("unexpected state target value: %q", value)
	}
}

func TestWaitForPaneTimesOutWithContext(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "dialtone_mod"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write dialtone_mod stub: %v", err)
	}
	err := waitForPane(tmp, "codex-view:0:0", 10*time.Millisecond)
	if err == nil {
		t.Fatalf("expected waitForPane to time out")
	}
	if !strings.Contains(err.Error(), "codex-view:0:0") {
		t.Fatalf("missing pane in error: %v", err)
	}
}

func TestRunDialtoneModQuietCapturesOutput(t *testing.T) {
	tmp := t.TempDir()
	script := "#!/bin/sh\nprintf 'ok from stub\\n'\n"
	if err := os.WriteFile(filepath.Join(tmp, "dialtone_mod"), []byte(script), 0o755); err != nil {
		t.Fatalf("write dialtone_mod stub: %v", err)
	}
	out, err := runDialtoneModQuiet(tmp, "ghostty", "v1", "list")
	if err != nil {
		t.Fatalf("runDialtoneModQuiet returned error: %v", err)
	}
	if strings.TrimSpace(out) != "ok from stub" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRunDialtoneModQuietIncludesStdoutInError(t *testing.T) {
	tmp := t.TempDir()
	script := "#!/bin/sh\nprintf 'stdout text\\n'\nprintf 'stderr text\\n' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(tmp, "dialtone_mod"), []byte(script), 0o755); err != nil {
		t.Fatalf("write dialtone_mod stub: %v", err)
	}
	_, err := runDialtoneModQuiet(tmp, "tmux", "v1", "list")
	if err == nil {
		t.Fatalf("expected runDialtoneModQuiet to fail")
	}
	text := err.Error()
	if !strings.Contains(text, "stdout text") || !strings.Contains(text, "stderr text") {
		t.Fatalf("expected stdout and stderr in error, got %q", text)
	}
}

func TestPaneExistsReflectsDialtoneModResult(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "dialtone_mod"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write dialtone_mod stub: %v", err)
	}
	if !paneExists(tmp, "codex-view:0:1") {
		t.Fatalf("expected paneExists to return true")
	}

	if err := os.WriteFile(filepath.Join(tmp, "dialtone_mod"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("rewrite dialtone_mod stub: %v", err)
	}
	if paneExists(tmp, "codex-view:0:1") {
		t.Fatalf("expected paneExists to return false")
	}
}

func TestLocateRepoRootFindsAncestor(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "mods.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write mods.go: %v", err)
	}
	nested := filepath.Join(root, "src", "mods", "shell", "v1")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	restore := mustChdir(t, nested)
	defer restore()

	got, err := locateRepoRoot()
	if err != nil {
		t.Fatalf("locateRepoRoot returned error: %v", err)
	}
	gotPath, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("eval symlinks for got path: %v", err)
	}
	wantPath, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("eval symlinks for root path: %v", err)
	}
	if gotPath != wantPath {
		t.Fatalf("expected repo root %q, got %q", wantPath, gotPath)
	}
}

func TestLocateRepoRootFailsOutsideRepo(t *testing.T) {
	restore := mustChdir(t, t.TempDir())
	defer restore()

	_, err := locateRepoRoot()
	if err == nil {
		t.Fatalf("expected locateRepoRoot to fail")
	}
	if !strings.Contains(err.Error(), "cannot locate repo root") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func mustChdir(t *testing.T, dir string) func() {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %q: %v", dir, err)
	}
	return func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore cwd %q: %v", cwd, err)
		}
	}
}

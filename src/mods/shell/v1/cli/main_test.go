package main

import (
	"encoding/json"
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

func TestRunEnqueueCommandRejectsMissingText(t *testing.T) {
	err := runEnqueueCommand(nil)
	if err == nil {
		t.Fatalf("expected missing command text to be rejected")
	}
	if !strings.Contains(err.Error(), "enqueue-command requires command text") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRunRejectsMissingText(t *testing.T) {
	err := runRun(nil)
	if err == nil {
		t.Fatalf("expected missing command text to be rejected")
	}
	if !strings.Contains(err.Error(), "run requires command text") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunTestRejectsPositionalArgs(t *testing.T) {
	err := runTest([]string{"extra"})
	if err == nil {
		t.Fatalf("expected positional args to be rejected")
	}
	if !strings.Contains(err.Error(), "test does not accept positional arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunTestBasicRejectsPositionalArgs(t *testing.T) {
	err := runTestBasic([]string{"extra"})
	if err == nil {
		t.Fatalf("expected positional args to be rejected")
	}
	if !strings.Contains(err.Error(), "test-basic does not accept positional arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunTestAllRejectsPositionalArgs(t *testing.T) {
	err := runTestAll([]string{"extra"})
	if err == nil {
		t.Fatalf("expected positional args to be rejected")
	}
	if !strings.Contains(err.Error(), "test-all does not accept positional arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWorkflowRejectsPositionalArgs(t *testing.T) {
	err := runWorkflow([]string{"extra"})
	if err == nil {
		t.Fatalf("expected positional args to be rejected")
	}
	if !strings.Contains(err.Error(), "workflow does not accept positional arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildShellGoTestCommandTargetsShellModule(t *testing.T) {
	got := buildGoTestCommand("/Users/user/dialtone", "shell", "v1")
	want := "clear && cd /Users/user/dialtone/src && go test ./mods/shell/v1/..."
	if got != want {
		t.Fatalf("unexpected command\nwant: %q\ngot:  %q", want, got)
	}
}

func TestBuildBasicTestCommandUsesCorePackages(t *testing.T) {
	got := buildBasicTestCommand("/Users/user/dialtone")
	want := "clear && cd /Users/user/dialtone/src && go test ./internal/modstate ./mods/shared/sqlitestate ./mods/mod/v1 ./mods/shell/v1/cli"
	if got != want {
		t.Fatalf("unexpected basic test command\nwant: %q\ngot:  %q", want, got)
	}
}

func TestBuildAllModsTestCommandUsesRecursiveModSweep(t *testing.T) {
	got := buildAllModsTestCommand("/Users/user/dialtone")
	want := "clear && cd /Users/user/dialtone/src && go test ./mods/..."
	if got != want {
		t.Fatalf("unexpected all-mods test command\nwant: %q\ngot:  %q", want, got)
	}
}

func TestVisibleGoTestCommandsReuseExistingNixShell(t *testing.T) {
	commands := []string{
		buildGoTestCommand("/Users/user/dialtone", "shell", "v1"),
		buildAllModsTestCommand("/Users/user/dialtone"),
	}
	for _, command := range commands {
		if strings.Contains(command, "nix develop") {
			t.Fatalf("visible dialtone-view command should reuse the existing nix shell, got %q", command)
		}
		if !strings.Contains(command, "go test") {
			t.Fatalf("visible command should run go test directly, got %q", command)
		}
	}
}

func TestBasicTestCommandStaysOutsideDialtoneViewShellBus(t *testing.T) {
	command := buildBasicTestCommand("/Users/user/dialtone")
	if strings.Contains(command, "nix develop") {
		t.Fatalf("basic test command should be the in-shell command only, got %q", command)
	}
	if !strings.Contains(command, "clear && cd /Users/user/dialtone/src && go test ./internal/modstate ./mods/shared/sqlitestate ./mods/mod/v1 ./mods/shell/v1/cli") {
		t.Fatalf("unexpected basic test command: %q", command)
	}
}

func TestShouldUseBackendCLIForShellInternalMods(t *testing.T) {
	cases := []struct {
		mod     string
		version string
		want    bool
	}{
		{mod: "ghostty", version: "v1", want: true},
		{mod: "tmux", version: "v1", want: true},
		{mod: "codex", version: "v1", want: true},
		{mod: "ssh", version: "v1", want: false},
		{mod: "mods", version: "v1", want: false},
	}
	for _, tc := range cases {
		if got := shouldUseBackendCLI(tc.mod, tc.version); got != tc.want {
			t.Fatalf("shouldUseBackendCLI(%q, %q) = %v, want %v", tc.mod, tc.version, got, tc.want)
		}
	}
}

func TestBackendCLIEntryUsesStandardizedPath(t *testing.T) {
	got := backendCLIEntry("tmux", "v1")
	want := "./mods/tmux/v1/cli"
	if got != want {
		t.Fatalf("unexpected backend cli entry\nwant: %q\ngot:  %q", want, got)
	}
}

func TestBackendCLIAvailableRequiresRepoLayout(t *testing.T) {
	repoRoot := t.TempDir()
	if backendCLIAvailable(repoRoot, "tmux", "v1") {
		t.Fatalf("expected backend cli to be unavailable without repo layout")
	}
	path := filepath.Join(repoRoot, "src", "mods", "tmux", "v1", "cli", "main.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir backend path: %v", err)
	}
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write backend main.go: %v", err)
	}
	if !backendCLIAvailable(repoRoot, "tmux", "v1") {
		t.Fatalf("expected backend cli to be available with repo layout")
	}
}

func TestDefaultDialtoneShellNameUsesDefaultNixShell(t *testing.T) {
	if got := defaultDialtoneShellName(); got != "default" {
		t.Fatalf("unexpected dialtone shell default: %q", got)
	}
}

func TestRunReadRejectsPositionalArgs(t *testing.T) {
	err := runRead([]string{"extra"})
	if err == nil {
		t.Fatalf("expected positional args to be rejected")
	}
	if !strings.Contains(err.Error(), "read does not accept positional arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunReadRejectsUnknownRole(t *testing.T) {
	err := runRead([]string{"--role", "bogus"})
	if err == nil {
		t.Fatalf("expected invalid role to be rejected")
	}
	if !strings.Contains(err.Error(), "--role must be prompt or command") {
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

func TestRunSyncOnceRejectsPositionalArgs(t *testing.T) {
	err := runSyncOnce([]string{"extra"})
	if err == nil {
		t.Fatalf("expected positional args to be rejected")
	}
	if !strings.Contains(err.Error(), "does not accept positional arguments") {
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

func TestLoadStateTargetRecoversPromptTargetFromObservedPane(t *testing.T) {
	repoRoot := t.TempDir()
	dbPath := filepath.Join(repoRoot, ".dialtone", "state.sqlite")
	t.Setenv("DIALTONE_STATE_DB", dbPath)
	db, err := modstate.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()
	if _, err := modstate.AppendShellBusObserved(db, "tmux", "pane", "snapshot", "shell-sync", "codex-view", "codex-view:0:0", 0, `{"summary":"left"}`); err != nil {
		t.Fatalf("AppendShellBusObserved returned error: %v", err)
	}
	value, err := loadStateTarget(repoRoot, sqlitestate.TmuxPromptTargetKey)
	if err != nil {
		t.Fatalf("loadStateTarget returned error: %v", err)
	}
	if value != "codex-view:0:0" {
		t.Fatalf("unexpected recovered prompt target: %q", value)
	}
}

func TestLoadStateTargetRecoversCommandTargetFromObservedPane(t *testing.T) {
	repoRoot := t.TempDir()
	dbPath := filepath.Join(repoRoot, ".dialtone", "state.sqlite")
	t.Setenv("DIALTONE_STATE_DB", dbPath)
	db, err := modstate.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()
	if _, err := modstate.AppendShellBusObserved(db, "tmux", "pane", "snapshot", "shell-sync", "codex-view", "codex-view:0:1", 0, `{"summary":"right"}`); err != nil {
		t.Fatalf("AppendShellBusObserved returned error: %v", err)
	}
	value, err := loadStateTarget(repoRoot, sqlitestate.TmuxTargetKey)
	if err != nil {
		t.Fatalf("loadStateTarget returned error: %v", err)
	}
	if value != "codex-view:0:1" {
		t.Fatalf("unexpected recovered command target: %q", value)
	}
}

func TestResolveReadPaneUsesRoleTargets(t *testing.T) {
	repoRoot := t.TempDir()
	dbPath := filepath.Join(repoRoot, ".dialtone", "state.sqlite")
	t.Setenv("DIALTONE_STATE_DB", dbPath)
	db, err := modstate.Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.TmuxPromptTargetKey, "codex-view:0:0"); err != nil {
		t.Fatalf("set prompt target: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.TmuxTargetKey, "codex-view:0:1"); err != nil {
		t.Fatalf("set command target: %v", err)
	}

	promptPane, err := resolveReadPane(repoRoot, "prompt", "")
	if err != nil {
		t.Fatalf("resolveReadPane prompt returned error: %v", err)
	}
	if promptPane != "codex-view:0:0" {
		t.Fatalf("unexpected prompt pane: %q", promptPane)
	}

	commandPane, err := resolveReadPane(repoRoot, "command", "")
	if err != nil {
		t.Fatalf("resolveReadPane command returned error: %v", err)
	}
	if commandPane != "codex-view:0:1" {
		t.Fatalf("unexpected command pane: %q", commandPane)
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

func TestSummarizeSnapshotUsesLastNonEmptyLine(t *testing.T) {
	got := summarizeSnapshot("first line\n\nlast line\n")
	if got != "last line" {
		t.Fatalf("unexpected summary: %q", got)
	}
}

func TestTrackedCommandExitStatus(t *testing.T) {
	status, ok := trackedCommandExitStatus("line one\nDIALTONE_CMD_DONE_42 exit=0\n")
	if !ok || status != 0 {
		t.Fatalf("expected successful tracked command exit status, got status=%d ok=%v", status, ok)
	}
	status, ok = trackedCommandExitStatus("line one\nDIALTONE_CMD_DONE_42 exit=17\n")
	if !ok || status != 17 {
		t.Fatalf("expected failing tracked command exit status, got status=%d ok=%v", status, ok)
	}
	status, ok = trackedCommandExitStatus("line one\nno sentinel here\n")
	if ok {
		t.Fatalf("expected missing sentinel to return ok=false, got status=%d ok=%v", status, ok)
	}
}

func TestLatestPaneSnapshotTextReadsObservedPayload(t *testing.T) {
	payload, err := json.Marshal(map[string]string{
		"text":    "line one\nline two\n",
		"summary": "line two",
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	rows := []modstate.ShellBusRecord{
		{
			Scope:    "observed",
			Subject:  "pane",
			Action:   "snapshot",
			Pane:     "codex-view:0:1",
			BodyJSON: string(payload),
		},
	}
	if got := latestPaneSnapshotText(rows, "codex-view:0:1"); got != "line one\nline two" {
		t.Fatalf("unexpected pane text: %q", got)
	}
}

func TestCapturePaneSnapshotWithReaderStoresObservedRow(t *testing.T) {
	db, err := modstate.Open(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()
	if err := capturePaneSnapshotWithReader(db, "codex-view", "codex-view:0:1", 7, func(target string) (string, error) {
		if target != "codex-view:0:1" {
			t.Fatalf("unexpected target: %q", target)
		}
		return "line one\nline two\n", nil
	}); err != nil {
		t.Fatalf("capturePaneSnapshotWithReader returned error: %v", err)
	}
	rows, err := modstate.LoadShellBus(db, "observed", 10)
	if err != nil {
		t.Fatalf("LoadShellBus returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].Pane != "codex-view:0:1" || rows[0].RefID != 7 {
		t.Fatalf("unexpected observed rows: %+v", rows)
	}
	if got := latestPaneSnapshotText(rows, "codex-view:0:1"); got != "line one\nline two" {
		t.Fatalf("unexpected stored pane text: %q", got)
	}
}

func TestMarkShellBusRowRunningUpdatesQueuedRow(t *testing.T) {
	db, err := modstate.Open(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()
	body := `{"command":"./dialtone_mod ssh v1 help","target":"codex-view:0:1"}`
	rowID, err := modstate.EnqueueShellBus(db, "shell", "desired", "command", "run", "shell-cli", "codex-view", "codex-view:0:1", body)
	if err != nil {
		t.Fatalf("EnqueueShellBus returned error: %v", err)
	}
	if err := markShellBusRowRunning(db, rowID, body); err != nil {
		t.Fatalf("markShellBusRowRunning returned error: %v", err)
	}
	record, ok, err := modstate.LoadShellBusRecord(db, rowID)
	if err != nil {
		t.Fatalf("LoadShellBusRecord returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected queued row to exist")
	}
	if record.Status != "running" {
		t.Fatalf("expected row status to be running, got %+v", record)
	}
	if record.BodyJSON != body {
		t.Fatalf("expected body json to be preserved, got %q", record.BodyJSON)
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

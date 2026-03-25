package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
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

func TestShellCLIUsageIncludesContractAndWorkflowCommands(t *testing.T) {
	output := captureShellStdout(t, printUsage)
	for _, want := range []string{"install", "build", "format", "test", "test-basic", "test-all", "start", "run", "serve", "ensure-worker", "state", "read", "events", "supervise"} {
		if !strings.Contains(output, want) {
			t.Fatalf("usage missing %q: %s", want, output)
		}
	}
}

func TestShellParseFormatArgsKeepsBlankDirEmpty(t *testing.T) {
	got, err := parseFormatArgs(nil)
	if err != nil {
		t.Fatalf("parseFormatArgs returned error: %v", err)
	}
	if got != "" {
		t.Fatalf("parseFormatArgs(nil) = %q, want empty string so format defaults to src/mods/shell/v1", got)
	}
}

func TestBuildShellGoTestCommandTargetsShellModule(t *testing.T) {
	got := buildGoTestCommand("/Users/user/dialtone", "shell", "v1")
	want := "clear && cd /Users/user/dialtone/src && go test ./mods/shell/v1/..."
	if got != want {
		t.Fatalf("unexpected command\nwant: %q\ngot:  %q", want, got)
	}
}

func TestShellTestPackagesCoversShellWorkflowStack(t *testing.T) {
	got := shellTestPackages()
	want := []string{
		"./internal/modcli",
		"./internal/modstate",
		"./mods/shared/dispatch",
		"./mods/shared/router",
		"./mods/shared/sqlitestate",
		"./mods/dialtone/v1",
		"./mods/codex/v1/...",
		"./mods/ghostty/v1/...",
		"./mods/shell/v1/...",
		"./mods/test/v1/...",
		"./mods/tmux/v1/...",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected shell test package set\nwant:\n%s\n\ngot:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
}

func TestBuildShellTestCommandUsesShellWorkflowPackageSet(t *testing.T) {
	got := buildShellTestCommand("/Users/user/dialtone")
	want := "clear && cd /Users/user/dialtone/src && go test ./internal/modcli ./internal/modstate ./mods/shared/dispatch ./mods/shared/router ./mods/shared/sqlitestate ./mods/dialtone/v1 ./mods/codex/v1/... ./mods/ghostty/v1/... ./mods/shell/v1/... ./mods/test/v1/... ./mods/tmux/v1/... && printf 'DIALTONE_SHELL_TEST_DONE\\n'"
	if got != want {
		t.Fatalf("unexpected shell test command\nwant: %q\ngot:  %q", want, got)
	}
}

func TestBuildBasicTestCommandUsesCorePackages(t *testing.T) {
	got := buildBasicTestCommand("/Users/user/dialtone")
	want := "clear && cd /Users/user/dialtone/src && go test ./internal/modstate ./mods/shared/sqlitestate ./mods/mod/v1 ./mods/shell/v1/cli"
	if got != want {
		t.Fatalf("unexpected basic test command\nwant: %q\ngot:  %q", want, got)
	}
}

func TestBuildShellWorkerStartCommandUsesServeInRepo(t *testing.T) {
	got := buildShellWorkerStartCommand("/Users/user/dialtone", "codex-view:0:1")
	want := "cd /Users/user/dialtone && ./dialtone_mod shell v1 serve --pane codex-view:0:1"
	if got != want {
		t.Fatalf("unexpected worker start command\nwant: %q\ngot:  %q", want, got)
	}
}

func TestBuildAllModsTestCommandUsesRecursiveModSweep(t *testing.T) {
	got := buildAllModsTestCommand("/Users/user/dialtone")
	want := "clear && cd /Users/user/dialtone/src && go test ./mods/..."
	if got != want {
		t.Fatalf("unexpected all-mods test command\nwant: %q\ngot:  %q", want, got)
	}
}

func TestBuildVisibleCommandEnvIncludesWorkerPaneOverride(t *testing.T) {
	env := buildVisibleCommandEnv([]string{"HOME=/tmp/home"}, "codex-view:0:1")
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "TERM=xterm-256color") {
		t.Fatalf("expected TERM override in visible command env: %q", joined)
	}
	if !strings.Contains(joined, "DIALTONE_TMUX_PROXY_ACTIVE=1") {
		t.Fatalf("expected tmux proxy marker in visible command env: %q", joined)
	}
	if !strings.Contains(joined, "DIALTONE_TMUX_TARGET=codex-view:0:1") {
		t.Fatalf("expected worker pane target in visible command env: %q", joined)
	}
}

func TestBuildVisibleCommandEnvSkipsBlankWorkerPaneOverride(t *testing.T) {
	env := buildVisibleCommandEnv([]string{"HOME=/tmp/home"}, "   ")
	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "DIALTONE_TMUX_TARGET=") {
		t.Fatalf("expected blank pane target to be omitted from visible command env: %q", joined)
	}
	if strings.Contains(joined, "DIALTONE_TMUX_PROXY_ACTIVE=1") {
		t.Fatalf("expected proxy marker to be omitted without a pane target: %q", joined)
	}
}

func TestBuildTmuxStartCommandUsesRepoRootConfig(t *testing.T) {
	got := buildTmuxStartCommand("/Users/user/dialtone", "codex-view")
	if !strings.Contains(got, "/Users/user/dialtone/.tmux.conf") {
		t.Fatalf("expected repo tmux config in command, got %q", got)
	}
	if !strings.Contains(got, "codex-view") {
		t.Fatalf("expected session name in command, got %q", got)
	}
}

func TestBuildCodexStartArgsTargetsPromptPane(t *testing.T) {
	got := buildCodexStartArgs("codex-view", "codex-view:0:0", "default", "medium", "gpt-5.4")
	want := []string{"codex", "v1", "start", "--session", "codex-view", "--pane", "codex-view:0:0", "--shell", "default", "--reasoning", "medium", "--model", "gpt-5.4"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected codex start args\nwant:\n%s\n\ngot:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
}

func TestVisibleGoTestCommandsReuseExistingNixShell(t *testing.T) {
	commands := []string{
		buildGoTestCommand("/Users/user/dialtone", "shell", "v1"),
		buildShellTestCommand("/Users/user/dialtone"),
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

func TestShellWorkflowStateReadyForVisibleCommands(t *testing.T) {
	state := shellWorkflowState{
		PromptTarget:       "codex-view:0:0",
		CommandTarget:      "codex-view:0:1",
		PromptPanePresent:  true,
		CommandPanePresent: true,
		WorkerHealthy:      true,
		WorkerPane:         "codex-view:0:1",
	}
	if !state.readyForVisibleCommands() {
		t.Fatalf("expected workflow state to be ready: %+v", state)
	}
	state.WorkerPane = "codex-view:0:9"
	if state.readyForVisibleCommands() {
		t.Fatalf("expected worker pane mismatch to make workflow unready: %+v", state)
	}
}

func TestInspectShellWorkflowStateReadsTargetsAndPanePresence(t *testing.T) {
	repoRoot, db := openShellWorkflowTestDB(t)
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.TmuxPromptTargetKey, "codex-view:0:0"); err != nil {
		t.Fatalf("set prompt target: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.TmuxTargetKey, "codex-view:0:1"); err != nil {
		t.Fatalf("set command target: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellWorkerStatusKey, "running"); err != nil {
		t.Fatalf("set worker status: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellWorkerPaneKey, "codex-view:0:1"); err != nil {
		t.Fatalf("set worker pane: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellWorkerHeartbeatKey, time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("set worker heartbeat: %v", err)
	}

	originalPaneExists := shellPaneExistsFn
	t.Cleanup(func() { shellPaneExistsFn = originalPaneExists })
	shellPaneExistsFn = func(_ string, pane string) bool {
		return pane == "codex-view:0:0" || pane == "codex-view:0:1"
	}

	state, err := inspectShellWorkflowState(repoRoot)
	if err != nil {
		t.Fatalf("inspectShellWorkflowState returned error: %v", err)
	}
	if state.PromptTarget != "codex-view:0:0" || state.CommandTarget != "codex-view:0:1" {
		t.Fatalf("unexpected targets: %+v", state)
	}
	if !state.PromptPanePresent || !state.CommandPanePresent || !state.WorkerHealthy {
		t.Fatalf("expected healthy workflow state, got %+v", state)
	}
}

func TestEnsureShellWorkflowWorkerStartsWorkflowWhenTargetsMissing(t *testing.T) {
	repoRoot, _ := openShellWorkflowTestDB(t)

	originalPaneExists := shellPaneExistsFn
	originalRunStart := shellRunStartFn
	originalStartWorker := shellStartWorkerFn
	t.Cleanup(func() {
		shellPaneExistsFn = originalPaneExists
		shellRunStartFn = originalRunStart
		shellStartWorkerFn = originalStartWorker
	})

	shellPaneExistsFn = func(string, string) bool { return false }
	startArgs := []string(nil)
	shellRunStartFn = func(args []string) error {
		startArgs = append([]string(nil), args...)
		return nil
	}
	shellStartWorkerFn = func(string, string, string, time.Duration) error {
		t.Fatalf("did not expect startShellWorker to be called")
		return nil
	}

	if err := ensureShellWorkflowWorker(repoRoot, "codex-view", "default", "medium", "gpt-5.4", "dialtone-view", 20*time.Second); err != nil {
		t.Fatalf("ensureShellWorkflowWorker returned error: %v", err)
	}
	got := strings.Join(startArgs, " ")
	for _, want := range []string{
		"--session codex-view",
		"--dialtone-shell default",
		"--reasoning medium",
		"--model gpt-5.4",
		"--right-title dialtone-view",
		"--wait-seconds 20",
		"--run-tests=false",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in start args %q", want, got)
		}
	}
}

func TestEnsureShellWorkflowWorkerRestartsWorkerWhenWorkerIsStale(t *testing.T) {
	repoRoot, db := openShellWorkflowTestDB(t)
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.TmuxPromptTargetKey, "codex-view:0:0"); err != nil {
		t.Fatalf("set prompt target: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.TmuxTargetKey, "codex-view:0:1"); err != nil {
		t.Fatalf("set command target: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellWorkerStatusKey, "stopped"); err != nil {
		t.Fatalf("set worker status: %v", err)
	}

	originalPaneExists := shellPaneExistsFn
	originalRunStart := shellRunStartFn
	originalStartWorker := shellStartWorkerFn
	t.Cleanup(func() {
		shellPaneExistsFn = originalPaneExists
		shellRunStartFn = originalRunStart
		shellStartWorkerFn = originalStartWorker
	})

	shellPaneExistsFn = func(_ string, pane string) bool {
		return pane == "codex-view:0:0" || pane == "codex-view:0:1"
	}
	shellRunStartFn = func(args []string) error {
		t.Fatalf("did not expect runStart to be called: %v", args)
		return nil
	}
	var (
		gotRepoRoot string
		gotPane     string
		gotShell    string
		gotWait     time.Duration
	)
	shellStartWorkerFn = func(repoRoot, pane, shellName string, wait time.Duration) error {
		gotRepoRoot = repoRoot
		gotPane = pane
		gotShell = shellName
		gotWait = wait
		return nil
	}

	if err := ensureShellWorkflowWorker(repoRoot, "codex-view", "default", "medium", "gpt-5.4", "dialtone-view", 30*time.Second); err != nil {
		t.Fatalf("ensureShellWorkflowWorker returned error: %v", err)
	}
	if gotRepoRoot != repoRoot || gotPane != "codex-view:0:1" || gotShell != "default" || gotWait != 30*time.Second {
		t.Fatalf("unexpected startShellWorker call: repo=%q pane=%q shell=%q wait=%s", gotRepoRoot, gotPane, gotShell, gotWait)
	}
}

func TestEnsureVisibleShellWorkflowReturnsActionableFailure(t *testing.T) {
	repoRoot, _ := openShellWorkflowTestDB(t)

	originalPaneExists := shellPaneExistsFn
	originalRunStart := shellRunStartFn
	originalStartWorker := shellStartWorkerFn
	t.Cleanup(func() {
		shellPaneExistsFn = originalPaneExists
		shellRunStartFn = originalRunStart
		shellStartWorkerFn = originalStartWorker
	})

	shellPaneExistsFn = func(string, string) bool { return false }
	shellRunStartFn = func([]string) error { return nil }
	shellStartWorkerFn = func(string, string, string, time.Duration) error { return nil }

	err := ensureVisibleShellWorkflow(repoRoot, "codex-view", "default", "medium", "gpt-5.4", "dialtone-view", 20*time.Second)
	if err == nil {
		t.Fatalf("expected ensureVisibleShellWorkflow to fail")
	}
	for _, want := range []string{"missing prompt_target", "missing command_target", "worker heartbeat is stale or stopped"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected %q in error: %v", want, err)
		}
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

func TestDeriveVisibleCommandErrorPrefersStructuredProbeError(t *testing.T) {
	output := strings.Join([]string{
		"probe_mode\tfail",
		"probe_result\tfailure",
		"probe_error\trequested failure",
		"exit status 1",
	}, "\n")
	if got := deriveVisibleCommandError(output, nil, 1); got != "requested failure" {
		t.Fatalf("unexpected derived error: %q", got)
	}
}

func TestDeriveVisibleCommandErrorFallsBackToLastMeaningfulLine(t *testing.T) {
	output := strings.Join([]string{
		"line one",
		"custom stderr detail",
		"exit status 1",
	}, "\n")
	if got := deriveVisibleCommandError(output, nil, 1); got != "custom stderr detail" {
		t.Fatalf("unexpected derived error: %q", got)
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

func TestEnqueueShellBusCommandStoresDeterministicLogPath(t *testing.T) {
	repoRoot := t.TempDir()
	stateDir := filepath.Join(t.TempDir(), ".dialtone")
	t.Setenv("DIALTONE_STATE_DIR", stateDir)
	db, err := modstate.Open(filepath.Join(stateDir, "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	rowID, err := enqueueShellBusCommand(db, repoRoot, "shell-cli", "codex-view", "codex-view:0:1", shellBusIntentBody{
		Command: "./dialtone_mod ssh v1 help",
	})
	if err != nil {
		t.Fatalf("enqueueShellBusCommand returned error: %v", err)
	}
	record, ok, err := modstate.LoadShellBusRecord(db, rowID)
	if err != nil {
		t.Fatalf("LoadShellBusRecord returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected shell bus row to exist")
	}
	body, ok := decodeShellBusIntentBody(record)
	if !ok {
		t.Fatalf("expected queued body to decode")
	}
	want := filepath.Join(stateDir, "logs", "commands", "shell-bus-"+strconv.FormatInt(rowID, 10)+".log")
	if body.LogPath != want {
		t.Fatalf("unexpected log path: got %q want %q", body.LogPath, want)
	}
}

func TestWriteShellBusCommandLogCreatesFile(t *testing.T) {
	repoRoot := t.TempDir()
	stateDir := filepath.Join(t.TempDir(), ".dialtone")
	t.Setenv("DIALTONE_STATE_DIR", stateDir)
	body := shellBusIntentBody{
		Command:        "./dialtone_mod mods v1 probe --mode success --label TEST_LOG",
		DisplayCommand: "./dialtone_mod mods v1 probe --mode success --label TEST_LOG",
		Target:         "codex-view:0:1",
		StartedAt:      "2026-03-24T00:00:00Z",
		FinishedAt:     "2026-03-24T00:00:01Z",
		ExitCode:       0,
		RuntimeMS:      1000,
		Summary:        "probe_result\tsuccess",
		Output:         "probe_mode\tsuccess\nprobe_result\tsuccess\n",
	}
	if err := writeShellBusCommandLog(repoRoot, 42, body, "done"); err != nil {
		t.Fatalf("writeShellBusCommandLog returned error: %v", err)
	}
	path := filepath.Join(stateDir, "logs", "commands", "shell-bus-42.log")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	text := string(raw)
	for _, want := range []string{
		"row_id=42",
		"status=done",
		"runtime_ms=1000",
		"output",
		"probe_result\tsuccess",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in log file:\n%s", want, text)
		}
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

func captureShellStdout(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = writer
	done := make(chan string, 1)
	go func() {
		var out strings.Builder
		var buf [4096]byte
		for {
			n, readErr := reader.Read(buf[:])
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
	_ = writer.Close()
	os.Stdout = oldStdout
	return <-done
}

func openShellWorkflowTestDB(t *testing.T) (string, *sql.DB) {
	t.Helper()
	repoRoot := t.TempDir()
	stateDir := filepath.Join(t.TempDir(), ".dialtone")
	stateDB := filepath.Join(stateDir, "state.sqlite")
	t.Setenv("DIALTONE_STATE_DIR", stateDir)
	t.Setenv("DIALTONE_STATE_DB", stateDB)
	db, err := modstate.Open(stateDB)
	if err != nil {
		t.Fatalf("open state db: %v", err)
	}
	if err := modstate.EnsureSchema(db); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return repoRoot, db
}

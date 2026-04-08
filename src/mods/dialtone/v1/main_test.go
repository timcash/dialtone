package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"dialtone/dev/internal/modstate"
	"dialtone/dev/mods/shared/dispatch"
	"dialtone/dev/mods/shared/sqlitestate"
)

func TestDialtoneV1Layout(t *testing.T) {
	root := currentDir(t)
	for _, rel := range []string{
		"README.md",
		"mod.json",
		"main.go",
		"main_test.go",
		filepath.Join("cli", "main.go"),
		filepath.Join("cli", "main_test.go"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("expected %s in dialtone/v1: %v", rel, err)
		}
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	readmeText := string(readme)
	for _, want := range []string{"## Quick Start", "## Dependencies", "## Test Results"} {
		if !strings.Contains(readmeText, want) {
			t.Fatalf("expected dialtone/v1 README to contain %s", want)
		}
	}
	if first := strings.Index(readmeText, "## Test Results"); first == -1 || strings.LastIndex(readmeText, "## Test Results") != first {
		t.Fatalf("expected exactly one Test Results section")
	}
	if strings.LastIndex(readmeText, "\n## ") > strings.Index(readmeText, "## Test Results") {
		t.Fatalf("expected Test Results to be the last README section")
	}
}

func TestEnsureDaemonStartsDetachedBinaryAndPersistsState(t *testing.T) {
	db := openDialtoneTestDB(t)

	originalNow := nowUTC
	originalAlive := processAliveFn
	originalCommand := processCommandFn
	originalTerminate := terminateProcessFn
	originalStart := startDetachedSelfFn
	t.Cleanup(func() {
		nowUTC = originalNow
		processAliveFn = originalAlive
		processCommandFn = originalCommand
		terminateProcessFn = originalTerminate
		startDetachedSelfFn = originalStart
	})

	nowUTC = func() time.Time { return time.Date(2026, 3, 23, 19, 40, 0, 0, time.UTC) }
	processAliveFn = func(pid int) bool { return false }
	processCommandFn = func(pid int) (string, error) { return "", nil }
	terminateProcessFn = func(pid int) error { return nil }
	startDetachedSelfFn = func(repoRoot string) (int, string, error) {
		if repoRoot != "/Users/user/dialtone" {
			t.Fatalf("unexpected repo root: %q", repoRoot)
		}
		return 321, "/tmp/dialtone.log", nil
	}

	result, err := ensureDaemon(db, "/Users/user/dialtone")
	if err != nil {
		t.Fatalf("ensureDaemon returned error: %v", err)
	}
	if result.PID != 321 || result.LogPath != "/tmp/dialtone.log" || result.State != "started" {
		t.Fatalf("unexpected ensure result: %+v", result)
	}

	assertStateValue(t, db, "dialtone.daemon.pid", "321")
	assertStateValue(t, db, "dialtone.daemon.log_path", "/tmp/dialtone.log")
	assertStateValue(t, db, "dialtone.daemon.status", "starting")
	assertStateValue(t, db, "dialtone.daemon.started_at", "2026-03-23T19:40:00Z")
	assertStateValue(t, db, "dialtone.daemon.heartbeat_at", "2026-03-23T19:40:00Z")
}

func TestEnsureDaemonReplacesLegacyWrapperProcess(t *testing.T) {
	db := openDialtoneTestDB(t)
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.pid", "123"); err != nil {
		t.Fatalf("set pid: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.status", "running"); err != nil {
		t.Fatalf("set status: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.log_path", "/tmp/legacy.log"); err != nil {
		t.Fatalf("set log path: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.heartbeat_at", "2026-03-23T19:39:59Z"); err != nil {
		t.Fatalf("set heartbeat: %v", err)
	}

	originalNow := nowUTC
	originalAlive := processAliveFn
	originalCommand := processCommandFn
	originalTerminate := terminateProcessFn
	originalStart := startDetachedSelfFn
	t.Cleanup(func() {
		nowUTC = originalNow
		processAliveFn = originalAlive
		processCommandFn = originalCommand
		terminateProcessFn = originalTerminate
		startDetachedSelfFn = originalStart
	})

	nowUTC = func() time.Time { return time.Date(2026, 3, 23, 19, 40, 0, 0, time.UTC) }
	legacyAlive := true
	terminatedPID := 0
	processAliveFn = func(pid int) bool {
		if pid != 123 {
			return false
		}
		return legacyAlive
	}
	processCommandFn = func(pid int) (string, error) {
		if pid != 123 {
			t.Fatalf("unexpected pid for processCommandFn: %d", pid)
		}
		return "/Users/user/dialtone/dialtone_mod __dialtone serve", nil
	}
	terminateProcessFn = func(pid int) error {
		terminatedPID = pid
		legacyAlive = false
		return nil
	}
	startDetachedSelfFn = func(repoRoot string) (int, string, error) {
		return 456, "/tmp/dialtone-new.log", nil
	}

	result, err := ensureDaemon(db, "/Users/user/dialtone")
	if err != nil {
		t.Fatalf("ensureDaemon returned error: %v", err)
	}
	if result.PID != 456 || result.LogPath != "/tmp/dialtone-new.log" || result.State != "started" {
		t.Fatalf("unexpected ensure result: %+v", result)
	}
	if terminatedPID != 123 {
		t.Fatalf("expected legacy daemon pid 123 to be terminated, got %d", terminatedPID)
	}

	assertStateValue(t, db, "dialtone.daemon.pid", "456")
	assertStateValue(t, db, "dialtone.daemon.log_path", "/tmp/dialtone-new.log")
	assertStateValue(t, db, "dialtone.daemon.status", "starting")
}

func TestEnsureWorkerAsyncReturnsWorkerStateWhenShellReadyAndHealthy(t *testing.T) {
	db := openDialtoneTestDB(t)
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.TmuxPromptTargetKey, "codex-view:0:0"); err != nil {
		t.Fatalf("set prompt target: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.TmuxTargetKey, "codex-view:0:1"); err != nil {
		t.Fatalf("set command target: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellWorkerStatusKey, "running"); err != nil {
		t.Fatalf("set worker status: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellWorkerHeartbeatKey, time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("set worker heartbeat: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellEnsurePIDKey, "77"); err != nil {
		t.Fatalf("set ensure pid: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellEnsureLogPathKey, "/tmp/ensure.log"); err != nil {
		t.Fatalf("set ensure log path: %v", err)
	}

	originalStart := startDetachedCmdFn
	t.Cleanup(func() { startDetachedCmdFn = originalStart })
	startDetachedCmdFn = func(string, ...string) (int, string, error) {
		t.Fatalf("did not expect ensureWorkerAsync to start a new process")
		return 0, "", nil
	}

	result, err := ensureWorkerAsync("/Users/user/dialtone", db)
	if err != nil {
		t.Fatalf("ensureWorkerAsync returned error: %v", err)
	}
	if result.PID != 77 || result.LogPath != "/tmp/ensure.log" || result.State != "worker" {
		t.Fatalf("unexpected ensure result: %+v", result)
	}
}

func TestEnsureWorkerAsyncReturnsExistingEnsureProcessWhenAlive(t *testing.T) {
	db := openDialtoneTestDB(t)
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellEnsurePIDKey, "88"); err != nil {
		t.Fatalf("set ensure pid: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellEnsureLogPathKey, "/tmp/existing-ensure.log"); err != nil {
		t.Fatalf("set ensure log path: %v", err)
	}

	originalAlive := processAliveFn
	originalStart := startDetachedCmdFn
	t.Cleanup(func() {
		processAliveFn = originalAlive
		startDetachedCmdFn = originalStart
	})
	processAliveFn = func(pid int) bool { return pid == 88 }
	startDetachedCmdFn = func(string, ...string) (int, string, error) {
		t.Fatalf("did not expect ensureWorkerAsync to start a new process")
		return 0, "", nil
	}

	result, err := ensureWorkerAsync("/Users/user/dialtone", db)
	if err != nil {
		t.Fatalf("ensureWorkerAsync returned error: %v", err)
	}
	if result.PID != 88 || result.LogPath != "/tmp/existing-ensure.log" || result.State != "existing" {
		t.Fatalf("unexpected ensure result: %+v", result)
	}
}

func TestEnsureWorkerAsyncStartsEnsureWorkerAndPersistsState(t *testing.T) {
	db := openDialtoneTestDB(t)

	originalNow := nowUTC
	originalAlive := processAliveFn
	originalStart := startDetachedCmdFn
	t.Cleanup(func() {
		nowUTC = originalNow
		processAliveFn = originalAlive
		startDetachedCmdFn = originalStart
	})
	nowUTC = func() time.Time { return time.Date(2026, 3, 23, 21, 5, 0, 0, time.UTC) }
	processAliveFn = func(int) bool { return false }

	var gotRepoRoot string
	var gotArgs []string
	startDetachedCmdFn = func(repoRoot string, args ...string) (int, string, error) {
		gotRepoRoot = repoRoot
		gotArgs = append([]string(nil), args...)
		return 99, "/tmp/dialtone-ensure.log", nil
	}

	result, err := ensureWorkerAsync("/Users/user/dialtone", db)
	if err != nil {
		t.Fatalf("ensureWorkerAsync returned error: %v", err)
	}
	if result.PID != 99 || result.LogPath != "/tmp/dialtone-ensure.log" || result.State != "started" {
		t.Fatalf("unexpected ensure result: %+v", result)
	}
	if gotRepoRoot != "/Users/user/dialtone" || strings.Join(gotArgs, " ") != "shell v1 ensure-worker --wait-seconds 30" {
		t.Fatalf("unexpected detached ensure command: repo=%q args=%q", gotRepoRoot, strings.Join(gotArgs, " "))
	}
	assertStateValue(t, db, sqlitestate.ShellEnsurePIDKey, "99")
	assertStateValue(t, db, sqlitestate.ShellEnsureLogPathKey, "/tmp/dialtone-ensure.log")
	assertStateValue(t, db, sqlitestate.ShellEnsureStartedAtKey, "2026-03-23T21:05:00Z")
}

func TestDaemonHealthyRejectsStaleHeartbeat(t *testing.T) {
	db := openDialtoneTestDB(t)
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.status", "running"); err != nil {
		t.Fatalf("set status: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.pid", "123"); err != nil {
		t.Fatalf("set pid: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.heartbeat_at", "2026-03-23T19:39:30Z"); err != nil {
		t.Fatalf("set heartbeat: %v", err)
	}

	originalNow := nowUTC
	originalAlive := processAliveFn
	t.Cleanup(func() {
		nowUTC = originalNow
		processAliveFn = originalAlive
	})
	nowUTC = func() time.Time { return time.Date(2026, 3, 23, 19, 40, 0, 0, time.UTC) }
	processAliveFn = func(pid int) bool { return pid == 123 }

	healthy, err := daemonHealthy(db, 10*time.Second)
	if err != nil {
		t.Fatalf("daemonHealthy returned error: %v", err)
	}
	if healthy {
		t.Fatalf("expected stale heartbeat to be unhealthy")
	}
}

func TestDaemonHealthyRejectsLegacyWrapperProcess(t *testing.T) {
	db := openDialtoneTestDB(t)
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.status", "running"); err != nil {
		t.Fatalf("set status: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.pid", "123"); err != nil {
		t.Fatalf("set pid: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.heartbeat_at", "2026-03-23T19:40:00Z"); err != nil {
		t.Fatalf("set heartbeat: %v", err)
	}

	originalNow := nowUTC
	originalAlive := processAliveFn
	originalCommand := processCommandFn
	t.Cleanup(func() {
		nowUTC = originalNow
		processAliveFn = originalAlive
		processCommandFn = originalCommand
	})
	nowUTC = func() time.Time { return time.Date(2026, 3, 23, 19, 40, 5, 0, time.UTC) }
	processAliveFn = func(pid int) bool { return pid == 123 }
	processCommandFn = func(pid int) (string, error) {
		if pid != 123 {
			t.Fatalf("unexpected pid: %d", pid)
		}
		return "/Users/user/dialtone/dialtone_mod __dialtone serve", nil
	}

	healthy, err := daemonHealthy(db, 10*time.Second)
	if err != nil {
		t.Fatalf("daemonHealthy returned error: %v", err)
	}
	if healthy {
		t.Fatalf("expected legacy wrapper daemon to be unhealthy")
	}
}

func TestExistingEnsureProcessIfAliveReturnsFalseWhenPIDIsDead(t *testing.T) {
	db := openDialtoneTestDB(t)
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellEnsurePIDKey, "88"); err != nil {
		t.Fatalf("set ensure pid: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellEnsureLogPathKey, "/tmp/ensure.log"); err != nil {
		t.Fatalf("set ensure log path: %v", err)
	}

	originalAlive := processAliveFn
	t.Cleanup(func() { processAliveFn = originalAlive })
	processAliveFn = func(int) bool { return false }

	pid, logPath, ok, err := existingEnsureProcessIfAlive(db)
	if err != nil {
		t.Fatalf("existingEnsureProcessIfAlive returned error: %v", err)
	}
	if ok || pid != 88 || logPath != "/tmp/ensure.log" {
		t.Fatalf("unexpected ensure state: pid=%d log=%q ok=%v", pid, logPath, ok)
	}
}

func TestClearEnsureStateIfShellReadyPreservesEnsureStateWhenWorkflowNotReady(t *testing.T) {
	db := openDialtoneTestDB(t)
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellWorkerStatusKey, "running"); err != nil {
		t.Fatalf("set worker status: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellWorkerHeartbeatKey, time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("set worker heartbeat: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellEnsurePIDKey, "88"); err != nil {
		t.Fatalf("set ensure pid: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellEnsureLogPathKey, "/tmp/ensure.log"); err != nil {
		t.Fatalf("set ensure log path: %v", err)
	}

	cleared, err := clearEnsureStateIfShellReady(db, 5*time.Second)
	if err != nil {
		t.Fatalf("clearEnsureStateIfShellReady returned error: %v", err)
	}
	if cleared {
		t.Fatalf("did not expect ensure state to be cleared while workflow targets are missing")
	}
	assertStateValue(t, db, sqlitestate.ShellEnsurePIDKey, "88")
	assertStateValue(t, db, sqlitestate.ShellEnsureLogPathKey, "/tmp/ensure.log")
}

func TestClearEnsureStateIfShellReadyClearsEnsureStateWhenWorkflowReadyAndHealthy(t *testing.T) {
	db := openDialtoneTestDB(t)
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.TmuxPromptTargetKey, "codex-view:0:0"); err != nil {
		t.Fatalf("set prompt target: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.TmuxTargetKey, "codex-view:0:1"); err != nil {
		t.Fatalf("set command target: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellWorkerStatusKey, "running"); err != nil {
		t.Fatalf("set worker status: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellWorkerHeartbeatKey, time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("set worker heartbeat: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellEnsurePIDKey, "88"); err != nil {
		t.Fatalf("set ensure pid: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellEnsureLogPathKey, "/tmp/ensure.log"); err != nil {
		t.Fatalf("set ensure log path: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellEnsureStartedAtKey, "2026-03-23T19:40:00Z"); err != nil {
		t.Fatalf("set ensure started at: %v", err)
	}

	cleared, err := clearEnsureStateIfShellReady(db, 5*time.Second)
	if err != nil {
		t.Fatalf("clearEnsureStateIfShellReady returned error: %v", err)
	}
	if !cleared {
		t.Fatalf("expected ensure state to be cleared once workflow is ready and healthy")
	}
	for _, key := range []string{
		sqlitestate.ShellEnsurePIDKey,
		sqlitestate.ShellEnsureLogPathKey,
		sqlitestate.ShellEnsureStartedAtKey,
	} {
		if _, ok, err := loadOptionalStateValue(db, key); err != nil {
			t.Fatalf("loadOptionalStateValue(%s) returned error: %v", key, err)
		} else if ok {
			t.Fatalf("expected %s to be deleted", key)
		}
	}
}

func TestParseStatusArgsAcceptsShellCompatibilityFlags(t *testing.T) {
	opts, err := parseStatusArgs([]string{"--row-id", "42", "--full", "--sync=false", "--limit", "20"})
	if err != nil {
		t.Fatalf("parseStatusArgs returned error: %v", err)
	}
	if opts.rowID != 42 || !opts.full {
		t.Fatalf("unexpected parsed options: %+v", opts)
	}
}

func TestDialtoneProcessRoleRecognizesDaemonBinaryAndWorker(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		{command: "/Users/user/.dialtone/bin/dialtone serve", want: "dialtone"},
		{command: "/Users/user/dialtone/dialtone_mod __dialtone serve", want: "dialtone"},
		{command: "go run ./mods.go shell v1 serve --pane codex-view:0:1", want: "worker"},
		{command: "/Users/user/dialtone/dialtone_mod shell v1 ensure-worker --wait-seconds 30", want: "ensure"},
		{command: "/Users/user/dialtone/dialtone_mod mods v1 db graph --format outline", want: "dialtone_mod"},
	}
	for _, tc := range cases {
		if got := dialtoneProcessRole(tc.command); got != tc.want {
			t.Fatalf("dialtoneProcessRole(%q) = %q, want %q", tc.command, got, tc.want)
		}
	}
}

func TestRenderRouteReportIncludesQueuedCommandAndInspectHint(t *testing.T) {
	db := openDialtoneTestDB(t)
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, "bootstrap.status", "ready"); err != nil {
		t.Fatalf("set bootstrap.status: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellWorkerStatusKey, "running"); err != nil {
		t.Fatalf("set worker status: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellWorkerPaneKey, "codex-view:0:1"); err != nil {
		t.Fatalf("set worker pane: %v", err)
	}

	originalLoadProcessReport := loadProcessReportFn
	t.Cleanup(func() { loadProcessReportFn = originalLoadProcessReport })
	loadProcessReportFn = func() ([]processRecord, error) {
		return []processRecord{
			{PID: 10, PPID: 1, Stat: "Ss", TTY: "??", Role: "dialtone", Background: true, Command: "/Users/user/.dialtone/bin/dialtone serve"},
			{PID: 11, PPID: 10, Stat: "S+", TTY: "s174", Role: "worker", Background: false, Command: "go run ./mods.go shell v1 serve --pane codex-view:0:1"},
		}, nil
	}

	text := renderRouteReport(
		"/Users/user/dialtone",
		db,
		948,
		72,
		"./dialtone_mod mods v1 db graph --format outline",
		"codex-view:0:1",
		ensureResult{PID: 10, LogPath: "/tmp/dialtone.log", State: "started"},
	)
	commandLogPath := sqlitestate.ResolveCommandLogPath("/Users/user/dialtone", 948)
	for _, want := range []string{
		"route\tqueued",
		"command_id\t948",
		"run_id\t72",
		"command\t./dialtone_mod mods v1 db graph --format outline",
		"command_target\tcodex-view:0:1",
		"command_log_path\t" + commandLogPath,
		"dialtone_pid\t10",
		"dialtone_status\tstarted",
		"worker_status\trunning",
		"inspect\t./dialtone_mod dialtone v1 command --row-id 948 --full",
		"inspect_log\t./dialtone_mod dialtone v1 log --kind command --row-id 948",
		"inspect_run\t./dialtone_mod mods v1 db run --id 72",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("renderRouteReport missing %q in output:\n%s", want, text)
		}
	}
}

func TestResolveDialtoneLogPathFallsBackToDeterministicCommandPath(t *testing.T) {
	db := openDialtoneTestDB(t)
	body := `{"command":"./dialtone_mod ssh v1 help"}`
	rowID, err := modstate.EnqueueShellBus(db, "shell", "desired", "command", "run", "dialtone_mod", "codex-view", "codex-view:0:1", body)
	if err != nil {
		t.Fatalf("EnqueueShellBus returned error: %v", err)
	}
	path, err := resolveDialtoneLogPath(db, "/Users/user/dialtone", "command", rowID)
	if err != nil {
		t.Fatalf("resolveDialtoneLogPath returned error: %v", err)
	}
	want := sqlitestate.ResolveCommandLogPath("/Users/user/dialtone", rowID)
	if path != want {
		t.Fatalf("unexpected log path: got %q want %q", path, want)
	}
}

func TestResolveDialtoneLogPathUsesRecordedStateForDaemonEnsureAndBootstrap(t *testing.T) {
	db := openDialtoneTestDB(t)
	stateValues := map[string]string{
		"dialtone.daemon.log_path":        "/tmp/dialtone.log",
		sqlitestate.ShellEnsureLogPathKey: "/tmp/ensure.log",
		"bootstrap.log_path":              "/tmp/bootstrap.log",
	}
	for key, value := range stateValues {
		if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, key, value); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
	}

	for _, tc := range []struct {
		kind string
		want string
	}{
		{kind: "daemon", want: "/tmp/dialtone.log"},
		{kind: "ensure", want: "/tmp/ensure.log"},
		{kind: "bootstrap", want: "/tmp/bootstrap.log"},
	} {
		got, err := resolveDialtoneLogPath(db, "/Users/user/dialtone", tc.kind, 0)
		if err != nil {
			t.Fatalf("resolveDialtoneLogPath(%s) returned error: %v", tc.kind, err)
		}
		if got != tc.want {
			t.Fatalf("resolveDialtoneLogPath(%s) = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestResolveDialtoneLogPathPrefersCommandIntentLogPath(t *testing.T) {
	db := openDialtoneTestDB(t)
	body := mustEncodeIntentBody(t, dispatch.ShellCommandIntent{
		Command: "./dialtone_mod mods v1 db graph --format outline",
		LogPath: "/tmp/explicit-command.log",
	})
	rowID, err := modstate.EnqueueShellBus(db, "shell", "desired", "command", "run", "dialtone_mod", "codex-view", "codex-view:0:1", body)
	if err != nil {
		t.Fatalf("EnqueueShellBus returned error: %v", err)
	}
	path, err := resolveDialtoneLogPath(db, "/Users/user/dialtone", "command", rowID)
	if err != nil {
		t.Fatalf("resolveDialtoneLogPath returned error: %v", err)
	}
	if path != "/tmp/explicit-command.log" {
		t.Fatalf("unexpected explicit command log path: %q", path)
	}
}

func TestMatchesCommandFiltersRequireAllSpecifiedFields(t *testing.T) {
	row := modstate.ShellBusRecord{
		Scope:   "desired",
		Subject: "command",
		Status:  "done",
		Session: "codex-view",
		Pane:    "codex-view:0:1",
	}
	if !matchesCommandFilters(row, commandsOptions{
		scope:   "desired",
		subject: "command",
		status:  "done",
		session: "codex-view",
		pane:    "codex-view:0:1",
	}) {
		t.Fatalf("expected row to match all exact filters")
	}
	if matchesCommandFilters(row, commandsOptions{scope: "observed"}) {
		t.Fatalf("did not expect row to match different scope")
	}
	if matchesCommandFilters(row, commandsOptions{pane: "codex-view:0:0"}) {
		t.Fatalf("did not expect row to match different pane")
	}
}

func TestLatestDesiredCommandRecordHydratesDisplayCommandAndTarget(t *testing.T) {
	row := modstate.ShellBusRecord{
		ID:      42,
		Subject: "command",
		Action:  "run",
		Pane:    "codex-view:0:1",
		BodyJSON: mustEncodeIntentBody(t, dispatch.ShellCommandIntent{
			Command:      "./dialtone_mod mods v1 db graph --format outline",
			InnerCommand: "./dialtone_mod mods v1 db graph --format outline",
		}),
	}
	gotRow, gotBody, ok := latestDesiredCommandRecord([]modstate.ShellBusRecord{
		{Subject: "pane", Action: "snapshot"},
		row,
	})
	if !ok {
		t.Fatalf("expected latestDesiredCommandRecord to find queued command")
	}
	if gotRow.ID != 42 {
		t.Fatalf("unexpected row id: %d", gotRow.ID)
	}
	if gotBody.DisplayCommand != "./dialtone_mod mods v1 db graph --format outline" {
		t.Fatalf("unexpected display command: %q", gotBody.DisplayCommand)
	}
	if gotBody.Target != "codex-view:0:1" {
		t.Fatalf("expected target to fall back to pane, got %q", gotBody.Target)
	}
}

func TestCommandRuntimeMillisFallsBackToUpdatedAt(t *testing.T) {
	runtimeMS, ok := commandRuntimeMillis(
		modstate.ShellBusRecord{UpdatedAt: "2026-03-23T19:40:02.500Z"},
		dispatch.ShellCommandIntent{StartedAt: "2026-03-23T19:40:01Z"},
	)
	if !ok {
		t.Fatalf("expected runtime calculation to succeed")
	}
	if runtimeMS != 1500 {
		t.Fatalf("unexpected runtime: %d", runtimeMS)
	}
}

func TestSummarizeProcessRecordsCountsRolesAndBackground(t *testing.T) {
	total, totalBackground, dialtoneModCount, dialtoneModBackground, dialtoneCount, workerCount, ensureCount := summarizeProcessRecords([]processRecord{
		{Role: "dialtone_mod", Background: true},
		{Role: "dialtone_mod", Background: false},
		{Role: "dialtone", Background: true},
		{Role: "worker", Background: false},
		{Role: "ensure", Background: true},
	})
	if total != 5 || totalBackground != 3 {
		t.Fatalf("unexpected total/background counts: total=%d background=%d", total, totalBackground)
	}
	if dialtoneModCount != 2 || dialtoneModBackground != 1 || dialtoneCount != 1 || workerCount != 1 || ensureCount != 1 {
		t.Fatalf("unexpected role counts: dialtone_mod=%d dialtone_mod_background=%d dialtone=%d worker=%d ensure=%d",
			dialtoneModCount, dialtoneModBackground, dialtoneCount, workerCount, ensureCount)
	}
}

func TestTailFileLinesReturnsTrailingLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "example.log")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\nfour\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	got, err := tailFileLines(path, 2)
	if err != nil {
		t.Fatalf("tailFileLines returned error: %v", err)
	}
	if got != "three\nfour" {
		t.Fatalf("unexpected tailed log text: %q", got)
	}
}

func TestRunProcessesPrintsSummariesFromProcessReport(t *testing.T) {
	originalLoadProcessReport := loadProcessReportFn
	t.Cleanup(func() { loadProcessReportFn = originalLoadProcessReport })
	loadProcessReportFn = func() ([]processRecord, error) {
		return []processRecord{
			{PID: 10, PPID: 1, Stat: "Ss", TTY: "??", Role: "dialtone", Background: true, Command: "/Users/user/.dialtone/bin/dialtone serve"},
			{PID: 11, PPID: 10, Stat: "S+", TTY: "s174", Role: "worker", Background: false, Command: "go run ./mods.go shell v1 serve --pane codex-view:0:1"},
			{PID: 12, PPID: 10, Stat: "S", TTY: "??", Role: "ensure", Background: true, Command: "/Users/user/dialtone/dialtone_mod shell v1 ensure-worker --wait-seconds 30"},
		}, nil
	}

	output, err := captureDialtoneStdout(t, func() error {
		return runProcesses(nil)
	})
	if err != nil {
		t.Fatalf("runProcesses returned error: %v", err)
	}
	for _, want := range []string{
		"dialtone_processes_running\t3",
		"worker_running\t1",
		"ensure_running\t1",
		"10\t1\tSs\t??\tdialtone\tyes\t/Users/user/.dialtone/bin/dialtone serve",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("runProcesses output missing %q:\n%s", want, output)
		}
	}
}

func TestRunCommandReadsSQLiteRowWithoutWorkflowDependencies(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "state.sqlite")
	db := openDialtoneTestDBAt(t, dbPath)
	configureDialtoneStateEnv(t, dbPath)

	body := mustEncodeIntentBody(t, dispatch.ShellCommandIntent{
		Command:        "./dialtone_mod mods v1 db graph --format outline",
		DisplayCommand: "./dialtone_mod mods v1 db graph --format outline",
		Target:         "codex-view:0:1",
		LogPath:        "/tmp/command.log",
		StartedAt:      "2026-03-23T19:40:01Z",
		FinishedAt:     "2026-03-23T19:40:03Z",
		RuntimeMS:      2000,
		Summary:        "graph completed",
		Output:         "graph_output",
		ExitCode:       0,
	})
	rowID, err := modstate.EnqueueShellBus(db, "shell", "desired", "command", "run", "dialtone_mod", "codex-view", "codex-view:0:1", body)
	if err != nil {
		t.Fatalf("EnqueueShellBus returned error: %v", err)
	}
	if err := modstate.UpdateShellBusStatus(db, rowID, "done", 0, body); err != nil {
		t.Fatalf("UpdateShellBusStatus returned error: %v", err)
	}

	output, err := captureDialtoneStdout(t, func() error {
		return runCommand([]string{"--row-id", strconv.FormatInt(rowID, 10), "--full"})
	})
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	for _, want := range []string{
		"command_row_id\t" + strconv.FormatInt(rowID, 10),
		"command_status\tdone",
		"command_target\tcodex-view:0:1",
		"command_log_path\t/tmp/command.log",
		"command_summary\tgraph completed",
		"command_output\ngraph_output",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("runCommand output missing %q:\n%s", want, output)
		}
	}
}

func TestRunStatusReadsSQLiteStateAndLatestCommandWithoutWorkflowDependencies(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "state.sqlite")
	db := openDialtoneTestDBAt(t, dbPath)
	configureDialtoneStateEnv(t, dbPath)

	stateValues := map[string]string{
		sqlitestate.TmuxPromptTargetKey:     "codex-view:0:0",
		sqlitestate.TmuxTargetKey:           "codex-view:0:1",
		sqlitestate.ShellWorkerStatusKey:    "running",
		sqlitestate.ShellWorkerPaneKey:      "codex-view:0:1",
		sqlitestate.ShellWorkerHeartbeatKey: "2026-03-23T19:40:03Z",
		"dialtone.daemon.status":            "running",
		"dialtone.daemon.pid":               "123",
		"dialtone.daemon.log_path":          "/tmp/dialtone.log",
	}
	for key, value := range stateValues {
		if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, key, value); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
	}

	body := mustEncodeIntentBody(t, dispatch.ShellCommandIntent{
		Command:        "./dialtone_mod mods v1 db graph --format outline",
		DisplayCommand: "./dialtone_mod mods v1 db graph --format outline",
		Target:         "codex-view:0:1",
		Summary:        "graph completed",
		Output:         "graph_output",
		StartedAt:      "2026-03-23T19:40:01Z",
		FinishedAt:     "2026-03-23T19:40:03Z",
	})
	rowID, err := modstate.EnqueueShellBus(db, "shell", "desired", "command", "run", "dialtone_mod", "codex-view", "codex-view:0:1", body)
	if err != nil {
		t.Fatalf("EnqueueShellBus returned error: %v", err)
	}
	if err := modstate.UpdateShellBusStatus(db, rowID, "done", 0, body); err != nil {
		t.Fatalf("UpdateShellBusStatus returned error: %v", err)
	}
	if _, err := modstate.AppendShellBusObserved(db, "shell", "pane", "snapshot", "shell", "codex-view", "codex-view:0:0", rowID, `{"text":"prompt line one\nprompt line two"}`); err != nil {
		t.Fatalf("AppendShellBusObserved prompt returned error: %v", err)
	}
	if _, err := modstate.AppendShellBusObserved(db, "shell", "pane", "snapshot", "shell", "codex-view", "codex-view:0:1", rowID, `{"text":"command line one\ncommand line two"}`); err != nil {
		t.Fatalf("AppendShellBusObserved command returned error: %v", err)
	}

	originalLoadProcessReport := loadProcessReportFn
	t.Cleanup(func() { loadProcessReportFn = originalLoadProcessReport })
	loadProcessReportFn = func() ([]processRecord, error) {
		return []processRecord{
			{PID: 123, PPID: 1, Stat: "Ss", TTY: "??", Role: "dialtone", Background: true, Command: "/Users/user/.dialtone/bin/dialtone serve"},
		}, nil
	}

	output, err := captureDialtoneStdout(t, func() error {
		return runStatus([]string{"--full"})
	})
	if err != nil {
		t.Fatalf("runStatus returned error: %v", err)
	}
	for _, want := range []string{
		"prompt_target\tcodex-view:0:0",
		"command_target\tcodex-view:0:1",
		"queued\t0",
		"worker_status\trunning",
		"command_row_id\t" + strconv.FormatInt(rowID, 10),
		"command_summary\tgraph completed",
		"prompt_snapshot\tprompt line two",
		"command_snapshot\tcommand line two",
		"prompt_text\nprompt line one\nprompt line two",
		"command_text\ncommand line one\ncommand line two",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("runStatus output missing %q:\n%s", want, output)
		}
	}
}

func openDialtoneTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := modstate.Open(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatalf("open state db: %v", err)
	}
	if err := modstate.EnsureSchema(db); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func openDialtoneTestDBAt(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := modstate.Open(path)
	if err != nil {
		t.Fatalf("open state db: %v", err)
	}
	if err := modstate.EnsureSchema(db); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func assertStateValue(t *testing.T, db *sql.DB, key, want string) {
	t.Helper()
	record, ok, err := modstate.LoadStateValue(db, sqlitestate.SystemScope, key)
	if err != nil {
		t.Fatalf("load state %s: %v", key, err)
	}
	if !ok || strings.TrimSpace(record.Value) != want {
		t.Fatalf("state %s = %q ok=%v, want %q", key, record.Value, ok, want)
	}
}

func configureDialtoneStateEnv(t *testing.T, dbPath string) {
	t.Helper()
	t.Setenv("DIALTONE_REPO_ROOT", repoRootForTests(t))
	t.Setenv("DIALTONE_STATE_DB", dbPath)
	t.Setenv("DIALTONE_STATE_DIR", filepath.Dir(dbPath))
}

func repoRootForTests(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join(currentDir(t), "..", "..", "..", ".."))
}

func captureDialtoneStdout(t *testing.T, fn func() error) (string, error) {
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
	runErr := fn()
	_ = w.Close()
	return <-done, runErr
}

func mustEncodeIntentBody(t *testing.T, body dispatch.ShellCommandIntent) string {
	t.Helper()
	encoded, err := dispatch.EncodeIntentBody(body)
	if err != nil {
		t.Fatalf("EncodeIntentBody returned error: %v", err)
	}
	return encoded
}

func currentDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}

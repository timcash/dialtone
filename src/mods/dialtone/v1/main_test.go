package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"dialtone/dev/internal/modstate"
	"dialtone/dev/mods/shared/sqlitestate"
)

func TestDialtoneV1Layout(t *testing.T) {
	root := currentDir(t)
	for _, rel := range []string{
		"README.md",
		"mod.json",
		"main.go",
		"main_test.go",
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
		"./dialtone_mod mods v1 db graph --format outline",
		"codex-view:0:1",
		ensureResult{PID: 10, LogPath: "/tmp/dialtone.log", State: "started"},
	)
	for _, want := range []string{
		"route\tqueued",
		"command_id\t948",
		"command\t./dialtone_mod mods v1 db graph --format outline",
		"command_target\tcodex-view:0:1",
		"dialtone_pid\t10",
		"dialtone_status\tstarted",
		"worker_status\trunning",
		"inspect\t./dialtone_mod shell v1 status --row-id 948 --full --sync=false",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("renderRouteReport missing %q in output:\n%s", want, text)
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

func currentDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}

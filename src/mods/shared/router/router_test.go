package router

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dialtone/dev/internal/modstate"
	"dialtone/dev/mods/shared/dispatch"
	"dialtone/dev/mods/shared/sqlitestate"
)

type fakeRunner struct {
	calls []runnerCall
	err   error
}

type runnerCall struct {
	RepoRoot string
	GoBin    string
	Entry    string
	Args     []string
}

func (f *fakeRunner) Run(repoRoot, goBin, entry string, args ...string) error {
	f.calls = append(f.calls, runnerCall{
		RepoRoot: repoRoot,
		GoBin:    goBin,
		Entry:    entry,
		Args:     append([]string(nil), args...),
	})
	return f.err
}

func TestStartShellWorkflowUsesShellCLI(t *testing.T) {
	runner := &fakeRunner{}
	if err := StartShellWorkflow("/Users/user/dialtone", "go", runner); err != nil {
		t.Fatalf("StartShellWorkflow returned error: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 runner call, got %d", len(runner.calls))
	}
	call := runner.calls[0]
	if call.Entry != "./mods/shell/v1/cli" {
		t.Fatalf("unexpected entry: %+v", call)
	}
	if got := strings.Join(call.Args, " "); got != "start --run-tests=false" {
		t.Fatalf("unexpected args: %q", got)
	}
}

func TestQueueCommandViaShellCreatesTrackedShellBusRow(t *testing.T) {
	db := openRouterTestDB(t)
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.TmuxTargetKey, "codex-view:0:1"); err != nil {
		t.Fatalf("set tmux target: %v", err)
	}

	rowID, err := QueueCommandViaShell(db, "/Users/user/dialtone", []string{"ssh", "v1", "help"})
	if err != nil {
		t.Fatalf("QueueCommandViaShell returned error: %v", err)
	}
	record, ok, err := modstate.LoadShellBusRecord(db, rowID)
	if err != nil {
		t.Fatalf("LoadShellBusRecord returned error: %v", err)
	}
	if !ok || record.Status != "queued" || record.Pane != "codex-view:0:1" {
		t.Fatalf("unexpected queued record: ok=%v record=%+v", ok, record)
	}
	body, err := dispatch.DecodeIntentBody(record.BodyJSON)
	if err != nil {
		t.Fatalf("DecodeIntentBody returned error: %v", err)
	}
	if body.InnerCommand != "./dialtone_mod ssh v1 help" {
		t.Fatalf("unexpected inner command: %+v", body)
	}
	if body.Command != "./dialtone_mod ssh v1 help" {
		t.Fatalf("unexpected visible command: %+v", body)
	}
	if len(body.Args) != 3 || body.Args[0] != "ssh" || body.Args[2] != "help" {
		t.Fatalf("expected raw args to be stored in sqlite, got %+v", body.Args)
	}
}

func TestSyncShellRunsShellSyncOnce(t *testing.T) {
	runner := &fakeRunner{}
	if err := SyncShell("/Users/user/dialtone", "go", runner, 20, 240); err != nil {
		t.Fatalf("SyncShell returned error: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 runner call, got %d", len(runner.calls))
	}
	call := runner.calls[0]
	if call.Entry != "./mods/shell/v1/cli" {
		t.Fatalf("unexpected entry: %+v", call)
	}
	if got := strings.Join(call.Args, " "); got != "sync-once --limit 20 --wait-seconds 240" {
		t.Fatalf("unexpected args: %q", got)
	}
}

func TestRunCommandViaShellUsesShellRunCLI(t *testing.T) {
	runner := &fakeRunner{}
	err := RunCommandViaShell("/Users/user/dialtone", "go", runner, []string{"ssh", "v1", "help"}, 240)
	if err != nil {
		t.Fatalf("RunCommandViaShell returned error: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 runner call, got %d", len(runner.calls))
	}
	call := runner.calls[0]
	if call.Entry != "./mods/shell/v1/cli" {
		t.Fatalf("unexpected entry: %+v", call)
	}
	if len(call.Args) != 4 || call.Args[0] != "run" || call.Args[1] != "--wait-seconds" || call.Args[2] != "240" {
		t.Fatalf("unexpected shell run args: %+v", call.Args)
	}
	if got := call.Args[len(call.Args)-1]; !strings.Contains(got, "./dialtone_mod ssh v1 help") {
		t.Fatalf("expected visible routed command, got %q", got)
	}
}

func TestBuildShellRunArgsWrapsDialtoneCommand(t *testing.T) {
	args := BuildShellRunArgs("/Users/user/dialtone", []string{"ssh", "v1", "help"}, 240)
	if len(args) != 4 {
		t.Fatalf("unexpected args: %+v", args)
	}
	if args[0] != "run" || args[1] != "--wait-seconds" || args[2] != "240" {
		t.Fatalf("unexpected shell run args: %+v", args)
	}
	if got := args[len(args)-1]; !strings.Contains(got, "./dialtone_mod ssh v1 help") {
		t.Fatalf("expected visible routed command, got %q", got)
	}
}

func TestShellWorkerHealthyRequiresFreshHeartbeat(t *testing.T) {
	db := openRouterTestDB(t)
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellWorkerStatusKey, "running"); err != nil {
		t.Fatalf("set worker status: %v", err)
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellWorkerHeartbeatKey, time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("set worker heartbeat: %v", err)
	}
	ok, err := ShellWorkerHealthy(db, 5*time.Second)
	if err != nil {
		t.Fatalf("ShellWorkerHealthy returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected worker to be healthy")
	}
}

func TestWaitForShellBusCompletionReturnsFinishedRecord(t *testing.T) {
	db := openRouterTestDB(t)
	rowID, err := modstate.EnqueueShellBus(db, "shell", "desired", "command", "run", "dialtone_mod", "codex-view", "codex-view:0:1", `{"command":"./dialtone_mod ssh v1 help"}`)
	if err != nil {
		t.Fatalf("EnqueueShellBus returned error: %v", err)
	}
	if err := modstate.UpdateShellBusStatus(db, rowID, "done", 99, `{"summary":"ok"}`); err != nil {
		t.Fatalf("UpdateShellBusStatus returned error: %v", err)
	}

	record, err := WaitForShellBusCompletion(db, rowID, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForShellBusCompletion returned error: %v", err)
	}
	if record.ID != rowID || record.Status != "done" {
		t.Fatalf("unexpected record: %+v", record)
	}
}

func openRouterTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := modstate.Open(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

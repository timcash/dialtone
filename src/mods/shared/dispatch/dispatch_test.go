package dispatch

import (
	"path/filepath"
	"strings"
	"testing"

	"dialtone/dev/internal/modstate"
	"dialtone/dev/mods/shared/sqlitestate"
)

func TestShellReadyRequiresBothTargets(t *testing.T) {
	db, err := modstate.Open(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	ready, err := ShellReady(db)
	if err != nil {
		t.Fatalf("ShellReady returned error: %v", err)
	}
	if ready {
		t.Fatalf("expected shell workflow to be unready without targets")
	}

	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.TmuxTargetKey, "codex-view:0:1"); err != nil {
		t.Fatalf("set command target: %v", err)
	}
	ready, err = ShellReady(db)
	if err != nil {
		t.Fatalf("ShellReady returned error: %v", err)
	}
	if ready {
		t.Fatalf("expected shell workflow to remain unready with only command target")
	}

	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.TmuxPromptTargetKey, "codex-view:0:0"); err != nil {
		t.Fatalf("set prompt target: %v", err)
	}
	ready, err = ShellReady(db)
	if err != nil {
		t.Fatalf("ShellReady returned error: %v", err)
	}
	if !ready {
		t.Fatalf("expected shell workflow to be ready once both targets are present")
	}
}

func TestShouldRouteViaShellExcludesBootstrapAndBackendMods(t *testing.T) {
	cases := []struct {
		mod     string
		command string
		want    bool
	}{
		{mod: "shell", command: "test", want: false},
		{mod: "tmux", command: "read", want: false},
		{mod: "ghostty", command: "list", want: false},
		{mod: "mod", command: "db", want: false},
		{mod: "ssh", command: "test", want: true},
		{mod: "codex", command: "status", want: true},
	}
	for _, tc := range cases {
		if got := ShouldRouteViaShell(tc.mod, tc.command); got != tc.want {
			t.Fatalf("ShouldRouteViaShell(%q, %q) = %v, want %v", tc.mod, tc.command, got, tc.want)
		}
	}
}

func TestBuildTrackedDialtoneCommandIncludesSentinelAndInnerCommand(t *testing.T) {
	command, expect := BuildTrackedDialtoneCommand([]string{"ssh", "v1", "test"}, 42)
	if expect != "DIALTONE_CMD_DONE_42" {
		t.Fatalf("unexpected expect sentinel: %q", expect)
	}
	if command == "" || command == expect {
		t.Fatalf("unexpected tracked command: %q", command)
	}
	if want := "./dialtone_mod ssh v1 test"; !strings.Contains(command, want) {
		t.Fatalf("tracked command missing inner command %q: %q", want, command)
	}
	if !strings.Contains(command, "printf 'DIALTONE_CMD_DONE_42 exit=%s\\n'") {
		t.Fatalf("tracked command missing sentinel print: %q", command)
	}
}

func TestBuildTrackedVisibleCommandRunsFromRepoRoot(t *testing.T) {
	command, expect, inner := BuildTrackedVisibleCommand("/Users/user/dialtone", []string{"ssh", "v1", "help"}, 88)
	if expect != "DIALTONE_CMD_DONE_88" {
		t.Fatalf("unexpected expect sentinel: %q", expect)
	}
	if inner != "./dialtone_mod ssh v1 help" {
		t.Fatalf("unexpected inner command: %q", inner)
	}
	if !strings.Contains(command, "cd /Users/user/dialtone && ./dialtone_mod ssh v1 help") {
		t.Fatalf("visible command should start from repo root, got %q", command)
	}
	if !strings.Contains(command, "DIALTONE_CMD_DONE_88") {
		t.Fatalf("visible command missing sentinel, got %q", command)
	}
}

func TestShouldExecuteDirectInPaneMatchesRunningInnerCommand(t *testing.T) {
	db, err := modstate.Open(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	command, expect := BuildTrackedDialtoneCommand([]string{"ssh", "v1", "test"}, 7)
	body, err := EncodeIntentBody(ShellCommandIntent{
		Command:      command,
		Expect:       expect,
		InnerCommand: "./dialtone_mod ssh v1 test",
	})
	if err != nil {
		t.Fatalf("EncodeIntentBody returned error: %v", err)
	}
	rowID, err := modstate.EnqueueShellBus(db, "shell", "desired", "command", "run", "dialtone_mod", "codex-view", "codex-view:0:1", body)
	if err != nil {
		t.Fatalf("EnqueueShellBus returned error: %v", err)
	}
	if err := modstate.UpdateShellBusStatus(db, rowID, "running", 0, body); err != nil {
		t.Fatalf("UpdateShellBusStatus returned error: %v", err)
	}

	ok, err := ShouldExecuteDirectInPane(db, []string{"ssh", "v1", "test"}, true)
	if err != nil {
		t.Fatalf("ShouldExecuteDirectInPane returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected direct execution inside dialtone-view for the running queued command")
	}
}

func TestShouldExecuteDirectInPaneSkipsOutsideTmux(t *testing.T) {
	db, err := modstate.Open(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	ok, err := ShouldExecuteDirectInPane(db, []string{"ssh", "v1", "test"}, false)
	if err != nil {
		t.Fatalf("ShouldExecuteDirectInPane returned error: %v", err)
	}
	if ok {
		t.Fatalf("expected direct execution to be disabled outside tmux")
	}
}

package modstate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultStateDirUsesUserHome(t *testing.T) {
	t.Setenv("HOME", "/tmp/dialtone-home")
	if got := DefaultStateDir("/tmp/repo"); got != "/tmp/dialtone-home/.dialtone" {
		t.Fatalf("unexpected default state dir: %q", got)
	}
	if got := DefaultDBPath("/tmp/repo"); got != "/tmp/dialtone-home/.dialtone/state.sqlite" {
		t.Fatalf("unexpected default db path: %q", got)
	}
}

func TestHasGoPackageIgnoresTestOnlyDirectories(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main_test.go"), "package example_test\n")
	if hasGoPackage(dir) {
		t.Fatalf("expected test-only directory to be treated as non-runnable")
	}
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n")
	if !hasGoPackage(dir) {
		t.Fatalf("expected non-test go file to be treated as runnable")
	}
}

func TestSyncRepoPersistsModsDependenciesAndEnv(t *testing.T) {
	repoRoot := t.TempDir()
	writeFile(t, filepath.Join(repoRoot, "dialtone_mod"), "#!/bin/sh\nexit 0\n")
	writeFile(t, filepath.Join(repoRoot, "src", "go.mod"), "module example\n\ngo 1.25\n")
	writeFile(t, filepath.Join(repoRoot, "src", "mods", "ghostty", "v1", "main.go"), "package main\n")
	writeFile(t, filepath.Join(repoRoot, "src", "mods", "ghostty", "v1", "nix.packages"), "nixpkgs#go_1_25\nnixpkgs#tmux\n")
	writeFile(t, filepath.Join(repoRoot, "src", "mods", "shell", "v1", "main.go"), "package main\n")
	writeFile(t, filepath.Join(repoRoot, "src", "mods", "shell", "v1", "cli", "main.go"), "package main\n")
	writeFile(t, filepath.Join(repoRoot, "src", "mods", "shell", "v1", "README.md"), "# shell\n")
	writeFile(t, filepath.Join(repoRoot, "src", "mods", "shell", "v1", "nix.packages"), "darwin:nixpkgs#ghostty\n")
	writeFile(t, filepath.Join(repoRoot, "src", "mods", "shell", "v1", "mod.json"), `{
  "name": "shell",
  "version": "v1",
  "depends_on": [
    { "name": "ghostty", "version": "v1" }
  ],
  "env_vars": [
    { "name": "DIALTONE_REPO_ROOT", "required": true },
    { "name": "DIALTONE_STATE_DB", "required": true }
  ],
  "testing": {
    "requires_nix": true,
    "serial_group": "desktop",
    "visible_tmux": true
  }
}`)

	db, err := Open(filepath.Join(repoRoot, ".dialtone", "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	summary, err := SyncRepo(db, repoRoot, map[string]string{
		"DIALTONE_REPO_ROOT": repoRoot,
		"DIALTONE_STATE_DB":  filepath.Join(repoRoot, ".dialtone", "state.sqlite"),
	})
	if err != nil {
		t.Fatalf("SyncRepo returned error: %v", err)
	}
	if summary.Mods != 2 || summary.Dependencies != 1 || summary.NixPackages != 3 || summary.EnvVars != 2 || summary.Manifests != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	var modCount int
	if err := db.QueryRow(`select count(*) from mods`).Scan(&modCount); err != nil {
		t.Fatalf("count mods: %v", err)
	}
	if modCount != 2 {
		t.Fatalf("expected 2 mods, got %d", modCount)
	}

	var serialGroup string
	if err := db.QueryRow(`select serial_group from mod_test_policies where mod_name = 'shell' and mod_version = 'v1'`).Scan(&serialGroup); err != nil {
		t.Fatalf("select serial_group: %v", err)
	}
	if serialGroup != "desktop" {
		t.Fatalf("unexpected serial group: %q", serialGroup)
	}

	envRows, err := LoadRuntimeEnv(db, "process")
	if err != nil {
		t.Fatalf("LoadRuntimeEnv returned error: %v", err)
	}
	if len(envRows) != 2 {
		t.Fatalf("expected 2 env rows, got %d", len(envRows))
	}

	edges, err := LoadGraph(db)
	if err != nil {
		t.Fatalf("LoadGraph returned error: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 graph edge, got %d", len(edges))
	}
	if edges[0].From.Name != "shell" || edges[0].To.Name != "ghostty" {
		t.Fatalf("unexpected edge: %+v", edges[0])
	}
}

func TestRenderGraphMermaidAndText(t *testing.T) {
	edges := []GraphEdge{
		{
			From:   GraphNode{Name: "shell", Version: "v1"},
			To:     GraphNode{Name: "tmux", Version: "v1"},
			Source: "mod.json",
		},
	}
	mermaid := RenderGraphMermaid(edges)
	if !strings.Contains(mermaid, "graph TD") || !strings.Contains(mermaid, "shell_v1 --> tmux_v1") {
		t.Fatalf("unexpected mermaid output: %q", mermaid)
	}
	text := RenderGraphText(edges)
	if !strings.Contains(text, "shell:v1 -> tmux:v1 (mod.json)") {
		t.Fatalf("unexpected text output: %q", text)
	}
}

func TestRenderGraphOutline(t *testing.T) {
	topology := []TopologyRecord{
		{ModName: "ghostty", ModVersion: "v1", TopoRank: 0},
		{ModName: "tmux", ModVersion: "v1", TopoRank: 1},
		{ModName: "shell", ModVersion: "v1", TopoRank: 2},
		{ModName: "tsnet", ModVersion: "v1", TopoRank: 3},
	}
	edges := []GraphEdge{
		{
			From:   GraphNode{Name: "shell", Version: "v1"},
			To:     GraphNode{Name: "ghostty", Version: "v1"},
			Source: "mod.json",
		},
		{
			From:   GraphNode{Name: "shell", Version: "v1"},
			To:     GraphNode{Name: "tmux", Version: "v1"},
			Source: "mod.json",
		},
	}
	outline := RenderGraphOutline(topology, edges)
	if !strings.Contains(outline, "- shell:v1\n  - ghostty:v1\n  - tmux:v1") {
		t.Fatalf("unexpected outline output: %q", outline)
	}
	if !strings.Contains(outline, "- tsnet:v1") {
		t.Fatalf("expected isolated root in outline output: %q", outline)
	}
	if strings.Contains(outline, "\n- ghostty:v1\n") || strings.HasPrefix(outline, "- ghostty:v1\n") {
		t.Fatalf("expected dependency node to be nested, got %q", outline)
	}
}

func TestStateValuesRoundTrip(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	if err := UpsertStateValue(db, "system", "tmux.target", "codex-view:0:0"); err != nil {
		t.Fatalf("UpsertStateValue returned error: %v", err)
	}
	record, ok, err := LoadStateValue(db, "system", "tmux.target")
	if err != nil {
		t.Fatalf("LoadStateValue returned error: %v", err)
	}
	if !ok || record.Value != "codex-view:0:0" {
		t.Fatalf("unexpected state record: ok=%v record=%+v", ok, record)
	}
	values, err := LoadStateValues(db, "system")
	if err != nil {
		t.Fatalf("LoadStateValues returned error: %v", err)
	}
	if len(values) != 1 || values[0].Key != "tmux.target" {
		t.Fatalf("unexpected state values: %+v", values)
	}
	if err := DeleteStateValue(db, "system", "tmux.target"); err != nil {
		t.Fatalf("DeleteStateValue returned error: %v", err)
	}
	_, ok, err = LoadStateValue(db, "system", "tmux.target")
	if err != nil {
		t.Fatalf("LoadStateValue returned error: %v", err)
	}
	if ok {
		t.Fatalf("expected deleted state value to be missing")
	}
}

func TestCommandQueueLifecycle(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	id, err := EnqueueCommand(db, "tmux", "send", "codex-view:0:0", "mods v1 help", `{"source":"test"}`)
	if err != nil {
		t.Fatalf("EnqueueCommand returned error: %v", err)
	}
	if err := MarkCommandStarted(db, id); err != nil {
		t.Fatalf("MarkCommandStarted returned error: %v", err)
	}
	if err := MarkCommandFinished(db, id, "done", "ok", ""); err != nil {
		t.Fatalf("MarkCommandFinished returned error: %v", err)
	}
	queue, err := LoadQueue(db, "tmux", 10)
	if err != nil {
		t.Fatalf("LoadQueue returned error: %v", err)
	}
	if len(queue) != 1 {
		t.Fatalf("expected 1 queued command, got %d", len(queue))
	}
	if queue[0].ID != id || queue[0].Status != "done" || queue[0].Target != "codex-view:0:0" {
		t.Fatalf("unexpected queue record: %+v", queue[0])
	}
}

func TestProtocolRunLifecycle(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	runID, err := StartProtocolRun(db, "demo-protocol", "prompt text", "codex-view:0:0", "codex-view:0:1")
	if err != nil {
		t.Fatalf("StartProtocolRun returned error: %v", err)
	}
	if err := AppendProtocolEvent(db, ProtocolEventRecord{
		RunID:       runID,
		EventIndex:  1,
		EventType:   "prompt_submitted",
		PaneTarget:  "codex-view:0:0",
		MessageText: "submitted prompt",
	}); err != nil {
		t.Fatalf("AppendProtocolEvent prompt returned error: %v", err)
	}
	if err := AppendProtocolEvent(db, ProtocolEventRecord{
		RunID:       runID,
		EventIndex:  2,
		EventType:   "command_written",
		QueueName:   "tmux",
		PaneTarget:  "codex-view:0:1",
		CommandText: "./dialtone_mod mods v1 db graph --format outline",
	}); err != nil {
		t.Fatalf("AppendProtocolEvent command returned error: %v", err)
	}
	if err := FinishProtocolRun(db, runID, "passed", "outline rendered", ""); err != nil {
		t.Fatalf("FinishProtocolRun returned error: %v", err)
	}
	runs, err := LoadProtocolRuns(db, 10)
	if err != nil {
		t.Fatalf("LoadProtocolRuns returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != runID || runs[0].Status != "passed" {
		t.Fatalf("unexpected protocol runs: %+v", runs)
	}
	events, err := LoadProtocolEvents(db, runID)
	if err != nil {
		t.Fatalf("LoadProtocolEvents returned error: %v", err)
	}
	if len(events) != 2 || events[1].CommandText != "./dialtone_mod mods v1 db graph --format outline" {
		t.Fatalf("unexpected protocol events: %+v", events)
	}
}

func TestShellBusLifecycle(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	id, err := EnqueueShellBus(db, "shell", "desired", "prompt", "submit", "controller", "codex-view", "codex-view:0:0", `{"text":"hello"}`)
	if err != nil {
		t.Fatalf("EnqueueShellBus returned error: %v", err)
	}
	queued, err := LoadQueuedShellBus(db, 10)
	if err != nil {
		t.Fatalf("LoadQueuedShellBus returned error: %v", err)
	}
	if len(queued) != 1 || queued[0].ID != id || queued[0].Status != "queued" {
		t.Fatalf("unexpected queued shell bus rows: %+v", queued)
	}
	if err := UpdateShellBusStatus(db, id, "done", 99, `{"result":"ok"}`); err != nil {
		t.Fatalf("UpdateShellBusStatus returned error: %v", err)
	}
	if _, err := AppendShellBusObserved(db, "tmux", "pane", "snapshot", "sync", "codex-view", "codex-view:0:0", id, `{"text":"hello"}`); err != nil {
		t.Fatalf("AppendShellBusObserved returned error: %v", err)
	}
	rows, err := LoadShellBus(db, "", 10)
	if err != nil {
		t.Fatalf("LoadShellBus returned error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 shell bus rows, got %d", len(rows))
	}
	if rows[0].Scope != "observed" || rows[1].Status != "done" || rows[1].RefID != 99 {
		t.Fatalf("unexpected shell bus rows: %+v", rows)
	}
	record, ok, err := LoadShellBusRecord(db, id)
	if err != nil {
		t.Fatalf("LoadShellBusRecord returned error: %v", err)
	}
	if !ok || record.Status != "done" || record.RefID != 99 {
		t.Fatalf("unexpected shell bus record: ok=%v record=%+v", ok, record)
	}
}

func TestBuildTopologyAndTestPlan(t *testing.T) {
	mods := []ModRecord{
		{Name: "ghostty", Version: "v1", Path: "src/mods/ghostty/v1"},
		{Name: "tmux", Version: "v1", Path: "src/mods/tmux/v1"},
		{Name: "shell", Version: "v1", Path: "src/mods/shell/v1"},
	}
	deps := []DependencyRecord{
		{FromName: "shell", FromVersion: "v1", ToName: "ghostty", ToVersion: "v1"},
		{FromName: "shell", FromVersion: "v1", ToName: "tmux", ToVersion: "v1"},
	}
	topology, err := BuildTopology(mods, deps)
	if err != nil {
		t.Fatalf("BuildTopology returned error: %v", err)
	}
	if len(topology) != 3 {
		t.Fatalf("expected 3 topology rows, got %d", len(topology))
	}
	if topology[2].ModName != "shell" || topology[2].TopoRank != 2 {
		t.Fatalf("expected shell last in topology, got %+v", topology)
	}
	plan := BuildTestPlan(mods, map[string]Manifest{
		"ghostty:v1": {Testing: TestPolicy{RequiresNix: true, SerialGroup: "desktop", VisibleTmux: true}},
		"tmux:v1":    {Testing: TestPolicy{RequiresNix: true, SerialGroup: "desktop", VisibleTmux: true}},
		"shell:v1":   {Testing: TestPolicy{RequiresNix: true, SerialGroup: "desktop", VisibleTmux: true}},
	}, topology)
	if len(plan) != 3 {
		t.Fatalf("expected 3 plan steps, got %d", len(plan))
	}
	if plan[2].ModName != "shell" || plan[2].CommandText != "go test ./mods/shell/v1/..." {
		t.Fatalf("unexpected final plan step: %+v", plan[2])
	}
}

func TestBuildTopologyRejectsCycles(t *testing.T) {
	mods := []ModRecord{
		{Name: "ghostty", Version: "v1", Path: "src/mods/ghostty/v1"},
		{Name: "tmux", Version: "v1", Path: "src/mods/tmux/v1"},
	}
	deps := []DependencyRecord{
		{FromName: "ghostty", FromVersion: "v1", ToName: "tmux", ToVersion: "v1"},
		{FromName: "tmux", FromVersion: "v1", ToName: "ghostty", ToVersion: "v1"},
	}
	_, err := BuildTopology(mods, deps)
	if err == nil {
		t.Fatalf("expected cycle detection error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("unexpected cycle error: %v", err)
	}
}

func TestSyncRepoPersistsTopologyAndTestSteps(t *testing.T) {
	repoRoot := t.TempDir()
	writeFile(t, filepath.Join(repoRoot, "dialtone_mod"), "#!/bin/sh\nexit 0\n")
	writeFile(t, filepath.Join(repoRoot, "src", "go.mod"), "module example\n\ngo 1.25\n")
	writeFile(t, filepath.Join(repoRoot, "src", "mods", "ghostty", "v1", "main.go"), "package main\n")
	writeFile(t, filepath.Join(repoRoot, "src", "mods", "ghostty", "v1", "mod.json"), `{"name":"ghostty","version":"v1","testing":{"requires_nix":true,"serial_group":"desktop","visible_tmux":true}}`)
	writeFile(t, filepath.Join(repoRoot, "src", "mods", "shell", "v1", "main.go"), "package main\n")
	writeFile(t, filepath.Join(repoRoot, "src", "mods", "shell", "v1", "mod.json"), `{"name":"shell","version":"v1","depends_on":[{"name":"ghostty","version":"v1"}],"testing":{"requires_nix":true,"serial_group":"desktop","visible_tmux":true}}`)

	db, err := Open(filepath.Join(repoRoot, ".dialtone", "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	summary, err := SyncRepo(db, repoRoot, map[string]string{})
	if err != nil {
		t.Fatalf("SyncRepo returned error: %v", err)
	}
	if summary.Topology != 2 || summary.TestSteps != 2 {
		t.Fatalf("unexpected topology summary: %+v", summary)
	}
	topology, err := LoadTopology(db)
	if err != nil {
		t.Fatalf("LoadTopology returned error: %v", err)
	}
	if len(topology) != 2 || topology[1].ModName != "shell" {
		t.Fatalf("unexpected persisted topology: %+v", topology)
	}
	plan, err := LoadTestPlan(db, "default")
	if err != nil {
		t.Fatalf("LoadTestPlan returned error: %v", err)
	}
	if len(plan) != 2 || plan[1].CommandText != "go test ./mods/shell/v1/..." {
		t.Fatalf("unexpected persisted test plan: %+v", plan)
	}
}

func TestSyncRepoPreservesStateValues(t *testing.T) {
	repoRoot := t.TempDir()
	writeFile(t, filepath.Join(repoRoot, "dialtone_mod"), "#!/bin/sh\nexit 0\n")
	writeFile(t, filepath.Join(repoRoot, "src", "go.mod"), "module example\n\ngo 1.25\n")
	writeFile(t, filepath.Join(repoRoot, "src", "mods", "ghostty", "v1", "cli", "main.go"), "package main\n")
	writeFile(t, filepath.Join(repoRoot, "src", "mods", "ghostty", "v1", "mod.json"), `{"name":"ghostty","version":"v1"}`)
	writeFile(t, filepath.Join(repoRoot, "src", "mods", "shell", "v1", "cli", "main.go"), "package main\n")
	writeFile(t, filepath.Join(repoRoot, "src", "mods", "shell", "v1", "mod.json"), `{"name":"shell","version":"v1","depends_on":[{"name":"ghostty","version":"v1"}]}`)

	db, err := Open(filepath.Join(repoRoot, ".dialtone", "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	if err := UpsertStateValue(db, "system", "tmux.target", "codex-view:0:1"); err != nil {
		t.Fatalf("set tmux.target: %v", err)
	}
	if err := UpsertStateValue(db, "system", "tmux.prompt_target", "codex-view:0:0"); err != nil {
		t.Fatalf("set tmux.prompt_target: %v", err)
	}

	if _, err := SyncRepo(db, repoRoot, map[string]string{}); err != nil {
		t.Fatalf("SyncRepo returned error: %v", err)
	}

	commandTarget, ok, err := LoadStateValue(db, "system", "tmux.target")
	if err != nil {
		t.Fatalf("LoadStateValue command target returned error: %v", err)
	}
	if !ok || commandTarget.Value != "codex-view:0:1" {
		t.Fatalf("unexpected command target after sync: ok=%v record=%+v", ok, commandTarget)
	}

	promptTarget, ok, err := LoadStateValue(db, "system", "tmux.prompt_target")
	if err != nil {
		t.Fatalf("LoadStateValue prompt target returned error: %v", err)
	}
	if !ok || promptTarget.Value != "codex-view:0:0" {
		t.Fatalf("unexpected prompt target after sync: ok=%v record=%+v", ok, promptTarget)
	}
}

func TestCaptureRuntimeEnvFiltersVolatileKeys(t *testing.T) {
	t.Setenv("DIALTONE_REPO_ROOT", "/tmp/repo")
	t.Setenv("DIALTONE_TMUX_PROXY_ACTIVE", "1")
	t.Setenv("DIALTONE_TMUX_TARGET", "codex-view:0:0")
	t.Setenv("DIALTONE_NIX_ACTIVE", "1")
	t.Setenv("NIXPKGS_FLAKE", "nixpkgs")

	env := CaptureRuntimeEnv()
	if env["DIALTONE_REPO_ROOT"] != "/tmp/repo" {
		t.Fatalf("expected persistent env key to be captured, got %+v", env)
	}
	if _, ok := env["DIALTONE_TMUX_PROXY_ACTIVE"]; ok {
		t.Fatalf("volatile tmux proxy flag should not be captured: %+v", env)
	}
	if _, ok := env["DIALTONE_TMUX_TARGET"]; ok {
		t.Fatalf("tmux target should be managed in sqlite state, not runtime env: %+v", env)
	}
	if _, ok := env["DIALTONE_NIX_ACTIVE"]; ok {
		t.Fatalf("volatile nix flag should not be captured: %+v", env)
	}
	if env["NIXPKGS_FLAKE"] != "nixpkgs" {
		t.Fatalf("expected NIXPKGS_FLAKE to be captured, got %+v", env)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

package main

import (
	"path/filepath"
	"testing"

	"dialtone/dev/internal/modstate"
)

func TestRunDBEnvSetAndUnset(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("DIALTONE_REPO_ROOT", repoRoot)
	t.Setenv("DIALTONE_STATE_DB", filepath.Join(repoRoot, ".dialtone", "state.sqlite"))
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "dialtone_mod"), "#!/bin/sh\nexit 0\n")
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "src", "go.mod"), "module example\n\ngo 1.25\n")

	if err := runDBEnv([]string{"--set", "DIALTONE_FOO=bar"}); err != nil {
		t.Fatalf("runDBEnv --set returned error: %v", err)
	}

	db, err := modstate.Open(filepath.Join(repoRoot, ".dialtone", "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	rows, err := modstate.LoadRuntimeEnv(db, "process")
	if err != nil {
		t.Fatalf("LoadRuntimeEnv returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].Key != "DIALTONE_FOO" || rows[0].Value != "bar" {
		t.Fatalf("unexpected runtime env rows after set: %+v", rows)
	}

	if err := runDBEnv([]string{"--unset", "DIALTONE_FOO"}); err != nil {
		t.Fatalf("runDBEnv --unset returned error: %v", err)
	}
	rows, err = modstate.LoadRuntimeEnv(db, "process")
	if err != nil {
		t.Fatalf("LoadRuntimeEnv returned error: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected runtime env to be empty after unset, got %+v", rows)
	}
}

func TestRunDBEnvRejectsConflictingFlags(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("DIALTONE_REPO_ROOT", repoRoot)
	t.Setenv("DIALTONE_STATE_DB", filepath.Join(repoRoot, ".dialtone", "state.sqlite"))
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "dialtone_mod"), "#!/bin/sh\nexit 0\n")
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "src", "go.mod"), "module example\n\ngo 1.25\n")

	err := runDBEnv([]string{"--set", "DIALTONE_FOO=bar", "--unset", "DIALTONE_FOO"})
	if err == nil {
		t.Fatalf("expected conflicting flags to fail")
	}
}

func TestRunDBStateSetAndUnset(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("DIALTONE_REPO_ROOT", repoRoot)
	t.Setenv("DIALTONE_STATE_DB", filepath.Join(repoRoot, ".dialtone", "state.sqlite"))
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "dialtone_mod"), "#!/bin/sh\nexit 0\n")
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "src", "go.mod"), "module example\n\ngo 1.25\n")

	if err := runDBState([]string{"--set", "tmux.target=codex-view:0:0"}); err != nil {
		t.Fatalf("runDBState --set returned error: %v", err)
	}

	db, err := modstate.Open(filepath.Join(repoRoot, ".dialtone", "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()
	record, ok, err := modstate.LoadStateValue(db, "system", "tmux.target")
	if err != nil {
		t.Fatalf("LoadStateValue returned error: %v", err)
	}
	if !ok || record.Value != "codex-view:0:0" {
		t.Fatalf("unexpected state record after set: ok=%v record=%+v", ok, record)
	}

	if err := runDBState([]string{"--unset", "tmux.target"}); err != nil {
		t.Fatalf("runDBState --unset returned error: %v", err)
	}
	_, ok, err = modstate.LoadStateValue(db, "system", "tmux.target")
	if err != nil {
		t.Fatalf("LoadStateValue returned error: %v", err)
	}
	if ok {
		t.Fatalf("expected state record to be deleted")
	}
}

func TestRunDBSyncPersistsTopologyAndPlan(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("DIALTONE_REPO_ROOT", repoRoot)
	t.Setenv("DIALTONE_STATE_DB", filepath.Join(repoRoot, ".dialtone", "state.sqlite"))
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "dialtone_mod"), "#!/bin/sh\nexit 0\n")
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "src", "go.mod"), "module example\n\ngo 1.25\n")
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "src", "mods", "ghostty", "v1", "main.go"), "package main\n")
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "src", "mods", "ghostty", "v1", "mod.json"), `{"name":"ghostty","version":"v1","testing":{"requires_nix":true,"serial_group":"desktop","visible_tmux":true}}`)
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "src", "mods", "shell", "v1", "main.go"), "package main\n")
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "src", "mods", "shell", "v1", "mod.json"), `{"name":"shell","version":"v1","depends_on":[{"name":"ghostty","version":"v1"}],"testing":{"requires_nix":true,"serial_group":"desktop","visible_tmux":true}}`)

	if err := runDBSync(nil); err != nil {
		t.Fatalf("runDBSync returned error: %v", err)
	}

	db, err := modstate.Open(filepath.Join(repoRoot, ".dialtone", "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	topology, err := modstate.LoadTopology(db)
	if err != nil {
		t.Fatalf("LoadTopology returned error: %v", err)
	}
	if len(topology) != 2 || topology[1].ModName != "shell" {
		t.Fatalf("unexpected topology after db sync: %+v", topology)
	}
	plan, err := modstate.LoadTestPlan(db, "default")
	if err != nil {
		t.Fatalf("LoadTestPlan returned error: %v", err)
	}
	if len(plan) != 2 || plan[1].CommandText != "go test ./mods/shell/v1" {
		t.Fatalf("unexpected test plan after db sync: %+v", plan)
	}
}

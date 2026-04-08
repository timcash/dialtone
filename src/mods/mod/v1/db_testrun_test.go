package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dialtone/dev/internal/modstate"
)

func TestRunDBTestRunPersistsRunStateAndReadmes(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("DIALTONE_REPO_ROOT", repoRoot)
	t.Setenv("DIALTONE_STATE_DB", filepath.Join(repoRoot, ".dialtone", "state.sqlite"))
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "dialtone_mod"), "#!/bin/sh\nexit 0\n")
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "src", "go.mod"), "module example\n\ngo 1.25\n")
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "src", "mods", "ghostty", "v1", "main.go"), "package main\n")
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "src", "mods", "ghostty", "v1", "mod.json"), `{"name":"ghostty","version":"v1","testing":{"requires_nix":false,"serial_group":"desktop","visible_tmux":true},"nix":{"flake_shell":"default"}}`)
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "src", "mods", "shell", "v1", "main.go"), "package main\n")
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "src", "mods", "shell", "v1", "mod.json"), `{"name":"shell","version":"v1","depends_on":[{"name":"ghostty","version":"v1"}],"testing":{"requires_nix":false,"serial_group":"desktop","visible_tmux":true},"nix":{"flake_shell":"default"}}`)

	if err := runDBTestRun([]string{"--name", "default"}); err != nil {
		t.Fatalf("runDBTestRun returned error: %v", err)
	}

	db, err := modstate.Open(filepath.Join(repoRoot, ".dialtone", "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	runs, err := loadTestRuns(db, 10)
	if err != nil {
		t.Fatalf("loadTestRuns returned error: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 test run, got %d", len(runs))
	}
	if runs[0].Status != "passed" || runs[0].PassedSteps != 2 || runs[0].FailedSteps != 0 {
		t.Fatalf("unexpected test run row: %+v", runs[0])
	}

	steps, err := loadTestRunSteps(db, runs[0].ID)
	if err != nil {
		t.Fatalf("loadTestRunSteps returned error: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 test steps, got %d", len(steps))
	}
	if steps[0].ModName != "ghostty" || steps[0].Status != "passed" {
		t.Fatalf("unexpected first test step: %+v", steps[0])
	}
	if steps[1].ModName != "shell" || steps[1].Status != "passed" {
		t.Fatalf("unexpected second test step: %+v", steps[1])
	}

	queueRows, err := modstate.LoadQueue(db, "tests", 10)
	if err != nil {
		t.Fatalf("LoadQueue returned error: %v", err)
	}
	if len(queueRows) != 2 {
		t.Fatalf("expected 2 queued test commands, got %d", len(queueRows))
	}
	if queueRows[0].Status != "done" || queueRows[1].Status != "done" {
		t.Fatalf("expected finished test queue rows, got %+v", queueRows)
	}

	ghosttyReadme, err := os.ReadFile(filepath.Join(repoRoot, "src", "mods", "ghostty", "v1", "README.md"))
	if err != nil {
		t.Fatalf("ReadFile ghostty README returned error: %v", err)
	}
	if !strings.Contains(string(ghosttyReadme), "## Quick Start") || !strings.Contains(string(ghosttyReadme), "## Test Results") {
		t.Fatalf("expected README sections to be written, got:\n%s", ghosttyReadme)
	}
	if !strings.Contains(string(ghosttyReadme), "./dialtone_mod ghostty v1 help") {
		t.Fatalf("expected README quick start to mention ghostty help, got:\n%s", ghosttyReadme)
	}
}

func TestRunDBTestRunStopsOnFirstFailure(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("DIALTONE_REPO_ROOT", repoRoot)
	t.Setenv("DIALTONE_STATE_DB", filepath.Join(repoRoot, ".dialtone", "state.sqlite"))
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "dialtone_mod"), "#!/bin/sh\nexit 0\n")
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "src", "go.mod"), "module example\n\ngo 1.25\n")
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "src", "mods", "ghostty", "v1", "main.go"), "package main\n\nfunc broken(\n")
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "src", "mods", "ghostty", "v1", "mod.json"), `{"name":"ghostty","version":"v1","testing":{"requires_nix":false,"serial_group":"desktop","visible_tmux":true},"nix":{"flake_shell":"default"}}`)
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "src", "mods", "shell", "v1", "main.go"), "package main\n")
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "src", "mods", "shell", "v1", "mod.json"), `{"name":"shell","version":"v1","depends_on":[{"name":"ghostty","version":"v1"}],"testing":{"requires_nix":false,"serial_group":"desktop","visible_tmux":true},"nix":{"flake_shell":"default"}}`)

	if err := runDBTestRun([]string{"--name", "default", "--update-readmes=false"}); err != nil {
		t.Fatalf("runDBTestRun returned error: %v", err)
	}

	db, err := modstate.Open(filepath.Join(repoRoot, ".dialtone", "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	runs, err := loadTestRuns(db, 10)
	if err != nil {
		t.Fatalf("loadTestRuns returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != "failed" || runs[0].FailedSteps != 1 {
		t.Fatalf("unexpected failed test run rows: %+v", runs)
	}
	steps, err := loadTestRunSteps(db, runs[0].ID)
	if err != nil {
		t.Fatalf("loadTestRunSteps returned error: %v", err)
	}
	if len(steps) != 1 || steps[0].Status != "failed" {
		t.Fatalf("expected one failed step, got %+v", steps)
	}
}

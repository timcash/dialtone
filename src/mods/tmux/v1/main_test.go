package main

import (
	"errors"
	"os"
	"testing"
)

func TestPersistedTargetRoundTripUsesSQLiteState(t *testing.T) {
	repoRoot := t.TempDir()
	if err := storePersistedTarget(repoRoot, "codex-view:0:0"); err != nil {
		t.Fatalf("storePersistedTarget returned error: %v", err)
	}
	value, err := loadPersistedTarget(repoRoot)
	if err != nil {
		t.Fatalf("loadPersistedTarget returned error: %v", err)
	}
	if value != "codex-view:0:0" {
		t.Fatalf("unexpected persisted target: %q", value)
	}
}

func TestClearPersistedTargetRemovesSQLiteState(t *testing.T) {
	repoRoot := t.TempDir()
	if err := storePersistedTarget(repoRoot, "codex-view:0:0"); err != nil {
		t.Fatalf("storePersistedTarget returned error: %v", err)
	}
	if err := clearPersistedTarget(repoRoot); err != nil {
		t.Fatalf("clearPersistedTarget returned error: %v", err)
	}
	_, err := loadPersistedTarget(repoRoot)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist after clear, got %v", err)
	}
}

func TestPersistedPromptTargetRoundTripUsesSQLiteState(t *testing.T) {
	repoRoot := t.TempDir()
	if err := storePersistedPromptTarget(repoRoot, "codex-view:0:0"); err != nil {
		t.Fatalf("storePersistedPromptTarget returned error: %v", err)
	}
	value, err := loadPersistedPromptTarget(repoRoot)
	if err != nil {
		t.Fatalf("loadPersistedPromptTarget returned error: %v", err)
	}
	if value != "codex-view:0:0" {
		t.Fatalf("unexpected persisted prompt target: %q", value)
	}
}

func TestClearPersistedPromptTargetRemovesSQLiteState(t *testing.T) {
	repoRoot := t.TempDir()
	if err := storePersistedPromptTarget(repoRoot, "codex-view:0:0"); err != nil {
		t.Fatalf("storePersistedPromptTarget returned error: %v", err)
	}
	if err := clearPersistedPromptTarget(repoRoot); err != nil {
		t.Fatalf("clearPersistedPromptTarget returned error: %v", err)
	}
	_, err := loadPersistedPromptTarget(repoRoot)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist after prompt clear, got %v", err)
	}
}

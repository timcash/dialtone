package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverModsPrefersSQLiteRegistry(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("DIALTONE_STATE_DB", filepath.Join(repoRoot, ".dialtone", "state.sqlite"))
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "src", "mods", "ghostty", "v1", "main.go"), "package main\n")
	writeDiscoverTestFile(t, filepath.Join(repoRoot, "src", "mods", "shell", "v1", "main.go"), "package main\n")
	writeDiscoverTestFile(t, filepath.Join(repoRoot, ".gitmodules"), `
[submodule "unused"]
	path = src/mods/zzz
	url = git@github.com:example/zzz.git
`)

	mods, err := discoverMods(repoRoot)
	if err != nil {
		t.Fatalf("discoverMods returned error: %v", err)
	}
	if len(mods) != 2 {
		t.Fatalf("expected 2 mods, got %d", len(mods))
	}
	if mods[0].Name != "ghostty" || mods[0].Path != "src/mods/ghostty" {
		t.Fatalf("unexpected first mod: %+v", mods[0])
	}
	if mods[1].Name != "shell" || mods[1].Path != "src/mods/shell" {
		t.Fatalf("unexpected second mod: %+v", mods[1])
	}
}

func TestDiscoverModsFallsBackToGitmodulesWhenSQLiteScanFails(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("DIALTONE_STATE_DB", filepath.Join(repoRoot, ".dialtone", "state.sqlite"))
	writeDiscoverTestFile(t, filepath.Join(repoRoot, ".gitmodules"), `
[submodule "ghostty"]
	path = src/mods/ghostty
	url = git@github.com:example/ghostty.git
[submodule "tmux"]
	path = src/mods/tmux
	url = git@github.com:example/tmux.git
`)

	mods, err := discoverMods(repoRoot)
	if err != nil {
		t.Fatalf("discoverMods returned error: %v", err)
	}
	if len(mods) != 2 {
		t.Fatalf("expected 2 mods, got %d", len(mods))
	}
	if mods[0].Path != "src/mods/ghostty" || mods[1].Path != "src/mods/tmux" {
		t.Fatalf("unexpected fallback mods: %+v", mods)
	}
}

func writeDiscoverTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

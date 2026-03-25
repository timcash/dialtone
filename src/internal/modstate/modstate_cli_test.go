package modstate

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSyncRepoMarksAllRealModsWithCLIWrappers(t *testing.T) {
	repoRoot := currentRepoRoot(t)
	db := openTestStateDB(t)
	if _, err := SyncRepo(db, repoRoot, nil); err != nil {
		t.Fatalf("SyncRepo returned error: %v", err)
	}

	records, err := LoadMods(db)
	if err != nil {
		t.Fatalf("LoadMods returned error: %v", err)
	}

	expected := map[string]string{
		"chrome":   "v1",
		"codex":    "v1",
		"db":       "v1",
		"dialtone": "v1",
		"ghostty":  "v1",
		"mesh":     "v3",
		"mod":      "v1",
		"mosh":     "v1",
		"shell":    "v1",
		"ssh":      "v1",
		"test":     "v1",
		"tmux":     "v1",
		"tsnet":    "v1",
	}

	seen := map[string]bool{}
	for _, record := range records {
		version, ok := expected[record.Name]
		if !ok || record.Version != version {
			continue
		}
		seen[record.Name] = true
		if !record.HasCLI {
			t.Fatalf("expected %s %s to have cli/main.go", record.Name, record.Version)
		}
	}
	for name := range expected {
		if !seen[name] {
			t.Fatalf("expected mod %s to be present in sqlite registry", name)
		}
	}
}

func TestResolveEntrypointUsesCLIWrapperForAllRealMods(t *testing.T) {
	repoRoot := currentRepoRoot(t)
	srcRoot := filepath.Join(repoRoot, "src")
	db := openTestStateDB(t)
	if _, err := SyncRepo(db, repoRoot, nil); err != nil {
		t.Fatalf("SyncRepo returned error: %v", err)
	}

	for _, tc := range []struct {
		name    string
		version string
	}{
		{name: "chrome", version: "v1"},
		{name: "codex", version: "v1"},
		{name: "db", version: "v1"},
		{name: "dialtone", version: "v1"},
		{name: "ghostty", version: "v1"},
		{name: "mesh", version: "v3"},
		{name: "mod", version: "v1"},
		{name: "mosh", version: "v1"},
		{name: "shell", version: "v1"},
		{name: "ssh", version: "v1"},
		{name: "test", version: "v1"},
		{name: "tmux", version: "v1"},
		{name: "tsnet", version: "v1"},
	} {
		entry, err := ResolveEntrypoint(db, srcRoot, tc.name, tc.version, "help")
		if err != nil {
			t.Fatalf("ResolveEntrypoint(%s %s) returned error: %v", tc.name, tc.version, err)
		}
		if !strings.HasSuffix(filepath.ToSlash(entry.Path), "/cli") {
			t.Fatalf("expected %s %s entrypoint to resolve to cli wrapper, got %q", tc.name, tc.version, entry.Path)
		}
	}
}

func openTestStateDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func currentRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

package sqlitestate

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"dialtone/dev/internal/modstate"
)

func TestUpsertAndDeleteRuntimeEnv(t *testing.T) {
	db := openTestDB(t)
	if err := UpsertRuntimeEnv(db, "process", "DIALTONE_FOO", "bar"); err != nil {
		t.Fatalf("UpsertRuntimeEnv returned error: %v", err)
	}
	rows, err := modstate.LoadRuntimeEnv(db, "process")
	if err != nil {
		t.Fatalf("LoadRuntimeEnv returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].Key != "DIALTONE_FOO" || rows[0].Value != "bar" {
		t.Fatalf("unexpected rows after upsert: %+v", rows)
	}
	if err := DeleteRuntimeEnv(db, "process", "DIALTONE_FOO"); err != nil {
		t.Fatalf("DeleteRuntimeEnv returned error: %v", err)
	}
	rows, err = modstate.LoadRuntimeEnv(db, "process")
	if err != nil {
		t.Fatalf("LoadRuntimeEnv returned error: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected no rows after delete, got %+v", rows)
	}
}

func TestLoadRuntimeEnvValue(t *testing.T) {
	db := openTestDB(t)
	if err := UpsertRuntimeEnv(db, ProcessScope, "DIALTONE_FOO", "bar"); err != nil {
		t.Fatalf("UpsertRuntimeEnv returned error: %v", err)
	}
	value, ok, err := LoadRuntimeEnvValue(db, ProcessScope, "DIALTONE_FOO")
	if err != nil {
		t.Fatalf("LoadRuntimeEnvValue returned error: %v", err)
	}
	if !ok || value != "bar" {
		t.Fatalf("unexpected lookup result: ok=%v value=%q", ok, value)
	}
}

func TestHydrateRuntimeEnvPreservesExistingValuesByDefault(t *testing.T) {
	db := openTestDB(t)
	if err := UpsertRuntimeEnv(db, "process", "DIALTONE_FOO", "from-db"); err != nil {
		t.Fatalf("UpsertRuntimeEnv returned error: %v", err)
	}
	t.Setenv("DIALTONE_FOO", "from-env")
	count, err := HydrateRuntimeEnv(db, "process", false)
	if err != nil {
		t.Fatalf("HydrateRuntimeEnv returned error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 hydrated values, got %d", count)
	}
	if got := os.Getenv("DIALTONE_FOO"); got != "from-env" {
		t.Fatalf("expected env value to remain, got %q", got)
	}
}

func TestHydrateRuntimeEnvOverridesWhenRequested(t *testing.T) {
	db := openTestDB(t)
	if err := UpsertRuntimeEnv(db, "process", "DIALTONE_FOO", "from-db"); err != nil {
		t.Fatalf("UpsertRuntimeEnv returned error: %v", err)
	}
	if err := UpsertRuntimeEnv(db, "process", "OTHER_FOO", "ignored"); err != nil {
		t.Fatalf("UpsertRuntimeEnv returned error: %v", err)
	}
	t.Setenv("DIALTONE_FOO", "from-env")
	count, err := HydrateRuntimeEnv(db, "process", true)
	if err != nil {
		t.Fatalf("HydrateRuntimeEnv returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 hydrated value, got %d", count)
	}
	if got := os.Getenv("DIALTONE_FOO"); got != "from-db" {
		t.Fatalf("expected db value to win, got %q", got)
	}
	if got := os.Getenv("OTHER_FOO"); got != "" {
		t.Fatalf("expected non-DIALTONE key to be ignored, got %q", got)
	}
}

func TestHydrateRuntimeEnvIgnoresVolatileKeys(t *testing.T) {
	db := openTestDB(t)
	t.Setenv("DIALTONE_TMUX_PROXY_ACTIVE", "")
	if err := UpsertRuntimeEnv(db, ProcessScope, "DIALTONE_TMUX_PROXY_ACTIVE", "1"); err != nil {
		t.Fatalf("UpsertRuntimeEnv returned error: %v", err)
	}
	count, err := HydrateRuntimeEnv(db, ProcessScope, true)
	if err != nil {
		t.Fatalf("HydrateRuntimeEnv returned error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 hydrated values for volatile keys, got %d", count)
	}
	if got := os.Getenv("DIALTONE_TMUX_PROXY_ACTIVE"); got != "" {
		t.Fatalf("expected volatile env to remain unset, got %q", got)
	}
}

func TestResolveStatePathsPreferEnvironment(t *testing.T) {
	repoRoot := "/tmp/example"
	t.Setenv("DIALTONE_STATE_DIR", "/tmp/custom-state")
	t.Setenv("DIALTONE_STATE_DB", "/tmp/custom.sqlite")
	if got := ResolveStateDir(repoRoot); got != "/tmp/custom-state" {
		t.Fatalf("unexpected state dir: %q", got)
	}
	if got := ResolveStateDBPath(repoRoot); got != "/tmp/custom.sqlite" {
		t.Fatalf("unexpected state db: %q", got)
	}
}

func TestResolveStatePathsMakeRelativeEnvAbsoluteFromRepoRoot(t *testing.T) {
	repoRoot := "/tmp/example"
	t.Setenv("DIALTONE_STATE_DIR", ".dialtone")
	t.Setenv("DIALTONE_STATE_DB", ".dialtone/state.sqlite")
	if got := ResolveStateDir(repoRoot); got != "/tmp/example/.dialtone" {
		t.Fatalf("unexpected state dir for relative env: %q", got)
	}
	if got := ResolveStateDBPath(repoRoot); got != "/tmp/example/.dialtone/state.sqlite" {
		t.Fatalf("unexpected state db for relative env: %q", got)
	}
}

func TestParseAssignment(t *testing.T) {
	key, value, err := ParseAssignment("DIALTONE_FOO=bar=baz")
	if err != nil {
		t.Fatalf("ParseAssignment returned error: %v", err)
	}
	if key != "DIALTONE_FOO" || value != "bar=baz" {
		t.Fatalf("unexpected assignment parse: %q %q", key, value)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := modstate.Open(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

package sqlitestate

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dialtone/dev/internal/modstate"
)

const ProcessScope = "process"
const SystemScope = "system"
const TmuxTargetKey = "tmux.target"
const TmuxPromptTargetKey = "tmux.prompt_target"

func ResolveStateDir(repoRoot string) string {
	if value := strings.TrimSpace(os.Getenv("DIALTONE_STATE_DIR")); value != "" {
		return resolvePathAgainstRepo(repoRoot, value)
	}
	return modstate.DefaultStateDir(repoRoot)
}

func ResolveStateDBPath(repoRoot string) string {
	if value := strings.TrimSpace(os.Getenv("DIALTONE_STATE_DB")); value != "" {
		return resolvePathAgainstRepo(repoRoot, value)
	}
	return filepath.Join(ResolveStateDir(repoRoot), "state.sqlite")
}

func ParseAssignment(raw string) (string, string, error) {
	key, value, ok := strings.Cut(strings.TrimSpace(raw), "=")
	if !ok {
		return "", "", fmt.Errorf("expected KEY=VALUE assignment, got %q", raw)
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", "", fmt.Errorf("assignment key is required")
	}
	return key, value, nil
}

func UpsertRuntimeEnv(db *sql.DB, scope, key, value string) error {
	if err := modstate.EnsureSchema(db); err != nil {
		return err
	}
	_, err := db.Exec(`insert into runtime_env(scope, key, value, updated_at) values(?, ?, ?, ?)
		on conflict(scope, key) do update set value=excluded.value, updated_at=excluded.updated_at`,
		strings.TrimSpace(scope),
		strings.TrimSpace(key),
		value,
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func DeleteRuntimeEnv(db *sql.DB, scope, key string) error {
	if err := modstate.EnsureSchema(db); err != nil {
		return err
	}
	_, err := db.Exec(`delete from runtime_env where scope = ? and key = ?`, strings.TrimSpace(scope), strings.TrimSpace(key))
	return err
}

func LoadRuntimeEnvValue(db *sql.DB, scope, key string) (string, bool, error) {
	rows, err := modstate.LoadRuntimeEnv(db, strings.TrimSpace(scope))
	if err != nil {
		return "", false, err
	}
	targetKey := strings.TrimSpace(key)
	for _, row := range rows {
		if row.Key == targetKey {
			return row.Value, true, nil
		}
	}
	return "", false, nil
}

func HydrateRuntimeEnv(db *sql.DB, scope string, overwrite bool) (int, error) {
	rows, err := modstate.LoadRuntimeEnv(db, strings.TrimSpace(scope))
	if err != nil {
		return 0, err
	}
	count := 0
	for _, row := range rows {
		if !modstate.ShouldPersistRuntimeEnvKey(row.Key) {
			continue
		}
		if !overwrite && strings.TrimSpace(os.Getenv(row.Key)) != "" {
			continue
		}
		if err := os.Setenv(row.Key, row.Value); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func resolvePathAgainstRepo(repoRoot, raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	root := strings.TrimSpace(repoRoot)
	if root == "" {
		return filepath.Clean(value)
	}
	return filepath.Clean(filepath.Join(root, value))
}

//go:build !no_duckdb
package cli

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/marcboeker/go-duckdb"
)

const (
	nexttoneDBFilename = "nexttone.duckdb"
)

func nexttoneDBPath() string {
	if p := os.Getenv("NEXTTONE_DB_PATH"); p != "" {
		return p
	}
	return filepath.Join(toneRootDir(), toneName(), toneName()+".duckdb")
}

func initDBPath() string {
	return filepath.Join("src", "plugins", "nexttone", "init.duckdb")
}

func toneName() string {
	if name := os.Getenv("NEXTTONE_TONE"); name != "" {
		return name
	}
	return "default"
}

func toneRootDir() string {
	if dir := os.Getenv("NEXTTONE_TONE_DIR"); dir != "" {
		return dir
	}
	return filepath.Join("src", "nexttone")
}

func toneTestDir() string {
	return filepath.Join(toneRootDir(), toneName(), "test")
}

func openNexttoneDB() (*sql.DB, error) {
	return openNexttoneDBAt(nexttoneDBPath())
}

func openNexttoneDBAt(dbPath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		_ = copyInitDB(initDBPath(), dbPath)
	}
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, err
	}
	if err := ensureNexttoneSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := seedMicrotoneGraph(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func copyInitDB(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = out.ReadFrom(in)
	return err
}

func InitDB(path string) error {
	if path != "" {
		_ = os.Setenv("NEXTTONE_DB_PATH", path)
	}
	db, err := openNexttoneDB()
	if err != nil {
		return err
	}
	return db.Close()
}

func backupDB() error {
	// Backups removed: each tone keeps a single duckdb file.
	return nil
}

func ensureNexttoneSchema(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS nexttone_tones (
			name TEXT PRIMARY KEY,
			description TEXT,
			created_at TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS nexttone_subtone_conditions (
			subtone_name TEXT NOT NULL,
			position INTEGER NOT NULL,
			condition TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS nexttone_sessions (
			id TEXT PRIMARY KEY,
			current_microtone TEXT,
			current_subtone TEXT,
			updated_at TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS nexttone_microtones (
			name TEXT PRIMARY KEY,
			position INTEGER NOT NULL,
			question TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS nexttone_subtones (
			name TEXT PRIMARY KEY,
			position INTEGER NOT NULL,
			description TEXT,
			test_condition TEXT,
			test_command TEXT,
			depends_on TEXT,
			test_output TEXT,
			agent_notes TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS nexttone_microtone_edges (
			from_name TEXT NOT NULL,
			to_name TEXT NOT NULL,
			edge_type TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS nexttone_signatures (
			microtone TEXT NOT NULL,
			response TEXT NOT NULL,
			timestamp TEXT NOT NULL
		);`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	if err := ensureColumn(db, "nexttone_subtones", "test_condition", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "nexttone_subtones", "test_command", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "nexttone_subtones", "depends_on", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "nexttone_subtones", "test_output", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "nexttone_subtones", "agent_notes", "TEXT"); err != nil {
		return err
	}
	return nil
}

func ensureColumn(db *sql.DB, table, column, colType string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info('%s')", table))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull bool
		var dflt sql.NullString
		var pk bool
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, colType))
	return err
}

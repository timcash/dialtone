package cli

import (
	"database/sql"
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
	return filepath.Join("src", "nexttone", nexttoneDBFilename)
}

func openNexttoneDB() (*sql.DB, error) {
	dbPath := nexttoneDBPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
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

func ensureNexttoneSchema(db *sql.DB) error {
	statements := []string{
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
			description TEXT
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
	return nil
}

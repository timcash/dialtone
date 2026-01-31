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
	return filepath.Join("src", "tones")
}

func toneTestDir() string {
	return filepath.Join(toneRootDir(), toneName(), "test")
}

func openNexttoneDB() (*sql.DB, error) {
	dbPath := nexttoneDBPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		_ = copyIfExists(initDBPath(), dbPath)
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
	src := nexttoneDBPath()
	dstDir := toneTestDir()
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}
	dst := filepath.Join(dstDir, "nexttone_backup.duckdb")
	return copyIfExists(src, dst)
}

func copyIfExists(src, dst string) error {
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

package main

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	_ "github.com/marcboeker/go-duckdb"
)

const nexttoneToneEnv = "NEXTTONE_TONE"
const nexttoneToneDirEnv = "NEXTTONE_TONE_DIR"

var testDBPath = ""
var testToneName = "demo-tone-flow"
var testToneRoot = ""

func runCmd(name string, args ...string) string {
	logLine("cmd", fmt.Sprintf("%s %v", name, args))
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(),
		nexttoneToneEnv+"="+testToneName,
		nexttoneToneDirEnv+"="+testToneRoot,
	)
	output, _ := cmd.CombinedOutput()
	fmt.Print(string(output))
	return string(output)
}

func logLine(level, message string) {
	fmt.Printf("[%s] %s\n", level, message)
}

func setupToneDir() {
	base := filepath.Join("src", "nexttone", "tmp")
	if err := os.MkdirAll(base, 0755); err != nil {
		logLine("error", fmt.Sprintf("failed to create temp base: %v", err))
		os.Exit(1)
	}
	dir, err := os.MkdirTemp(base, "nexttone-tone-*")
	if err != nil {
		logLine("error", fmt.Sprintf("failed to create temp tone dir: %v", err))
		os.Exit(1)
	}
	testToneRoot = dir
	testDBPath = filepath.Join(testToneRoot, testToneName, testToneName+".duckdb")
}

func cleanupToneDir() {
	if testDB != nil {
		_ = testDB.Close()
		testDB = nil
	}
	if testToneRoot != "" {
		_ = os.RemoveAll(testToneRoot)
	}
}

func assertState(expectedMicrotone, expectedSubtone string) error {
	db, err := openTestDB()
	if err != nil {
		return err
	}

	var microtone string
	var subtone string
	if err := db.QueryRow(
		`SELECT current_microtone, current_subtone FROM nexttone_sessions WHERE id = 'default'`,
	).Scan(&microtone, &subtone); err != nil {
		return err
	}
	if microtone != expectedMicrotone {
		return fmt.Errorf("expected microtone %s, got %s", expectedMicrotone, microtone)
	}
	if subtone != expectedSubtone {
		return fmt.Errorf("expected subtone %s, got %s", expectedSubtone, subtone)
	}
	return nil
}

func assertDBExists() error {
	dbPath := filepath.Join(testToneRoot, testToneName, testToneName+".duckdb")
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("expected db at %s", dbPath)
	}
	return nil
}

func resetToneDB() error {
	dbPath := filepath.Join(testToneRoot, testToneName, testToneName+".duckdb")
	if testDB != nil {
		_ = testDB.Close()
		testDB = nil
	}
	if _, err := os.Stat(dbPath); err == nil {
		if err := os.Remove(dbPath); err != nil {
			return err
		}
	}
	runCmd("./dialtone.sh", "nexttone", "add", testToneName)
	if err := assertDBExists(); err != nil {
		return err
	}
	_, err := openTestDB()
	return err
}

func openTestDB() (*sql.DB, error) {
	if testDB != nil {
		return testDB, nil
	}
	db, err := sql.Open("duckdb", testDBPath)
	if err != nil {
		return nil, err
	}
	testDB = db
	return testDB, nil
}

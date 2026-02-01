package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func TestToneAddCreatesFiles() error {
	logLine("step", "Create tone folder + files")
	runCmd("./dialtone.sh", "nexttone", "add", testToneName)

	toneDir := filepath.Join(testToneRoot, testToneName)
	if _, err := os.Stat(toneDir); err != nil {
		return fmt.Errorf("expected tone folder at %s", toneDir)
	}
	testGo := filepath.Join(toneDir, "test", "test.go")
	if _, err := os.Stat(testGo); err != nil {
		return fmt.Errorf("expected test.go at %s", testGo)
	}
	if err := assertDBExists(); err != nil {
		return err
	}
	return nil
}

func TestInitDBSeedsMicrotones() error {
	logLine("step", "Verify init.duckdb seed + pointer advance")

	db, err := openTestDB()
	if err != nil {
		return err
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM nexttone_microtones`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("expected seeded microtones from init.duckdb")
	}
	if err := assertState("set-git-clean", "alpha"); err != nil {
		return err
	}

	runCmd("./dialtone.sh", "nexttone", "--sign", "yes")
	if err := assertState("align-goal-subtone-names", "alpha"); err != nil {
		return err
	}
	if err := resetToneDB(); err != nil {
		return err
	}
	return nil
}

func TestListShowsGraph() error {
	logLine("step", "List microtone graph")
	if err := resetToneDB(); err != nil {
		return err
	}
	output := runCmd("./dialtone.sh", "nexttone", "list")
	if !strings.Contains(output, "MICROTONE") || !strings.Contains(output, "CURRENT") {
		return fmt.Errorf("expected list output to include graph and current position")
	}
	if !strings.Contains(output, "SUBTONES") || !strings.Contains(output, "CURRENT SUBTONE") {
		return fmt.Errorf("expected list output to include subtone list and current subtone")
	}
	if err := assertState("set-git-clean", "alpha"); err != nil {
		return err
	}
	return nil
}

func TestNextRequiresSignature() error {
	logLine("step", "Require signature")
	if err := resetToneDB(); err != nil {
		return err
	}
	output := runCmd("./dialtone.sh", "nexttone")
	if !strings.Contains(output, "DIALTONE") {
		return fmt.Errorf("expected DIALTONE prompt output")
	}
	if !strings.Contains(output, "--sign yes") || !strings.Contains(output, "--sign no") {
		return fmt.Errorf("expected explicit --sign yes|no commands")
	}
	if !strings.Contains(output, "?") {
		return fmt.Errorf("expected a question in DIALTONE output")
	}
	if !strings.Contains(output, "DIALTONE [") {
		return fmt.Errorf("expected DIALTONE microtone tag")
	}
	if err := assertState("set-git-clean", "alpha"); err != nil {
		return err
	}
	return nil
}

func TestSignAdvancesMicrotone() error {
	logLine("step", "Sign advances microtone")
	if err := resetToneDB(); err != nil {
		return err
	}
	first := runCmd("./dialtone.sh", "nexttone")
	if !strings.Contains(first, "DIALTONE [") {
		return fmt.Errorf("expected microtone tag in first prompt")
	}
	after := runCmd("./dialtone.sh", "nexttone", "--sign", "yes")
	if strings.Contains(after, "DIALTONE [set-git-clean]") && strings.Contains(first, "DIALTONE [set-git-clean]") {
		return fmt.Errorf("expected microtone to advance after signing")
	}
	if !strings.Contains(after, "DIALTONE [") || !strings.Contains(after, "?") {
		return fmt.Errorf("expected next microtone prompt after signing")
	}
	if err := assertState("align-goal-subtone-names", "alpha"); err != nil {
		return err
	}
	if err := assertDBExists(); err != nil {
		return err
	}
	return nil
}

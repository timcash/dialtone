package main

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/marcboeker/go-duckdb"
)

const nexttoneDBEnv = "NEXTTONE_DB_PATH"
const nexttoneToneEnv = "NEXTTONE_TONE"
const nexttoneToneDirEnv = "NEXTTONE_TONE_DIR"

var testDBPath = tempDBPath()
var testToneName = "demo-tone"
var testToneRoot = ""

func main() {
	logLine("info", "Starting nexttone integration test")

	allPassed := true
	runTest := func(name string, fn func() error) {
		logLine("test", name)
		if err := fn(); err != nil {
			logLine("fail", fmt.Sprintf("%s - %v", name, err))
			allPassed = false
		} else {
			logLine("pass", name)
		}
	}

	defer func() {
		logLine("info", "Nexttone tests completed")
		if !allPassed {
			logLine("error", "Some tests failed")
			os.Exit(1)
		}
	}()

	setupToneDir()

	runTest("Nexttone list shows graph + cursor", TestListShowsGraph)
	runTest("Nexttone requires signature", TestNextRequiresSignature)
	runTest("Nexttone sign advances microtone", TestSignAdvancesMicrotone)
	runTest("Nexttone loops over subtones", TestLoopOverSubtones)
	runTest("Nexttone completes tone", TestCompleteTone)

	cleanupDB()
	cleanupToneDir()
	fmt.Println()
}

func TestListShowsGraph() error {
	logLine("step", "List microtone graph")
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
	if err := assertBackupExists(); err != nil {
		return err
	}
	return nil
}

func TestLoopOverSubtones() error {
	logLine("step", "Loop over subtones")
	runCmd("./dialtone.sh", "nexttone", "--sign", "yes")
	runCmd("./dialtone.sh", "nexttone", "--sign", "yes")

	first := runCmd("./dialtone.sh", "nexttone")
	if !strings.Contains(first, "DIALTONE [") {
		return fmt.Errorf("expected DIALTONE prompt")
	}
	if !strings.Contains(first, "SUBTONE:") {
		return fmt.Errorf("expected subtone prompt")
	}
	if err := assertState("subtone-review", "alpha"); err != nil {
		return err
	}

	next := runCmd("./dialtone.sh", "nexttone", "--sign", "yes")
	if !strings.Contains(next, "DIALTONE [subtone-run-test]") {
		return fmt.Errorf("expected subtone-run-test prompt")
	}
	if !strings.Contains(next, "TEST RESULT: PASS") {
		return fmt.Errorf("expected subtone test result output")
	}
	if strings.Contains(first, "SUBTONE: alpha") {
		if err := assertState("subtone-run-test", "alpha"); err != nil {
			return err
		}
		if err := assertBackupExists(); err != nil {
			return err
		}
		return nil
	}
	if strings.Contains(first, "SUBTONE: beta") {
		if err := assertState("subtone-run-test", "beta"); err != nil {
			return err
		}
		if err := assertBackupExists(); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("unexpected subtone prompt content")
}

func TestCompleteTone() error {
	logLine("step", "Complete tone")

	oldDB := testDBPath
	testDBPath = tempDBPath()
	if oldDB != "" {
		_ = os.Remove(oldDB)
	}

	// Advance into subtone-review
	output := runCmd("./dialtone.sh", "nexttone", "--sign", "yes")
	if !strings.Contains(output, "DIALTONE [") {
		return fmt.Errorf("expected DIALTONE prompt after sign")
	}
	output = runCmd("./dialtone.sh", "nexttone", "--sign", "yes")
	if !strings.Contains(output, "DIALTONE [") {
		return fmt.Errorf("expected DIALTONE prompt after sign")
	}
	output = runCmd("./dialtone.sh", "nexttone", "--sign", "yes")
	if !strings.Contains(output, "DIALTONE [") {
		return fmt.Errorf("expected DIALTONE prompt after sign")
	}

	// Finish subtones (alpha, beta) and reach completion flow
	output = runCmd("./dialtone.sh", "nexttone", "--sign", "yes") // alpha -> run-test
	if !strings.Contains(output, "DIALTONE [") {
		return fmt.Errorf("expected DIALTONE prompt after sign")
	}
	output = runCmd("./dialtone.sh", "nexttone", "--sign", "yes") // run-test -> review(beta)
	if !strings.Contains(output, "DIALTONE [") {
		return fmt.Errorf("expected DIALTONE prompt after sign")
	}
	output = runCmd("./dialtone.sh", "nexttone", "--sign", "yes") // beta -> run-test
	if !strings.Contains(output, "DIALTONE [") {
		return fmt.Errorf("expected DIALTONE prompt after sign")
	}
	output = runCmd("./dialtone.sh", "nexttone", "--sign", "yes") // run-test -> complete
	if !strings.Contains(output, "DIALTONE [") {
		return fmt.Errorf("expected DIALTONE prompt after sign")
	}
	if !strings.Contains(output, "subtone-review-complete") {
		return fmt.Errorf("expected subtone review completion prompt")
	}
	if err := assertState("subtone-review-complete", "alpha"); err != nil {
		return err
	}

	// Advance through completion microtones to final signature
	output = runCmd("./dialtone.sh", "nexttone", "--sign", "yes")
	if !strings.Contains(output, "DIALTONE [") {
		return fmt.Errorf("expected DIALTONE prompt after sign")
	}
	if !strings.Contains(output, "start-complete-phase") {
		return fmt.Errorf("expected start complete phase prompt")
	}
	if err := assertState("start-complete-phase", "alpha"); err != nil {
		return err
	}

	output = runCmd("./dialtone.sh", "nexttone", "--sign", "yes")
	if !strings.Contains(output, "DIALTONE [") {
		return fmt.Errorf("expected DIALTONE prompt after sign")
	}
	if !strings.Contains(output, "confirm-pr-merged") {
		return fmt.Errorf("expected confirm PR merged prompt")
	}
	if err := assertState("confirm-pr-merged", "alpha"); err != nil {
		return err
	}

	output = runCmd("./dialtone.sh", "nexttone", "--sign", "yes")
	if !strings.Contains(output, "DIALTONE [") {
		return fmt.Errorf("expected DIALTONE prompt after sign")
	}
	if !strings.Contains(output, "complete") {
		return fmt.Errorf("expected complete prompt")
	}
	if err := assertState("complete", "alpha"); err != nil {
		return err
	}
	output = runCmd("./dialtone.sh", "nexttone", "--sign", "yes")
	if !strings.Contains(output, "COMPLETE!") || !strings.Contains(output, "PR merged") {
		return fmt.Errorf("expected COMPLETE and PR merged confirmation")
	}
	if err := assertBackupExists(); err != nil {
		return err
	}
	return nil
}

func runCmd(name string, args ...string) string {
	logLine("cmd", fmt.Sprintf("%s %v", name, args))
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(),
		nexttoneDBEnv+"="+testDBPath,
		nexttoneToneEnv+"="+testToneName,
		nexttoneToneDirEnv+"="+testToneRoot,
	)
	output, _ := cmd.CombinedOutput()
	fmt.Print(string(output))
	return string(output)
}

func tempDBPath() string {
	base := fmt.Sprintf("nexttone-test-%d.duckdb", time.Now().UnixNano())
	return filepath.Join(os.TempDir(), base)
}

func cleanupDB() {
	if testDBPath != "" {
		_ = os.Remove(testDBPath)
	}
}

func logLine(level, message string) {
	fmt.Printf("[%s] %s\n", level, message)
}

func setupToneDir() {
	base := filepath.Join("src", "tones", "tmp")
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
	toneTestDir := filepath.Join(testToneRoot, testToneName, "test")
	if err := os.MkdirAll(toneTestDir, 0755); err != nil {
		logLine("error", fmt.Sprintf("failed to create tone test dir: %v", err))
		os.Exit(1)
	}
	testGo := filepath.Join(toneTestDir, "test.go")
	content := []byte(`package test

import (
	"os"
	"testing"
)

func TestSubtone(t *testing.T) {
	if os.Getenv("NEXTTONE_SUBTONE") == "" {
		t.Skip("no subtone provided")
	}
}
`)
	if err := os.WriteFile(testGo, content, 0644); err != nil {
		logLine("error", fmt.Sprintf("failed to write test.go: %v", err))
		os.Exit(1)
	}
}

func cleanupToneDir() {
	if testToneRoot != "" {
		_ = os.RemoveAll(testToneRoot)
	}
}

func assertState(expectedMicrotone, expectedSubtone string) error {
	db, err := sql.Open("duckdb", testDBPath)
	if err != nil {
		return err
	}
	defer db.Close()

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

func assertBackupExists() error {
	backupPath := filepath.Join(testToneRoot, testToneName, "test", "nexttone_backup.duckdb")
	if _, err := os.Stat(backupPath); err != nil {
		return fmt.Errorf("expected backup at %s", backupPath)
	}
	return nil
}

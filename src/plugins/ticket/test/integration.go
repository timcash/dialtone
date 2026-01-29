package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/marcboeker/go-duckdb"
)

const ticketV2Dir = "src/tickets"
const testDataDir = "src/plugins/ticket/test"
const ticketDBFile = "tickets.duckdb"

func main() {
	fmt.Println("=== Starting ticket Granular Integration Tests ===")

	allPassed := true
	runTest := func(name string, fn func() error) {
		fmt.Printf("\n[TEST] %s\n", name)
		if err := fn(); err != nil {
			fmt.Printf("FAIL: %s - %v\n", name, err)
			allPassed = false
		} else {
			fmt.Printf("PASS: %s\n", name)
		}
	}

	defer func() {
		fmt.Println("\n=== Integration Tests Completed ===")
		if !allPassed {
			fmt.Println("\n!!! SOME TESTS FAILED !!!")
			os.Exit(1)
		}
	}()

	runTest("ticket add", TestAddGranular)
	runTest("ticket start", TestStartGranular)
	runTest("ticket ask", TestAskGranular)
	runTest("ticket log", TestLogGranular)
	runTest("ticket next", TestNextGranular)
	runTest("ticket validate", TestValidateGranular)
	runTest("ticket done", TestDoneGranular)
	runTest("subtask basics", TestSubtaskBasicsGranular)
	runTest("subtask done/failed", TestSubtaskDoneFailedGranular)
	runTest("conversational flow (V2)", TestConversationalFlow)
	runTest("subtask verification loop", TestSubtaskVerificationLoop)
	runTest("summary workflow", TestSummaryWorkflow)
	runTest("summary search", TestSummarySearch)
	runTest("summary timeout", TestSummaryTimeout)

	finalCleanup()
	fmt.Println()
}

func finalCleanup() {
	fmt.Println("=== Final Integration Cleanup ===")
	// List all subdirectories in src/tickets and remove them if they look like test tickets
	dirs, err := os.ReadDir(ticketV2Dir)
	if err != nil {
		return
	}
	for _, d := range dirs {
		if d.IsDir() {
			fmt.Printf("Removing dangling test directory: %s\n", d.Name())
			os.RemoveAll(filepath.Join(ticketV2Dir, d.Name()))
		}
	}
	// We keep the DB but clear its rows if needed.
	// Or just remove the DB for a total fresh start if the user wants "empty".
	// The user said "the src/tickets folder should be empty", so let's remove everything.
	os.Remove(filepath.Join(ticketV2Dir, "tickets.duckdb"))
}

// runTest removed from global scope to use closure in main for error tracking

func getUniqueName(base string) string {
	return fmt.Sprintf("%s-%d", base, time.Now().Unix())
}

func TestAddGranular() error {
	name := getUniqueName("test-add")
	cleanupTicket(name)
	defer cleanupTicket(name)

	output := runCmd("./dialtone.sh", "ticket", "add", name)
	if !strings.Contains(output, "Created") {
		return fmt.Errorf("expected 'Created' message")
	}

	// Verify files exist
	if _, err := os.Stat(ticketDBPath()); err != nil {
		return fmt.Errorf("tickets.duckdb missing")
	}
	if _, err := os.Stat(filepath.Join(ticketV2Dir, name, "test", "test.go")); err != nil {
		return fmt.Errorf("test/test.go missing")
	}

	entries, err := getLogEntries(name)
	if err != nil {
		return fmt.Errorf("failed to read log entries: %v", err)
	}
	if !findLogEntry(entries, "command", "ticket add "+name, "") {
		return fmt.Errorf("missing command log entry")
	}

	return nil
}

func TestStartGranular() error {
	name := getUniqueName("test-start")
	cleanupTicket(name)
	defer cleanupTicket(name)

	output := runCmd("./dialtone.sh", "ticket", "start", name)

	checks := []string{
		"Ticket " + name + " started successfully",
	}
	for _, c := range checks {
		if !strings.Contains(output, c) {
			return fmt.Errorf("missing log check: %s", c)
		}
	}

	return nil
}

func TestAskGranular() error {
	name := getUniqueName("test-ask")
	cleanupTicket(name)
	defer cleanupTicket(name)
	runCmd("./dialtone.sh", "ticket", "add", name)

	output := runCmd("./dialtone.sh", "ticket", "ask", "How do we handle auth?")
	if !strings.Contains(output, "Captured question in") {
		return fmt.Errorf("missing capture confirmation")
	}
	entries, err := getLogEntries(name)
	if err != nil {
		return fmt.Errorf("failed to read log entries: %v", err)
	}
	if !findLogEntry(entries, "question", "How do we handle auth?", "") {
		return fmt.Errorf("missing question entry")
	}

	output = runCmd("./dialtone.sh", "ticket", "ask", "--subtask", "init", "Is init required?")
	if !strings.Contains(output, "Captured question in") {
		return fmt.Errorf("missing capture confirmation for subtask")
	}

	entries, err = getLogEntries(name)
	if err != nil {
		return fmt.Errorf("failed to read log entries: %v", err)
	}
	if !findLogEntry(entries, "question", "Is init required?", "init") {
		return fmt.Errorf("missing subtask question")
	}

	return nil
}

func TestLogGranular() error {
	name := getUniqueName("test-log")
	cleanupTicket(name)
	defer cleanupTicket(name)
	runCmd("./dialtone.sh", "ticket", "add", name)

	output := runCmd("./dialtone.sh", "ticket", "log", "Adding a note.")
	if !strings.Contains(output, "Captured log in") {
		return fmt.Errorf("missing log capture confirmation")
	}
	entries, err := getLogEntries(name)
	if err != nil {
		return fmt.Errorf("failed to read log entries: %v", err)
	}
	if !findLogEntry(entries, "log", "Adding a note.", "") {
		return fmt.Errorf("missing log entry")
	}

	return nil
}

func TestNextGranular() error {
	name := getUniqueName("test-next")
	cleanupTicket(name)
	defer cleanupTicket(name)
	runCmd("./dialtone.sh", "ticket", "add", name)

	// Sub-item 2: Dependency Check & Auto-Promotion
	err := saveTicket(name, "Granular next test", []seedSubtask{
		{
			Name:   "t1",
			Status: "todo",
		},
		{
			Name:         "t2",
			Status:       "todo",
			Dependencies: []string{"t1"},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to seed ticket: %v", err)
	}

	fmt.Println("--- Checking Auto-Promotion/Execution ---")
	output := runCmd("./dialtone.sh", "ticket", "next", name)
	if !strings.Contains(output, "Promoting subtask t1 to progress") {
		return fmt.Errorf("failed auto-promotion")
	}
	if !strings.Contains(output, "Subtask t1 failed") {
		return fmt.Errorf("expected failure since no test logic added yet")
	}
	if !strings.Contains(output, "Fail-Timestamp:") {
		return fmt.Errorf("missing fail-timestamp")
	}

	fmt.Println("--- Checking Pass State ---")
	testGoPath := filepath.Join(ticketV2Dir, name, "test", "test.go")
	os.WriteFile(testGoPath, []byte(fmt.Sprintf(`package test
import "dialtone/cli/src/dialtest"
func init() {
	dialtest.RegisterTicket("%s")
	dialtest.AddSubtaskTest("t1", func() error { return nil }, nil)
}
`, name)), 0644)

	output = runCmd("./dialtone.sh", "ticket", "next", name)
	if !strings.Contains(output, "Subtask t1 passed") {
		return fmt.Errorf("expected pass message")
	}

	return nil
}

func TestConversationalFlow() error {
	name := getUniqueName("test-v2-flow")
	cleanupTicket(name)
	defer cleanupTicket(name)

	fmt.Println("--- Checking Start Alert ---")
	output := runCmd("./dialtone.sh", "ticket", "start", name+"-api")
	if !strings.Contains(output, "[ALERT]") {
		return fmt.Errorf("missing contextual alert on start")
	}

	fmt.Println("--- Checking Next Block on Question ---")
	runCmd("./dialtone.sh", "ticket", "ask", "What is the meaning of life?")
	output = runCmd("./dialtone.sh", "ticket", "next")
	if !strings.Contains(output, "[BLOCK]") {
		return fmt.Errorf("expected block on unacknowledged question")
	}

	fmt.Println("--- Checking Next Unblock after Ack ---")
	runCmd("./dialtone.sh", "ticket", "ack", "I understand everything now.")
	output = runCmd("./dialtone.sh", "ticket", "next")
	if strings.Contains(output, "[BLOCK]") {
		return fmt.Errorf("expected unblock after ack")
	}

	return nil
}

func TestSubtaskVerificationLoop() error {
	name := getUniqueName("test-verify-loop")
	cleanupTicket(name)
	defer cleanupTicket(name)

	runCmd("./dialtone.sh", "ticket", "start", name)

	// Overwrite with failing test
	testGoPath := filepath.Join(ticketV2Dir, name, "test", "test.go")
	failingTest := fmt.Sprintf(`package test
import (
	"dialtone/cli/src/dialtest"
	"fmt"
)
func init() {
	dialtest.RegisterTicket("%s")
	dialtest.AddSubtaskTest("init", func() error { return fmt.Errorf("simulated failure") }, nil)
}
`, name)
	os.WriteFile(testGoPath, []byte(failingTest), 0644)

	fmt.Println("--- Checking Failure Block ---")
	output := runCmd("./dialtone.sh", "ticket", "subtask", "done", name, "init")
	if !strings.Contains(output, "Verification failed") {
		return fmt.Errorf("expected failure when test fails")
	}

	// Overwrite with passing test
	passingTest := fmt.Sprintf(`package test
import (
	"dialtone/cli/src/dialtest"
)
func init() {
	dialtest.RegisterTicket("%s")
	dialtest.AddSubtaskTest("init", func() error { return nil }, nil)
}
`, name)
	os.WriteFile(testGoPath, []byte(passingTest), 0644)

	fmt.Println("--- Checking Success Unblock ---")
	// Must create agent_summary.md for V2 done
	dir := filepath.Join("src", "tickets", name)
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "agent_summary.md"), []byte("I did some stuff"), 0644)

	output = runCmd("./dialtone.sh", "ticket", "subtask", "done", name, "init")
	if !strings.Contains(output, "marked as done") {
		return fmt.Errorf("expected success after fixing test")
	}

	fmt.Println("--- Checking ticket test command ---")
	output = runCmd("./dialtone.sh", "ticket", "test", name)
	if !strings.Contains(output, "All tests passed") {
		return fmt.Errorf("expected ticket test success")
	}

	return nil
}

func TestSummaryWorkflow() error {
	name := getUniqueName("test-summary")
	cleanupTicket(name)
	defer cleanupTicket(name)

	runCmd("./dialtone.sh", "ticket", "start", name)

	// 1. Check ingest without file fails and prints guidance
	fmt.Println("--- Checking Missing Summary File Guidance ---")
	output := runCmd("./dialtone.sh", "ticket", "summary", "update")
	if !strings.Contains(output, "Could not read") || !strings.Contains(output, "searched with grep") {
		return fmt.Errorf("expected error with guidance when summary file is missing")
	}

	// 2. Create file and ingest
	dir := filepath.Join("src", "tickets", name)
	summaryPath := filepath.Join(dir, "agent_summary.md")
	os.WriteFile(summaryPath, []byte("Working on the flux capacitor."), 0644)
	runCmd("./dialtone.sh", "ticket", "summary", "update")

	// Verify file is deleted
	if _, err := os.Stat(summaryPath); !os.IsNotExist(err) {
		return fmt.Errorf("agent_summary.md should have been deleted after update")
	}

	// 3. Verify SHA256 block (must recreate file)
	fmt.Println("--- Checking SHA256 Block ---")
	os.WriteFile(summaryPath, []byte("Working on the flux capacitor."), 0644)
	output = runCmd("./dialtone.sh", "ticket", "summary", "update")
	if !strings.Contains(output, "content has not changed") {
		return fmt.Errorf("expected block on duplicate summary content")
	}

	// 4. Update file and ingest again
	fmt.Println("--- Checking Multiple Summaries ---")
	os.WriteFile(summaryPath, []byte("Flux capacitor now at 1.21 gigawatts."), 0644)
	runCmd("./dialtone.sh", "ticket", "summary", "update")

	// 5. Check --idle
	fmt.Println("--- Checking Idle Summary ---")
	output = runCmd("./dialtone.sh", "ticket", "summary", "--idle")
	if !strings.Contains(output, "Summary captured") {
		return fmt.Errorf("idle summary failed")
	}

	// 6. Print all summaries (User requirement)
	fmt.Println("--- Final Summary List Printout ---")
	output = runCmd("./dialtone.sh", "ticket", "summary")
	if !strings.Contains(output, "Working on the flux capacitor") || !strings.Contains(output, "1.21 gigawatts") {
		return fmt.Errorf("summary list missing content")
	}

	return nil
}

func TestSummarySearch() error {
	name := getUniqueName("test-search")
	cleanupTicket(name)
	defer cleanupTicket(name)

	runCmd("./dialtone.sh", "ticket", "start", name)
	dir := filepath.Join("src", "tickets", name)
	os.WriteFile(filepath.Join(dir, "agent_summary.md"), []byte("Found a bug in the matrix."), 0644)
	runCmd("./dialtone.sh", "ticket", "summary", "update")

	fmt.Println("--- Checking Search ---")
	output := runCmd("./dialtone.sh", "ticket", "search", "matrix")
	if !strings.Contains(output, "Found a bug in the matrix") {
		return fmt.Errorf("search failed to find content")
	}

	return nil
}

func TestSummaryTimeout() error {
	name := getUniqueName("test-timeout")
	cleanupTicket(name)
	defer cleanupTicket(name)

	runCmd("./dialtone.sh", "ticket", "start", name)

	// Manually backdate the last summary time in DuckDB
	db, err := openTicketDB()
	if err != nil {
		return err
	}
	defer db.Close()

	backdatedTime := time.Now().Add(-15 * time.Minute).Format(time.RFC3339)
	_, err = db.Exec(`UPDATE tickets SET last_summary_time = ? WHERE id = ?`, backdatedTime, name)
	db.Close() // RELEASE LOCK BEFORE CLI CALL
	if err != nil {
		return fmt.Errorf("failed to backdate: %v", err)
	}

	fmt.Println("--- Checking Timeout Block ---")
	output := runCmd("./dialtone.sh", "ticket", "next", name)
	if !strings.Contains(output, "10-minute activity window exceeded") {
		return fmt.Errorf("expected block after 10 minutes")
	}

	fmt.Println("--- Checking Timeout Unblock with --idle ---")
	output = runCmd("./dialtone.sh", "ticket", "next", name, "--idle")
	// Should fail on test execution now, but not on timeout block
	if strings.Contains(output, "10-minute activity window exceeded") {
		return fmt.Errorf("expected --idle to bypass block")
	}

	return nil
}

func TestValidateGranular() error {
	fmt.Println("--- Checking Timestamp Regression ---")
	name := getUniqueName("test-validate")
	cleanupTicket(name)
	defer cleanupTicket(name)
	err := saveTicket(name, "", []seedSubtask{
		{
			Name:          "r",
			Status:        "done",
			PassTimestamp: "2026-01-27T10:00:00Z",
			FailTimestamp: "2026-01-27T11:00:00Z",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to seed ticket: %v", err)
	}

	output := runCmd("./dialtone.sh", "ticket", "validate", name)
	if !strings.Contains(output, "[REGRESSION]") {
		return fmt.Errorf("failed regression detection")
	}

	return nil
}

func TestDoneGranular() error {
	name := getUniqueName("test-done")
	cleanupTicket(name)
	defer cleanupTicket(name)

	runCmd("./dialtone.sh", "ticket", "start", name)

	// Create agent_summary.md for subtask done
	dir := filepath.Join("src", "tickets", name)
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "agent_summary.md"), []byte("Subtask progress"), 0644)

	runCmd("./dialtone.sh", "ticket", "subtask", "done", name, "init")

	fmt.Println("--- Checking Done Missing Summary ---")
	os.Remove(filepath.Join(dir, "agent_summary.md"))
	output := runCmd("./dialtone.sh", "ticket", "done")
	if !strings.Contains(output, "Missing mandatory agent_summary.md") {
		return fmt.Errorf("expected error for missing agent_summary.md")
	}

	fmt.Println("--- Checking Done Completion with Summary ---")
	os.WriteFile(filepath.Join(dir, "agent_summary.md"), []byte("Full ticket summary."), 0644)
	output = runCmd("./dialtone.sh", "ticket", "done")
	checks := []string{
		"Ticket " + name + " completed",
	}
	for _, c := range checks {
		if !strings.Contains(output, c) {
			return fmt.Errorf("missing log check: %s", c)
		}
	}

	return nil
}

func TestSubtaskBasicsGranular() error {
	name := getUniqueName("test-sub-basics")
	cleanupTicket(name)
	defer cleanupTicket(name)
	runCmd("./dialtone.sh", "ticket", "add", name)

	// subtask list
	output := runCmd("./dialtone.sh", "ticket", "subtask", "list", name)
	if !strings.Contains(output, "Subtasks for "+name) {
		return fmt.Errorf("failed subtask list")
	}

	return nil
}

func TestSubtaskDoneFailedGranular() error {
	name := getUniqueName("test-sub-state")
	cleanupTicket(name)
	defer cleanupTicket(name)

	runCmd("./dialtone.sh", "ticket", "start", name)

	// Create agent_summary.md for subtask done
	dir := filepath.Join("src", "tickets", name)
	os.WriteFile(filepath.Join(dir, "agent_summary.md"), []byte("Doing initialization"), 0644)

	runCmd("./dialtone.sh", "ticket", "subtask", "done", name, "init")
	runCmd("./dialtone.sh", "ticket", "subtask", "failed", name, "init")

	return nil
}

func runCmd(name string, args ...string) string {
	fmt.Printf("> %s %v\n", name, args)
	cmd := exec.Command(name, args...)
	output, _ := cmd.CombinedOutput()
	fmt.Println(string(output))
	return string(output)
}

type logEntry struct {
	EntryType string
	Message   string
	Subtask   string
}

type seedSubtask struct {
	Name           string
	Description    string
	Status         string
	Dependencies   []string
	TestConditions []string
	AgentNotes     string
	PassTimestamp  string
	FailTimestamp  string
}

type testCondition struct {
	Condition string `json:"condition"`
}

func ticketDBPath() string {
	return filepath.Join(ticketV2Dir, ticketDBFile)
}

func openTicketDB() (*sql.DB, error) {
	if err := os.MkdirAll(ticketV2Dir, 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("duckdb", ticketDBPath())
	if err != nil {
		return nil, err
	}
	if err := ensureTicketSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func ensureTicketSchema(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS tickets (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			tags TEXT,
			description TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS subtasks (
			ticket_id TEXT NOT NULL,
			position INTEGER NOT NULL,
			name TEXT NOT NULL,
			tags TEXT,
			dependencies TEXT,
			description TEXT,
			test_conditions TEXT,
			agent_notes TEXT,
			pass_timestamp TEXT,
			fail_timestamp TEXT,
			status TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS ticket_logs (
			ticket_id TEXT NOT NULL,
			timestamp TEXT NOT NULL,
			entry_type TEXT NOT NULL,
			message TEXT NOT NULL,
			subtask TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS ticket_meta (
			key TEXT PRIMARY KEY,
			value TEXT
		);`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func cleanupTicket(name string) {
	fmt.Printf("--- Cleanup: %s ---\n", name)
	if err := deleteTicketData(name); err != nil {
		fmt.Printf("WARNING: Failed to cleanup ticket data %s: %v\n", name, err)
	} else {
		fmt.Printf("Deleted DuckDB rows for %s\n", name)
	}
	if err := os.RemoveAll(filepath.Join(ticketV2Dir, name)); err != nil {
		fmt.Printf("WARNING: Failed to cleanup %s: %v\n", name, err)
	} else {
		fmt.Printf("Removed directory %s\n", filepath.Join(ticketV2Dir, name))
	}
}

func deleteTicketData(name string) error {
	db, err := openTicketDB()
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM ticket_logs WHERE ticket_id = ?`, name); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM subtasks WHERE ticket_id = ?`, name); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM tickets WHERE id = ?`, name); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM ticket_meta WHERE key = 'current_ticket' AND value = ?`, name); err != nil {
		return err
	}
	return tx.Commit()
}

func saveTicket(name, description string, subtasks []seedSubtask) error {
	db, err := openTicketDB()
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM subtasks WHERE ticket_id = ?`, name); err != nil {
		return err
	}

	if _, err := tx.Exec(`INSERT INTO tickets (id, name, tags, description)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name = excluded.name, tags = excluded.tags, description = excluded.description`,
		name, name, "", description); err != nil {
		return err
	}

	for i, st := range subtasks {
		depsPayload, err := json.Marshal(st.Dependencies)
		if err != nil {
			return err
		}
		tests := make([]testCondition, 0, len(st.TestConditions))
		for _, cond := range st.TestConditions {
			tests = append(tests, testCondition{Condition: cond})
		}
		testsPayload, err := json.Marshal(tests)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO subtasks (
			ticket_id, position, name, tags, dependencies, description, test_conditions, agent_notes, pass_timestamp, fail_timestamp, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			name,
			i,
			st.Name,
			"",
			string(depsPayload),
			st.Description,
			string(testsPayload),
			st.AgentNotes,
			st.PassTimestamp,
			st.FailTimestamp,
			st.Status,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func getLogEntries(ticketID string) ([]logEntry, error) {
	db, err := openTicketDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT entry_type, message, subtask FROM ticket_logs WHERE ticket_id = ?`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []logEntry
	for rows.Next() {
		var entry logEntry
		var subtask sql.NullString
		if err := rows.Scan(&entry.EntryType, &entry.Message, &subtask); err != nil {
			return nil, err
		}
		if subtask.Valid {
			entry.Subtask = subtask.String
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func findLogEntry(entries []logEntry, entryType, messageContains, subtask string) bool {
	for _, entry := range entries {
		if entry.EntryType != entryType {
			continue
		}
		if subtask != "" && entry.Subtask != subtask {
			continue
		}
		if strings.Contains(entry.Message, messageContains) {
			return true
		}
	}
	return false
}

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

const ticketV2Dir = "src/tickets"
const currentTicketFile = ".current_ticket"

func main() {
	fmt.Println("=== Starting Ticket Workflow Integration Test ===")

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

	runTest("End-to-End Ticket V2 Workflow", TestFullWorkflow)
	runTest("Key Management Workflow", TestKeyWorkflow)

	finalCleanup()
	fmt.Println()
}

func TestFullWorkflow() error {
	name := getUniqueName("workflow-demo")
	cleanupTicket(name)
	defer cleanupTicket(name)

	// --- STEP 1: Starting Ticket ---
	fmt.Println("\n--- STEP 1: Starting Ticket ---")
	output := runCmd("./dialtone.sh", "ticket", "start", name)
	if !strings.Contains(output, "Ticket "+name+" started successfully") {
		return fmt.Errorf("failed to start ticket")
	}

	// --- STEP 2: Conversational Block ---
	fmt.Println("\n--- STEP 2: Conversational Block ---")
	runCmd("./dialtone.sh", "ticket", "ask", "Should we use DuckDB directly?")
	output = runCmd("./dialtone.sh", "ticket", "next")
	if !strings.Contains(output, "[BLOCK]") {
		return fmt.Errorf("expected block on unacknowledged question")
	}

	// --- STEP 3: Acknowledge & Unblock ---
	fmt.Println("\n--- STEP 3: Acknowledge & Unblock ---")
	runCmd("./dialtone.sh", "ticket", "ack", "Yes, DuckDB is the standard.")

	// Overwrite scaffold with a failing test to demonstrate TDD cycle
	dir := filepath.Join("src", "tickets", name)
	testGoPath := filepath.Join(dir, "test", "test.go")
	os.WriteFile(testGoPath, []byte(fmt.Sprintf(`package test
import (
	"dialtone/cli/src/dialtest"
	"fmt"
)
func init() {
	dialtest.RegisterTicket("%s")
	dialtest.AddSubtaskTest("init", func() error { return fmt.Errorf("not yet implemented") }, nil)
}
`, name)), 0644)

	output = runCmd("./dialtone.sh", "ticket", "next")
	// In DIALTONE mode, `next` prints guidance and does not auto-run tests.
	if !strings.Contains(output, "DIALTONE:") {
		return fmt.Errorf("expected DIALTONE guidance after next")
	}

	// --- STEP 4: TDD Failure ---
	fmt.Println("\n--- STEP 4: TDD Failure ---")
	if !strings.Contains(output, "Run the subtask test") {
		return fmt.Errorf("expected guidance to run subtask tests")
	}

	// --- STEP 5: Fixing Test & Passing ---
	fmt.Println("\n--- STEP 5: Fixing Test & Passing ---")
	dir = filepath.Join("src", "tickets", name)
	testGoPath = filepath.Join(dir, "test", "test.go")
	os.WriteFile(testGoPath, []byte(fmt.Sprintf(`package test
import "dialtone/cli/src/dialtest"
func init() {
	dialtest.RegisterTicket("%s")
	dialtest.AddSubtaskTest("init", func() error { return nil }, nil)
}
`, name)), 0644)

	// Agent runs the test explicitly (DIALTONE does not auto-run).
	output = runCmd("./dialtone.sh", "ticket", "test", name, "--subtask", "init")
	if !strings.Contains(output, "Tests passed") && !strings.Contains(output, "Test passed") {
		return fmt.Errorf("expected tests to pass after fixing test")
	}

	// Mark subtask done (manual step)
	output = runCmd("./dialtone.sh", "ticket", "subtask", "done", name, "init")
	if !strings.Contains(output, "marked as done") {
		return fmt.Errorf("expected subtask to be marked done")
	}

	// --- STEP 6: Summary Ingestion ---
	fmt.Println("\n--- STEP 6: Summary Ingestion ---")
	summaryPath := filepath.Join(dir, "agent_summary.md")
	os.WriteFile(summaryPath, []byte("Started implementation. Verified DuckDB schema."), 0644)
	runCmd("./dialtone.sh", "ticket", "summary", "update")

	if _, err := os.Stat(summaryPath); !os.IsNotExist(err) {
		return fmt.Errorf("agent_summary.md should have been deleted")
	}

	// --- STEP 7: SHA256 Block ---
	fmt.Println("\n--- STEP 7: SHA256 Block ---")
	os.WriteFile(summaryPath, []byte("Started implementation. Verified DuckDB schema."), 0644)
	output = runCmd("./dialtone.sh", "ticket", "summary", "update")
	if !strings.Contains(output, "content has not changed") {
		return fmt.Errorf("expected SHA256 block")
	}

	// --- STEP 8: 10-Minute Timeout ---
	fmt.Println("\n--- STEP 8: 10-Minute Timeout ---")
	db, err := openTicketDB(name)
	if err != nil {
		return err
	}
	backdated := time.Now().Add(-15 * time.Minute).Format(time.RFC3339)
	_, err = db.Exec(`UPDATE tickets SET last_summary_time = ? WHERE id = ?`, backdated, name)
	db.Close()
	if err != nil {
		return err
	}

	output = runCmd("./dialtone.sh", "ticket", "next")
	if !strings.Contains(output, "10-minute activity window exceeded") {
		return fmt.Errorf("expected timeout block")
	}

	// --- STEP 9: Summary Guidance ---
	fmt.Println("\n--- STEP 9: Summary Guidance ---")
	if !strings.Contains(output, "[EXAMPLE]") || !strings.Contains(output, "searched with grep") {
		return fmt.Errorf("expected guidance example in block message")
	}

	// --- STEP 10: Search ---
	fmt.Println("\n--- STEP 10: Search ---")
	output = runCmd("./dialtone.sh", "ticket", "search", "DuckDB")
	if !strings.Contains(output, "Verified DuckDB schema") {
		return fmt.Errorf("search failed to find content")
	}

	// --- STEP 11: Final Summary List ---
	fmt.Println("\n--- STEP 11: Final Summary List ---")
	output = runCmd("./dialtone.sh", "ticket", "summary")
	if !strings.Contains(output, "Verified DuckDB schema") {
		return fmt.Errorf("summary list failed")
	}

	// --- STEP 12: Completion ---
	fmt.Println("\n--- STEP 12: Completion ---")
	os.WriteFile(summaryPath, []byte("Feature finished successfully."), 0644)
	output = runCmd("./dialtone.sh", "ticket", "done")
	if !strings.Contains(output, "completed") {
		return fmt.Errorf("failed to complete ticket")
	}

	return nil
}

func TestKeyWorkflow() error {
	// --- STEP 1: Add Key ---
	fmt.Println("\n--- STEP 1: Add Key ---")
	output := runCmd("./dialtone.sh", "ticket", "key", "add", "test-key", "secret-value", "password123")
	if !strings.Contains(output, "stored securely") {
		return fmt.Errorf("failed to add key")
	}

	// --- STEP 2: List Keys ---
	fmt.Println("\n--- STEP 2: List Keys ---")
	output = runCmd("./dialtone.sh", "ticket", "key", "list")
	if !strings.Contains(output, "test-key") {
		return fmt.Errorf("expected test-key in list")
	}

	// --- STEP 3: Lease Key (Correct Password) ---
	fmt.Println("\n--- STEP 3: Lease Key (Correct Password) ---")
	output = runCmd("./dialtone.sh", "ticket", "key", "test-key", "password123")
	if output != "secret-value" {
		return fmt.Errorf("expected 'secret-value', got '%s'", output)
	}

	// --- STEP 4: Lease Key (Wrong Password) ---
	fmt.Println("\n--- STEP 4: Lease Key (Wrong Password) ---")
	output = runCmd("./dialtone.sh", "ticket", "key", "test-key", "wrong")
	if !strings.Contains(output, "Invalid password") {
		return fmt.Errorf("expected error for wrong password")
	}

	// --- STEP 5: Remove Key ---
	fmt.Println("\n--- STEP 5: Remove Key ---")
	runCmd("./dialtone.sh", "ticket", "key", "rm", "test-key")
	output = runCmd("./dialtone.sh", "ticket", "key", "list")
	if strings.Contains(output, "test-key") {
		return fmt.Errorf("expected test-key to be removed")
	}

	return nil
}

func getUniqueName(base string) string {
	return fmt.Sprintf("%s-%d", base, time.Now().Unix())
}

func finalCleanup() {
	fmt.Println("=== Final Integration Cleanup ===")
	dirs, err := os.ReadDir(ticketV2Dir)
	if err != nil {
		return
	}
	for _, d := range dirs {
		if d.IsDir() && strings.HasPrefix(d.Name(), "workflow-demo") {
			fmt.Printf("Removing dangling test directory: %s\n", d.Name())
			os.RemoveAll(filepath.Join(ticketV2Dir, d.Name()))
		}
	}
	// Clear current ticket pointer
	_ = os.Remove(filepath.Join(ticketV2Dir, currentTicketFile))
}

func cleanupTicket(name string) {
	fmt.Printf("--- Cleanup: %s ---\n", name)
	db, err := openTicketDB(name)
	if err != nil {
		return
	}
	defer db.Close()
	db.Exec(`DELETE FROM ticket_logs WHERE ticket_id = ?`, name)
	db.Exec(`DELETE FROM subtasks WHERE ticket_id = ?`, name)
	db.Exec(`DELETE FROM tickets WHERE id = ?`, name)
	db.Exec(`DELETE FROM ticket_meta WHERE key = 'current_ticket' AND value = ?`, name)
	os.RemoveAll(filepath.Join(ticketV2Dir, name))
}

func runCmd(name string, args ...string) string {
	fmt.Printf("> %s %v\n", name, args)
	cmd := exec.Command(name, args...)
	output, _ := cmd.CombinedOutput()
	fmt.Println(string(output))
	return string(output)
}

func ticketDBPath(ticketID string) string {
	if p := os.Getenv("TICKET_DB_PATH"); p != "" {
		return p
	}
	return filepath.Join(ticketV2Dir, ticketID, ticketID+".duckdb")
}

func openTicketDB(ticketID string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Join(ticketV2Dir, ticketID), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("duckdb", ticketDBPath(ticketID))
	if err != nil {
		return nil, err
	}
	return db, nil
}

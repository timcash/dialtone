package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const testTicket = "modular-integration-test"
const ticketV2Dir = "src/tickets_v2"

func main() {
	fmt.Println("=== Starting ticket_v2 Modular Integration Tests ===")

	runTest("Scaffolding (add)", TestAdd)
	runTest("Validation Failures", TestValidateFailure)
	runTest("Lifecycle (start -> next -> done)", TestLifecycle)

	fmt.Println("\n=== Integration Tests Completed ===")
}

func runTest(name string, fn func() error) {
	fmt.Printf("\n[TEST] %s\n", name)
	if err := fn(); err != nil {
		fmt.Printf("FAIL: %s - %v\n", name, err)
		os.Exit(1)
	}
	fmt.Printf("PASS: %s\n", name)
}

func TestAdd() error {
	os.RemoveAll(filepath.Join(ticketV2Dir, testTicket))
	
	output := runCmd("./dialtone.sh", "ticket_v2", "add", testTicket)
	if !strings.Contains(output, "Created") {
		return fmt.Errorf("expected 'Created' message in output")
	}

	if _, err := os.Stat(filepath.Join(ticketV2Dir, testTicket, "ticket.md")); err != nil {
		return fmt.Errorf("ticket.md not created")
	}
	return nil
}

func TestValidateFailure() error {
	// 1. Missing Header
	fmt.Println("--- Checking missing Name header ---")
	output := runCmd("./dialtone.sh", "ticket_v2", "validate", "fake-ticket-validate-error")
	if !strings.Contains(output, "missing '# Name:' header") {
		return fmt.Errorf("expected 'missing Name header' error")
	}

	// 2. Invalid Status
	fmt.Println("--- Checking invalid status ---")
	invalidStatusTicket := "invalid-status-ticket"
	os.MkdirAll(filepath.Join(ticketV2Dir, invalidStatusTicket), 0755)
	os.WriteFile(filepath.Join(ticketV2Dir, invalidStatusTicket, "ticket.md"), []byte("# Name: "+invalidStatusTicket+"\n## SUBTASK: Task\n- name: t1\n- status: unknown\n"), 0644)
	output = runCmd("./dialtone.sh", "ticket_v2", "validate", invalidStatusTicket)
	if !strings.Contains(output, "invalid status") {
		return fmt.Errorf("expected 'invalid status' error")
	}
	os.RemoveAll(filepath.Join(ticketV2Dir, invalidStatusTicket))

	// 3. Timestamp Regression
	fmt.Println("--- Checking timestamp regression ---")
	regTicket := "regression-ticket"
	os.MkdirAll(filepath.Join(ticketV2Dir, regTicket), 0755)
	os.WriteFile(filepath.Join(ticketV2Dir, regTicket, "ticket.md"), []byte("# Name: "+regTicket+"\n## SUBTASK: Task\n- name: t1\n- pass-timestamp: 2026-01-27T10:00:00Z\n- fail-timestamp: 2026-01-27T11:00:00Z\n- status: done\n"), 0644)
	output = runCmd("./dialtone.sh", "ticket_v2", "validate", regTicket)
	if !strings.Contains(output, "[REGRESSION]") {
		return fmt.Errorf("expected '[REGRESSION]' error")
	}
	os.RemoveAll(filepath.Join(ticketV2Dir, regTicket))

	return nil
}

func TestLifecycle() error {
	// A. Start
	fmt.Println("--- Phase: Start ---")
	runCmd("./dialtone.sh", "ticket_v2", "start", testTicket)
	
	// B. Define subtasks
	ticketPath := filepath.Join(ticketV2Dir, testTicket, "ticket.md")
	content := fmt.Sprintf(`# Name: %s
# Goal
Lifecycle test.

## SUBTASK: First
- name: first-task
- status: todo

## SUBTASK: Second
- name: second-task
- dependencies: first-task
- status: todo
`, testTicket)
	os.WriteFile(ticketPath, []byte(content), 0644)

	// C. Next - Expect Failing Test
	fmt.Println("--- Phase: Next (Failure) ---")
	testGoPath := filepath.Join(ticketV2Dir, testTicket, "test", "test.go")
	os.WriteFile(testGoPath, []byte(fmt.Sprintf(`package test
import "dialtone/cli/src/dialtest"
import "fmt"
func init() {
	dialtest.RegisterTicket("%s")
	dialtest.AddSubtaskTest("first-task", func() error { return fmt.Errorf("failure-msg") }, nil)
}
`, testTicket)), 0644)
	
	output := runCmd("./dialtone.sh", "ticket_v2", "next")
	if !strings.Contains(output, "failure-msg") {
		return fmt.Errorf("expected test failure message")
	}

	// D. Next - Expect Pass after Fix
	fmt.Println("--- Phase: Next (Pass) ---")
	os.WriteFile(testGoPath, []byte(fmt.Sprintf(`package test
import "dialtone/cli/src/dialtest"
func init() {
	dialtest.RegisterTicket("%s")
	dialtest.AddSubtaskTest("first-task", func() error { return nil }, nil)
	dialtest.AddSubtaskTest("second-task", func() error { return nil }, nil)
}
`, testTicket)), 0644)
	
	output = runCmd("./dialtone.sh", "ticket_v2", "next")
	if !strings.Contains(output, "Subtask first-task passed") {
		return fmt.Errorf("expected first-task pass")
	}
	if !strings.Contains(output, "Subtask second-task passed") {
		return fmt.Errorf("expected second-task pass (auto-promotion)")
	}

	// E. Done
	fmt.Println("--- Phase: Done ---")
	output = runCmd("./dialtone.sh", "ticket_v2", "done")
	if !strings.Contains(output, "completed") {
		return fmt.Errorf("expected ticket completion")
	}

	return nil
}

func runCmd(name string, args ...string) string {
	fmt.Printf("> %s %v\n", name, args)
	cmd := exec.Command(name, args...)
	output, _ := cmd.CombinedOutput()
	fmt.Println(string(output))
	return string(output)
}

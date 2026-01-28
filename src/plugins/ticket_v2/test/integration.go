package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const tempTicketName = "tmp-test-ticket"
const ticketV2Dir = "src/tickets_v2"

func main() {
	fmt.Println("Starting ticket_v2 Integration Test...")

	// 1. Setup
	if err := setup(); err != nil {
		fmt.Printf("Setup failed: %v\n", err)
		os.Exit(1)
	}

	// 2. Run Tests
	if err := runTests(); err != nil {
		fmt.Printf("Tests failed: %v\n", err)
		teardown() // try to cleanup anyway
		os.Exit(1)
	}

	// 3. Teardown
	if err := teardown(); err != nil {
		fmt.Printf("Teardown failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nIntegration Test PASSED!")
}

func setup() error {
	fmt.Println("Setting up temporary ticket...")
	// Remove if exists
	teardown()

	// Use the CLI to add a ticket
	cmd := exec.Command("./dialtone.sh", "ticket_v2", "add", tempTicketName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add ticket: %v, output: %s", err, string(output))
	}
	fmt.Println("Ticket scaffolded.")
	return nil
}

func teardown() error {
	path := filepath.Join(ticketV2Dir, tempTicketName)
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("Cleaning up %s...\n", path)
		return os.RemoveAll(path)
	}
	return nil
}

func runTests() error {
	// A. Verify file existence
	ticketPath := filepath.Join(ticketV2Dir, tempTicketName, "ticket.md")
	if _, err := os.Stat(ticketPath); os.IsNotExist(err) {
		return fmt.Errorf("ticket.md not found at %s", ticketPath)
	}

	// B. Test 'ticket_v2 start' (simulated)
	fmt.Println("Running ticket_v2 start...")
	// We use a different name to ensure we are not on 'fake-ticket'
	cmd := exec.Command("./dialtone.sh", "ticket_v2", "start", tempTicketName)
	cmd.CombinedOutput()
	
	// Ensure we are on the correct branch
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branch, _ := cmd.Output()
	fmt.Printf("Current branch: %s\n", strings.TrimSpace(string(branch)))

	// C. Modify subtasks for testing
	fmt.Println("Adding test subtasks...")
	content := fmt.Sprintf(`# Name: %s
# Goal
Test the ticket_v2 system.

## SUBTASK: Init
- name: init-task
- description: First task
- test-condition-1: always passes
- status: todo

## SUBTASK: Dependent
- name: dependent-task
- dependencies: init-task
- description: Second task
- test-condition-1: always passes
- status: todo
`, tempTicketName)

	if err := os.WriteFile(ticketPath, []byte(content), 0644); err != nil {
		return err
	}

	// D. Register tests in test.go with named functions
	testGoPath := filepath.Join(ticketV2Dir, tempTicketName, "test", "test.go")
	testGoContent := fmt.Sprintf(`package test

import (
	"dialtone/cli/src/dialtest"
	"fmt"
)

func init() {
	dialtest.RegisterTicket("%%s")
	dialtest.AddSubtaskTest("init-task", RunInitTask, nil)
	dialtest.AddSubtaskTest("dependent-task", RunDependentTask, nil)
}

func RunInitTask() error {
	fmt.Println("init-task running")
	return nil
}

func RunDependentTask() error {
	fmt.Println("dependent-task running")
	return nil
}
`, tempTicketName)
	if err := os.WriteFile(testGoPath, []byte(testGoContent), 0644); err != nil {
		return err
	}

	// E. Run 'ticket_v2 validate'
	fmt.Println("Running ticket_v2 validate...")
	cmd = exec.Command("./dialtone.sh", "ticket_v2", "validate", tempTicketName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("validate failed: %v, output: %s", err, string(output))
	}

	// F. Run 'ticket_v2 next'
	fmt.Println("Running ticket_v2 next (1st task)...")
	cmd = exec.Command("./dialtone.sh", "ticket_v2", "next")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ticket_v2 next failed: %v, output: %s", err, string(output))
	}
	fmt.Println(string(output))

	// G. Verify progress in ticket.md
	data, _ := os.ReadFile(ticketPath)
	if !strings.Contains(string(data), "status: done") {
		return fmt.Errorf("init-task should be done. Current content:\n%s", string(data))
	}
	fmt.Println("init-task verified as done.")

	// H. Simulate a failure in dependent-task
	fmt.Println("Simulating a failure in dependent-task...")
	testGoContent = fmt.Sprintf(`package test

import (
	"dialtone/cli/src/dialtest"
	"fmt"
)

func init() {
	dialtest.RegisterTicket("%%s")
	dialtest.AddSubtaskTest("init-task", RunInitTask, nil)
	dialtest.AddSubtaskTest("dependent-task", RunDependentTask, nil)
}

func RunInitTask() error {
	return nil
}

func RunDependentTask() error {
	return fmt.Errorf("simulated failure")
}
`, tempTicketName)
	os.WriteFile(testGoPath, []byte(testGoContent), 0644)

	fmt.Println("Running ticket_v2 next (should fail)...")
	cmd = exec.Command("./dialtone.sh", "ticket_v2", "next")
	output, _ = cmd.CombinedOutput()
	if !strings.Contains(string(output), "Subtask dependent-task failed") {
		return fmt.Errorf("expected failure message not found in output: %s", string(output))
	}
	fmt.Println("Failure detected correctly.")

	// I. Fix the test and run 'next' again
	fmt.Println("Fixing the test and running next (should pass)...")
	testGoContent = strings.Replace(testGoContent, `return fmt.Errorf("simulated failure")`, `return nil`, 1)
	os.WriteFile(testGoPath, []byte(testGoContent), 0644)

	cmd = exec.Command("./dialtone.sh", "ticket_v2", "next")
	if output, err = cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("final next failed: %v, output: %s", err, string(output))
	}
	fmt.Println("dependent-task passed.")

	// J. Run 'ticket_v2 done'
	fmt.Println("Running ticket_v2 done...")
	cmd = exec.Command("./dialtone.sh", "ticket_v2", "done")
	output, _ = cmd.CombinedOutput()
	fmt.Println(string(output))

	return nil
}

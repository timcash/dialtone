package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const tempTicketName = "walkthrough-live-demo"
const ticketV2Dir = "src/tickets_v2"

func main() {
	fmt.Println("Starting ticket_v2 Live Walkthrough Verification...")

	// 1. Setup
	if err := setup(); err != nil {
		fmt.Printf("Setup failed: %v\n", err)
		cleanupGit()
		os.Exit(1)
	}

	// 2. Run Comprehensive Tests
	if err := runLifecycle(); err != nil {
		fmt.Printf("Lifecycle failed: %v\n", err)
		cleanupGit()
		os.Exit(1)
	}

	// 3. Final Teardown
	if err := teardown(); err != nil {
		fmt.Printf("Teardown failed: %v\n", err)
	}

	fmt.Println("\nWalkthrough Verification PASSED!")
}

func setup() error {
	fmt.Println("\n[PHASE 1: Scaffolding]")
	// Remove if exists
	teardown()
	cleanupGit()

	// Use 'add' to scaffold
	cmd := exec.Command("./dialtone.sh", "ticket_v2", "add", tempTicketName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add ticket: %v, output: %s", err, string(output))
	}
	fmt.Println("Ticket scaffolded successfully.")
	return nil
}

func cleanupGit() {
	exec.Command("git", "checkout", "main").Run()
	exec.Command("git", "branch", "-D", tempTicketName).Run()
}

func teardown() error {
	path := filepath.Join(ticketV2Dir, tempTicketName)
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("Cleaning up files at %s...\n", path)
		return os.RemoveAll(path)
	}
	return nil
}

func runLifecycle() error {
	ticketPath := filepath.Join(ticketV2Dir, tempTicketName, "ticket.md")
	testGoPath := filepath.Join(ticketV2Dir, tempTicketName, "test", "test.go")
	var output []byte
	var err error

	fmt.Println("\n[PHASE 2: Start and Branching]")
	// 'start' will checkout the branch and create a placeholder PR
	exec.Command("./dialtone.sh", "ticket_v2", "start", tempTicketName).Run()
	
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branch, _ := cmd.Output()
	fmt.Printf("Active Branch: %s\n", strings.TrimSpace(string(branch)))

	fmt.Println("\n[PHASE 3: Defining Requirements]")
	content := fmt.Sprintf(`# Name: %s
# Goal
Show the full ticket_v2 lifecycle.

## SUBTASK: Logic Implementation
- name: logic-impl
- description: Implement core business logic.
- test-condition-1: logic passes verification
- status: todo

## SUBTASK: Integration
- name: integration-task
- dependencies: logic-impl
- description: Integrate with existing modules.
- test-condition-1: integration is successful
- status: todo
`, tempTicketName)

	if err := os.WriteFile(ticketPath, []byte(content), 0644); err != nil {
		return err
	}
	fmt.Println("Updated ticket.md with walkthrough requirements.")

	fmt.Println("\n[PHASE 4: Validating Format]")
	cmd = exec.Command("./dialtone.sh", "ticket_v2", "validate", tempTicketName)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("validation failed: %v, output: %s", err, string(output))
	}
	fmt.Println("Ticket validation passed.")

	fmt.Println("\n[PHASE 5: TDD Loop - Failing Subtask]")
	testGoContent := fmt.Sprintf(`package test

import (
	"dialtone/cli/src/dialtest"
	"fmt"
)

func init() {
	dialtest.RegisterTicket("%%s")
	dialtest.AddSubtaskTest("logic-impl", RunLogicTest, nil)
	dialtest.AddSubtaskTest("integration-task", RunIntegrationTest, nil)
}

func RunLogicTest() error {
	fmt.Println("Running logic test...")
	return fmt.Errorf("logic error: expected 42, got 0")
}

func RunIntegrationTest() error {
	fmt.Println("Running integration test...")
	return nil
}
`, tempTicketName)
	os.WriteFile(testGoPath, []byte(testGoContent), 0644)

	fmt.Println("Running 'ticket_v2 next' (expecting failure)...")
	cmd = exec.Command("./dialtone.sh", "ticket_v2", "next")
	output, err = cmd.CombinedOutput()
	if !strings.Contains(string(output), "Subtask logic-impl failed") {
		return fmt.Errorf("expected failure message not found. Output:\n%s", string(output))
	}
	fmt.Println("Alerted that test is not passing. State promoted to 'progress'.")

	fmt.Println("\n[PHASE 6: Fixing and Promoting]")
	fixedTest := fmt.Sprintf(`package test

import (
	"dialtone/cli/src/dialtest"
	"fmt"
)

func init() {
	dialtest.RegisterTicket("%%s")
	dialtest.AddSubtaskTest("logic-impl", RunLogicTest, nil)
	dialtest.AddSubtaskTest("integration-task", RunIntegrationTest, nil)
}

func RunLogicTest() error {
	fmt.Println("Running logic test (FIXED)...")
	return nil
}

func RunIntegrationTest() error {
	fmt.Println("Running integration test...")
	return nil
}
`, tempTicketName)
	if err := os.WriteFile(testGoPath, []byte(fixedTest), 0644); err != nil {
		return err
	}

	fmt.Println("Running 'ticket_v2 next' (expecting pass)...")
	runCmd("./dialtone.sh", "ticket_v2", "next")
	
	// G. Verify progress in ticket.md
	data, _ := os.ReadFile(ticketPath)
	if !strings.Contains(string(data), "status: done") {
		return fmt.Errorf("logic-impl should be done. Current content:\n%s", string(data))
	}
	fmt.Println("logic-impl verified as done.")

	fmt.Println("\n[PHASE 7: Completing dependencies]")
	// Should run integration-task automatically because next recurses, 
	// but let's call it again to be sure if recursion was stopped.
	runCmd("./dialtone.sh", "ticket_v2", "next")
	
	fmt.Println("Running 'ticket_v2 subtask list' to verify final state...")
	runCmd("./dialtone.sh", "ticket_v2", "subtask", "list")

	// J. Done validation - should fail if not all done
	fmt.Println("\n[PHASE 8: Ticket Completion (Expect Failure)]")
	cmd = exec.Command("./dialtone.sh", "ticket_v2", "done")
	output, _ = cmd.CombinedOutput()
	fmt.Println(string(output))
	if !strings.Contains(string(output), "is still") {
		return fmt.Errorf("expected completion failure but it passed")
	}

	// K. Test validation error demo
	fmt.Println("\n[PHASE 9: Demonstration of Validation Error]")
	cmd = exec.Command("./dialtone.sh", "ticket_v2", "validate", "fake-ticket-validate-error")
	output, _ = cmd.CombinedOutput()
	fmt.Println(string(output))
	if !strings.Contains(string(output), "missing '# Name:' header") {
		return fmt.Errorf("expected validation error not found")
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

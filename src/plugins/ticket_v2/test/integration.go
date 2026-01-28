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

	// 2. Run Lifecycle
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
	teardown()
	cleanupGit()

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

func runCmd(name string, args ...string) string {
	fmt.Printf("> %s %v\n", name, args)
	cmd := exec.Command(name, args...)
	output, _ := cmd.CombinedOutput()
	fmt.Println(string(output))
	return string(output)
}

func runLifecycle() error {
	ticketPath := filepath.Join(ticketV2Dir, tempTicketName, "ticket.md")
	testGoPath := filepath.Join(ticketV2Dir, tempTicketName, "test", "test.go")

	fmt.Println("\n[PHASE 2: Start and Branching]")
	runCmd("./dialtone.sh", "ticket_v2", "start", tempTicketName)
	
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branch, _ := cmd.Output()
	fmt.Printf("Active Branch: %s\n", strings.TrimSpace(string(branch)))

	fmt.Println("\n[PHASE 3: Defining Requirements]")
	content := fmt.Sprintf(`# Name: %s
# Goal
Show the full ticket_v2 lifecycle.

## SUBTASK: Logic Impl
- name: logic-impl
- description: Implement core business logic.
- test-condition-1: logic passes verification
- status: todo

## SUBTASK: Integration Task
- name: integration-task
- dependencies: logic-impl
- description: Integrate with existing modules.
- test-condition-1: integration is successful
- status: todo
`, tempTicketName)
	os.WriteFile(ticketPath, []byte(content), 0644)

	fmt.Println("\n[PHASE 4: Validating Format]")
	output := runCmd("./dialtone.sh", "ticket_v2", "validate", tempTicketName)
	if !strings.Contains(output, "is valid") {
		return fmt.Errorf("validation failed")
	}

	fmt.Println("\n[PHASE 5: TDD Loop - Failing Subtask]")
	testGoContent := fmt.Sprintf(`package test
import (
	"dialtone/cli/src/dialtest"
	"fmt"
)
func init() {
	dialtest.RegisterTicket("%s")
	dialtest.AddSubtaskTest("logic-impl", RunLogicTest, nil)
	dialtest.AddSubtaskTest("integration-task", RunIntegrationTest, nil)
}
func RunLogicTest() error {
	fmt.Println("Running logic test...")
	return fmt.Errorf("logic error: expected 42, got 0")
}
func RunIntegrationTest() error {
	return nil
}
`, tempTicketName)
	os.WriteFile(testGoPath, []byte(testGoContent), 0644)

	fmt.Println("Running 'ticket_v2 next' (expecting failure)...")
	output = runCmd("./dialtone.sh", "ticket_v2", "next")
	if !strings.Contains(output, "Subtask logic-impl failed") {
		return fmt.Errorf("expected failure message not found")
	}

	fmt.Println("\n[PHASE 6: Fixing and Promoting]")
	fixedTest := fmt.Sprintf(`package test
import (
	"dialtone/cli/src/dialtest"
	"fmt"
)
func init() {
	dialtest.RegisterTicket("%s")
	dialtest.AddSubtaskTest("logic-impl", RunLogicTest, nil)
	dialtest.AddSubtaskTest("integration-task", RunIntegrationTest, nil)
}
func RunLogicTest() error {
	fmt.Println("Running logic test (FIXED)...")
	return nil
}
func RunIntegrationTest() error {
	return nil
}
`, tempTicketName)
	os.WriteFile(testGoPath, []byte(fixedTest), 0644)

	fmt.Println("Running 'ticket_v2 next' (expecting pass)...")
	runCmd("./dialtone.sh", "ticket_v2", "next")
	
	data, _ := os.ReadFile(ticketPath)
	if !strings.Contains(string(data), "status: done") {
		return fmt.Errorf("logic-impl should be done")
	}

	fmt.Println("\n[PHASE 7: Completing dependencies]")
	runCmd("./dialtone.sh", "ticket_v2", "next")
	runCmd("./dialtone.sh", "ticket_v2", "subtask", "list")

	fmt.Println("\n[PHASE 8: Ticket Completion (Expect Failure)]")
	output = runCmd("./dialtone.sh", "ticket_v2", "done")
	if !strings.Contains(output, "is still") {
		return fmt.Errorf("expected completion failure but it passed")
	}

	fmt.Println("\n[PHASE 9: Demonstration of Validation Error]")
	output = runCmd("./dialtone.sh", "ticket_v2", "validate", "fake-ticket-validate-error")
	if !strings.Contains(output, "missing '# Name:' header") {
		return fmt.Errorf("expected validation error not found")
	}

	fmt.Println("\n[PHASE 10: Timestamp Regression Check]")
	regTicket := "fake-ticket-regression"
	regPath := filepath.Join(ticketV2Dir, regTicket, "ticket.md")
	os.MkdirAll(filepath.Join(ticketV2Dir, regTicket, "test"), 0755)
	regContent := fmt.Sprintf(`# Name: %s
# Goal
Test regression check.
## SUBTASK: Regress
- name: regress-task
- status: progress
- pass-timestamp: 2026-01-27T10:00:00Z
- fail-timestamp: 2026-01-27T11:00:00Z
`, regTicket)
	os.WriteFile(regPath, []byte(regContent), 0644)
	regGo := fmt.Sprintf(`package test
import "dialtone/cli/src/dialtest"
func init() {
	dialtest.RegisterTicket("%s")
	dialtest.AddSubtaskTest("regress-task", func() error { return nil }, nil)
}
`, regTicket)
	os.WriteFile(filepath.Join(ticketV2Dir, regTicket, "test", "test.go"), []byte(regGo), 0644)
	output = runCmd("./dialtone.sh", "ticket_v2", "validate", regTicket)
	if !strings.Contains(output, "[REGRESSION]") {
		return fmt.Errorf("expected [REGRESSION] error not found")
	}
	fmt.Println("Regression check verified.")
	os.RemoveAll(filepath.Join(ticketV2Dir, regTicket))

	return nil
}

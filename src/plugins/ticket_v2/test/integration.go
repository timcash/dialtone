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

	// B. Modify subtasks for testing
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

	// C. Register tests in test.go (simulated)
	// We need to make sure the tests are registered.
	// Since we are running outside the main binary's dev loop, we might need a way to mock dialtest.
	// But the user wants to verify the ticket_v2 API.
	
	// Let's implement a small test helper in the scaffolded test.go
	testGoPath := filepath.Join(ticketV2Dir, tempTicketName, "test", "test.go")
	testGoContent := fmt.Sprintf(`package test

import (
	"dialtone/cli/src/dialtest"
	"fmt"
)

func init() {
	dialtest.RegisterTicket("%%s")
	dialtest.AddSubtaskTest("init-task", func() error {
		fmt.Println("init-task running")
		return nil
	}, nil)
	dialtest.AddSubtaskTest("dependent-task", func() error {
		fmt.Println("dependent-task running")
		return nil
	}, nil)
}
`, tempTicketName)
	if err := os.WriteFile(testGoPath, []byte(testGoContent), 0644); err != nil {
		return err
	}

	// D. Run 'ticket_v2 next'
	// We need to be on the branch for 'next' to work easily in this implementation
	// For testing, let's try to pass the ticket name if possible, but my current impl of RunNext uses GetCurrentBranch.
	// I'll simulate a branch by checking out a temp branch.
	
	fmt.Println("Simulating work on branch...")
	exec.Command("git", "checkout", "-b", tempTicketName).Run()
	defer exec.Command("git", "checkout", "-").Run()
	defer exec.Command("git", "branch", "-D", tempTicketName).Run()

	fmt.Println("Running ticket_v2 next (1st time)...")
	cmd := exec.Command("./dialtone.sh", "ticket_v2", "next")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ticket_v2 next failed: %v, output: %s", err, string(output))
	}
	fmt.Println(string(output))

	// E. Verify progress in ticket.md
	data, _ := os.ReadFile(ticketPath)
	if !strings.Contains(string(data), "- status: done") {
		return fmt.Errorf("init-task should be done. Current content:\n%s", string(data))
	}
	fmt.Println("init-task verified as done.")

	return nil
}

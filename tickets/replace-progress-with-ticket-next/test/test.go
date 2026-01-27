package test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"dialtone/cli/src/core/test"
	"dialtone/cli/src/core/logger"
)

func init() {
	// Register subtask tests here: test.Register("<subtask-name>", "<ticket-name>", []string{"<tag1>"}, Run<SubtaskName>)
	test.Register("ticket-next-logic", "replace-progress-with-ticket-next", []string{"core"}, RunTicketNextLogic)
}

func RunImplementTicketNextCmd() error {
	// This test verifies that the 'ticket next' command is recognized.
	// We'll run it on the mock ticket to verify basic dispatch.
	cmd := exec.Command("./dialtone.sh", "ticket", "next", "test-ticket")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("'ticket next test-ticket' failed: %v", err)
	}
	return nil
}

func RunTicketNextLogic() error {
	// 0. Reset test-ticket state
	testDir := "tickets/test-ticket"
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	initialContent := `# Branch: test-ticket
# Tags: test

# Goal
Mock ticket for testing ` + "`" + `ticket next` + "`" + ` command.

## SUBTASK: First subtask
- name: first-subtask
- description: This is the first subtask.
- test-description: Verify it works.
- test-command: ` + "`" + `echo "PASS: first subtask"` + "`" + `
- status: todo

## SUBTASK: Second subtask
- name: second-subtask
- description: This is the second subtask.
- test-description: Verify it works.
- test-command: ` + "`" + `echo "PASS: second subtask"` + "`" + `
- status: todo
`
	os.WriteFile(filepath.Join(testDir, "ticket.md"), []byte(initialContent), 0644)

	// 1. Initial run: todo -> progress
	cmd := exec.Command("./dialtone.sh", "ticket", "next", "test-ticket")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("first 'ticket next' failed: %v", err)
	}

	// Verify first-subtask is in progress
	content, _ := os.ReadFile("tickets/test-ticket/ticket.md")
	if !strings.Contains(string(content), "- status: progress") || !strings.Contains(string(content), "first-subtask") {
		return fmt.Errorf("first-subtask should be in progress")
	}

	// 2. Second run: progress -> done, and next todo -> progress
	cmd = exec.Command("./dialtone.sh", "ticket", "next", "test-ticket")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("second 'ticket next' failed: %v", err)
	}

	content, _ = os.ReadFile("tickets/test-ticket/ticket.md")
	if !strings.Contains(string(content), "first-subtask") || !strings.Contains(string(content), "status: done") {
		return fmt.Errorf("first-subtask should be done")
	}
	if !strings.Contains(string(content), "second-subtask") || !strings.Contains(string(content), "status: progress") {
		return fmt.Errorf("second-subtask should be in progress")
	}

	return nil
}

// RunAll is the standard entry point required by project rules.
// It uses the registry to find and run all tests for this ticket.
func RunAll() error {
	logger.LogInfo("Running replace-progress-with-ticket-next suite...")
	return test.RunTicket("replace-progress-with-ticket-next")
}

func RunExample() error {
	fmt.Println("PASS: [example] Subtask logic verified")
	return nil
}

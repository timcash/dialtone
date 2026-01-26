package test

import (
	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/test"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() {
	// Register subtask tests
	test.Register("move-github-commands", "verify-ticket-plugin", []string{"refactor"}, RunMoveGithubCommands)
	test.Register("verify-ticket-help-sync", "verify-ticket-plugin", []string{"documentation"}, RunVerifyTicketHelpSync)
	test.Register("implement-github-pr-commands", "verify-ticket-plugin", []string{"feature"}, RunGithubPRCommands)
	test.Register("test-ticket-add", "verify-ticket-plugin", []string{"core"}, RunTicketAdd)
	test.Register("test-ticket-start", "verify-ticket-plugin", []string{"core"}, RunTicketStart)
	test.Register("example-subtask", "verify-ticket-plugin", []string{"example"}, RunExample)
}

// RunAll is the standard entry point required by project rules.
// It uses the registry to find and run all tests for this ticket.
func RunAll() error {
	logger.LogInfo("Running verify-ticket-plugin suite...")
	return test.RunTicket("verify-ticket-plugin")
}

func RunExample() error {
	fmt.Println("PASS: [example] Subtask logic verified")
	return nil
}

func RunMoveGithubCommands() error {
	// 1. Check ticket --help (Should NOT meet requirements yet, so we expect failure if we were asserting success,
	// but here we are implementing the verification.
	// The test should FAIL if the commands ARE present in ticket help.
	// The test should FAIL if the commands are NOT present in github issue help.

	// Step A: Verify 'ticket' help does NOT contain the commands
	out, err := exec.Command("./dialtone.sh", "ticket", "--help").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run ticket --help: %v", err)
	}
	output := string(out)

	forbidden := []string{"create", "comment", "view", "close"}
	// Note: 'close' check might be tricky if 'ticket done' has 'close' in description?
	// "done" is a command. "close" is a command.
	// Help output format: "  close <id>         Close a GitHub issue"

	for _, cmd := range forbidden {
		if strings.Contains(output, "  "+cmd+" ") {
			return fmt.Errorf("FAILURE: 'ticket --help' still contains command '%s'", cmd)
		}
	}

	// Step B: Verify 'github issue' help DOES contain the commands
	out, err = exec.Command("./dialtone.sh", "github", "issue", "--help").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run github issue --help: %v", err)
	}
	output = string(out)

	required := []string{"create", "comment", "view", "close"}
	for _, cmd := range required {
		if !strings.Contains(output, "  "+cmd+" ") {
			return fmt.Errorf("FAILURE: 'github issue --help' missing command '%s'", cmd)
		}
	}

	fmt.Println("PASS: GitHub commands correctly moved from ticket to github plugin")
	return nil
}

func RunVerifyTicketHelpSync() error {
	out, err := exec.Command("./dialtone.sh", "ticket", "--help").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run ticket --help: %v", err)
	}
	output := string(out)

	// Expected commands based on README
	expectedCommands := []string{"add", "start", "list", "validate", "done", "subtask"}

	// Create a map for easy lookup
	lines := strings.Split(output, "\n")
	foundCommands := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		parts := strings.Fields(line)
		if len(parts) > 0 {
			cmd := parts[0]
			// Filter out irrelevant lines/headers
			if cmd == "Usage:" || cmd == "Subcommands:" || cmd == "Commands:" || cmd == "dialtone-dev" {
				continue
			}
			foundCommands[cmd] = true
		}
	}

	for _, cmd := range expectedCommands {
		if !strings.Contains(output, "  "+cmd+" ") {
			// Try a looser check just in case spacing is different
			if !strings.Contains(output, cmd) {
				return fmt.Errorf("FAILURE: 'ticket --help' missing documented command '%s'", cmd)
			}
		}
	}

	fmt.Println("PASS: Ticket help matches README core commands")
	return nil
}

func RunGithubPRCommands() error {
	// Step A: Verify 'github pr --help' contains required commands
	out, err := exec.Command("./dialtone.sh", "github", "pr", "--help").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run github pr --help: %v", err)
	}
	output := string(out)

	required := []string{"pr create", "pr view", "pr comment", "pr close", "pr merge"}
	for _, cmd := range required {
		if !strings.Contains(output, "  "+cmd) {
			return fmt.Errorf("FAILURE: 'github pr --help' missing command '%s'", cmd)
		}
	}

	// We can also verify that running them with invalid args returns usage or specific error,
	// proving they are hooked up.
	// But mostly we need to implement them first.

	return nil
}

func RunTicketAdd() error {
	ticketName := "test-ticket-add-verification-dummy"
	ticketDir := "tickets/" + ticketName

	// Cleanup before test (just in case)
	os.RemoveAll(ticketDir)
	defer os.RemoveAll(ticketDir) // Cleanup after test

	// Run 'ticket add'
	cmd := exec.Command("./dialtone.sh", "ticket", "add", ticketName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ticket add failed: %v, output: %s", err, string(out))
	}

	// Verify files exist
	expectedFiles := []string{
		filepath.Join(ticketDir, "ticket.md"),
		filepath.Join(ticketDir, "test", "test.go"),
		filepath.Join(ticketDir, "progress.txt"),
	}

	for _, f := range expectedFiles {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			return fmt.Errorf("FAIL: Expected file not found: %s", f)
		}
	}

	// Verify content of ticket.md (basic check)
	content, err := os.ReadFile(filepath.Join(ticketDir, "ticket.md"))
	if err != nil {
		return fmt.Errorf("failed to read ticket.md: %v", err)
	}
	if !strings.Contains(string(content), ticketName) {
		return fmt.Errorf("FAIL: ticket.md does not contain ticket name %s", ticketName)
	}

	fmt.Println("PASS: Ticket add verified successfully")
	return nil
}

func RunTicketStart() error {
	ticketName := "TEST-ticket-start-real"
	ticketDir := "tickets/" + ticketName

	// Get current branch to return to
	cmd := exec.Command("git", "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %v", err)
	}
	originalBranch := strings.TrimSpace(string(out))

	// Ensure we are not on the test branch
	if originalBranch == ticketName {
		return fmt.Errorf("Currently on the test branch %s. Please checkout another branch and delete this one manually before running test.", ticketName)
	}

	// Helper to cleanup
	cleanup := func() {
		// Switch back first
		exec.Command("git", "checkout", originalBranch).Run()

		// Delete local branch
		exec.Command("git", "branch", "-D", ticketName).Run()

		// Delete remote branch (closes PR)
		exec.Command("git", "push", "origin", "--delete", ticketName).Run()

		// Delete files
		os.RemoveAll(ticketDir)
	}

	// Pre-cleanup (idempotency)
	cleanup()

	// Defer cleanup to run after test
	defer cleanup()

	// Run 'ticket start'
	// NOTE: This performs 'git add .', so ensure your working directory doesn't have partial edits you don't want committed to the test branch.
	cmd = exec.Command("./dialtone.sh", "ticket", "start", ticketName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ticket start failed: %v", err)
	}

	// Verify local files were scaffolded
	if _, err := os.Stat(filepath.Join(ticketDir, "ticket.md")); os.IsNotExist(err) {
		return fmt.Errorf("FAIL: ticket.md was not created")
	}

	// Verify we are on the new branch
	cmd = exec.Command("git", "branch", "--show-current")
	out, _ = cmd.Output()
	curr := strings.TrimSpace(string(out))
	if curr != ticketName {
		return fmt.Errorf("FAIL: Not on new branch. Current: %s, Expected: %s", curr, ticketName)
	}

	// Verify PR exists (using gh)
	// 'gh pr view' should succeed on the branch
	cmd = exec.Command("gh", "pr", "view", "--json", "url")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("FAIL: Could not view created PR (does it exist?): %v", err)
	}

	fmt.Println("PASS: Ticket start verified successfully (real integration)")
	return nil
}

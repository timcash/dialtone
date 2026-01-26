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
	test.Register("test-ticket-list", "verify-ticket-plugin", []string{"core"}, RunTicketList)
	test.Register("test-ticket-validate", "verify-ticket-plugin", []string{"core"}, RunTicketValidate)
	test.Register("test-ticket-subtask-list", "verify-ticket-plugin", []string{"core"}, RunTicketSubtaskList)
	test.Register("test-ticket-subtask-next", "verify-ticket-plugin", []string{"core"}, RunTicketSubtaskNext)
	test.Register("test-ticket-subtask-test", "verify-ticket-plugin", []string{"core"}, RunTicketSubtaskTest)
	test.Register("test-ticket-subtask-done", "verify-ticket-plugin", []string{"core"}, RunTicketSubtaskDone)
	test.Register("test-ticket-subtask-failed", "verify-ticket-plugin", []string{"core"}, RunTicketSubtaskFailed)
	test.Register("test-ticket-done", "verify-ticket-plugin", []string{"core"}, RunTicketDone)
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

func RunTicketList() error {
	ticketName := "test-ticket-list-dummy"
	ticketDir := "tickets/" + ticketName

	// Create dummy ticket manually to avoid full scaffold overhead/logs
	os.MkdirAll(ticketDir, 0755)
	os.WriteFile(filepath.Join(ticketDir, "ticket.md"), []byte("# Branch: "+ticketName), 0644)
	defer os.RemoveAll(ticketDir)

	cmd := exec.Command("./dialtone.sh", "ticket", "list")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ticket list failed: %v", err)
	}
	output := string(out)

	if !strings.Contains(output, ticketName) {
		return fmt.Errorf("FAIL: 'ticket list' output does not contain local ticket '%s'", ticketName)
	}

	// Also check if it lists "Remote GitHub Issues" header
	if !strings.Contains(output, "Remote GitHub Issues") {
		return fmt.Errorf("FAIL: 'ticket list' missing 'Remote GitHub Issues' section")
	}

	fmt.Println("PASS: Ticket list verified successfully")
	return nil
}

func RunTicketValidate() error {
	// 1. Valid Ticket
	validName := "test-ticket-validate-valid"
	validDir := "tickets/" + validName
	os.MkdirAll(validDir, 0755)
	defer os.RemoveAll(validDir)

	validContent := `# Ticket: Valid Ticket
# Goal
Some goal.

## SUBTASK: Subtask 1
- name: subtask-1
- description: desc
- test-description: test desc
- test-command: echo pass
- status: todo
`
	if err := os.WriteFile(filepath.Join(validDir, "ticket.md"), []byte(validContent), 0644); err != nil {
		return fmt.Errorf("failed to create valid ticket: %v", err)
	}

	cmd := exec.Command("./dialtone.sh", "ticket", "validate", validName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("FAIL: Valid ticket failed validation: %v. Output: %s", err, string(out))
	}

	// 2. Invalid Ticket (Missing status)
	invalidName := "test-ticket-validate-invalid"
	invalidDir := "tickets/" + invalidName
	os.MkdirAll(invalidDir, 0755)
	defer os.RemoveAll(invalidDir)

	invalidContent := `# Ticket: Invalid Ticket
## SUBTASK: Subtask 1
- name: subtask-1
- status: invalid-status-value
`
	if err := os.WriteFile(filepath.Join(invalidDir, "ticket.md"), []byte(invalidContent), 0644); err != nil {
		return fmt.Errorf("failed to create invalid ticket: %v", err)
	}

	cmd = exec.Command("./dialtone.sh", "ticket", "validate", invalidName)
	// We expect this to fail
	if err := cmd.Run(); err == nil {
		return fmt.Errorf("FAIL: Invalid ticket PASSED validation (it should have failed).Content:\n%s", invalidContent)
	}

	fmt.Println("PASS: Ticket validate verified successfully")
	return nil
}

func RunTicketSubtaskList() error {
	name := "test-ticket-subtask-list-dummy"
	dir := "tickets/" + name
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	content := `# Ticket: Dummy
## SUBTASK: Task A
- name: task-a
- status: done

## SUBTASK: Task B
- name: task-b
- status: todo
`
	if err := os.WriteFile(filepath.Join(dir, "ticket.md"), []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to create dummy ticket: %v", err)
	}

	cmd := exec.Command("./dialtone.sh", "ticket", "subtask", "list", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("subtask list failed: %v", err)
	}
	output := string(out)

	// Verify output contains subtask names and statuses
	checks := []string{
		"task-a", "done",
		"task-b", "todo",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			return fmt.Errorf("FAIL: Output missing expected string '%s'", check)
		}
	}

	fmt.Println("PASS: Ticket subtask list verified successfully")
	return nil
}

func RunTicketSubtaskNext() error {
	name := "test-ticket-subtask-next-dummy"
	dir := "tickets/" + name
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	content := `# Ticket: Dummy
## SUBTASK: Task A
- name: task-a
- status: done

## SUBTASK: Task B
- name: task-b
- status: todo

## SUBTASK: Task C
- name: task-c
- status: todo
`
	if err := os.WriteFile(filepath.Join(dir, "ticket.md"), []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to create dummy ticket: %v", err)
	}

	cmd := exec.Command("./dialtone.sh", "ticket", "subtask", "next", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("subtask next failed: %v", err)
	}
	output := string(out)

	// Verify it shows Task B (first todo)
	checks := []string{
		"task-b",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			return fmt.Errorf("FAIL: Output missing expected string '%s' (should be task-b). Output:\n%s", check, output)
		}
	}

	// Verify it does NOT show Task A or C details as "Next"
	// (Simple check: name should match task-b)
	if strings.Contains(output, "task-a") {
		return fmt.Errorf("FAIL: Output contains task-a which is done")
	}

	fmt.Println("PASS: Ticket subtask next verified successfully")
	return nil
}

func RunTicketSubtaskTest() error {
	name := "test-ticket-subtask-test-dummy"
	dir := "tickets/" + name
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	// Create ticket with a subtask that runs 'echo hello-world-test'
	content := `# Ticket: Dummy
## SUBTASK: Task A
- name: task-a
- test-command: echo hello-world-test
- status: todo
`
	if err := os.WriteFile(filepath.Join(dir, "ticket.md"), []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to create dummy ticket: %v", err)
	}

	cmd := exec.Command("./dialtone.sh", "ticket", "subtask", "test", name, "task-a")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("subtask test failed: %v", err)
	}
	output := string(out)

	if !strings.Contains(output, "hello-world-test") {
		return fmt.Errorf("FAIL: Output missing execution result 'hello-world-test'. Output:\n%s", output)
	}

	fmt.Println("PASS: Ticket subtask test verified successfully")
	return nil
}

func RunTicketSubtaskDone() error {
	name := "test-ticket-subtask-done-dummy"
	dir := "tickets/" + name
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	content := `# Ticket: Dummy
## SUBTASK: Task A
- name: task-a
- status: todo
`
	if err := os.WriteFile(filepath.Join(dir, "ticket.md"), []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to create dummy ticket: %v", err)
	}

	// 1. Run without progress.txt. Should fail on Progress Check.
	// NOTE: We assume the test environment *might* have git logic working.
	// However, 'git status' on untracked file 'dummy/progress.txt' returns "?? ...".
	// The code validation says: if output > 0, it is dirty.
	// But here progress.txt DOES NOT EXIST.
	// The code: exec("git", "status", ... path)
	// If file doesn't exist, git status might return nothing for that path or error?
	// Actually git status returns nothing if file is ignored or clean.
	// If file is missing and not tracked, it returns nothing.
	// So `isProgressUpdated` returns false.
	// So we expect "Error: ... has not been updated"

	cmd := exec.Command("./dialtone.sh", "ticket", "subtask", "done", name, "task-a")
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err == nil {
		return fmt.Errorf("FAIL: 'done' should have failed due to missing progress.txt")
	}
	if !strings.Contains(output, "has not been updated") {
		// It might have failed on "Git repository is not clean" if checking logic is skipped?
		// No, code says check progress first.
		return fmt.Errorf("FAIL: Expected progress.txt error, got: %s", output)
	}

	// 2. Create progress.txt. Run without flag. Should fail on Git Clean Check (assuming dirty dev env).
	// If the dev env happens to be clean, this test step relies on it being dirty?
	// That's flaky.
	// The integration test runner (me) knows I am editing files.
	// But ideally tests shouldn't rely on it.
	// However, if I make progress.txt DIRTY (untracked), then the repo IS DIRTY.
	// So Git Clean Check MUST FAIL.
	if err := os.WriteFile(filepath.Join(dir, "progress.txt"), []byte("Progress update"), 0644); err != nil {
		return fmt.Errorf("failed to create progress.txt: %v", err)
	}

	cmd = exec.Command("./dialtone.sh", "ticket", "subtask", "done", name, "task-a")
	out, err = cmd.CombinedOutput()
	output = string(out)

	if err == nil {
		return fmt.Errorf("FAIL: 'done' should have failed due to dirty git (untracked progress.txt)")
	}
	if !strings.Contains(output, "Git repository is not clean") {
		// It might fail on progress check?
		// isProgressUpdated returns true (dirty).
		// So it proceeds to git clean check.
		return fmt.Errorf("FAIL: Expected Git clean error, got: %s", output)
	}

	// 3. Run WITH flag. Should succeed.
	cmd = exec.Command("./dialtone.sh", "ticket", "subtask", "done", name, "task-a")
	cmd.Env = append(os.Environ(), "DIALTONE_DISABLE_GIT_CHECKS=1")
	out, err = cmd.CombinedOutput()
	output = string(out)

	if err != nil {
		return fmt.Errorf("FAIL: 'done' failed with override flag: %v. Output: %s", err, output)
	}

	// Verify status updated in ticket.md
	newContent, err := os.ReadFile(filepath.Join(dir, "ticket.md"))
	if err != nil {
		return fmt.Errorf("failed to read updated ticket.md: %v", err)
	}
	if !strings.Contains(string(newContent), "status: done") {
		return fmt.Errorf("FAIL: Status not updated to done. Content:\n%s", string(newContent))
	}

	fmt.Println("PASS: Ticket subtask done verified successfully")
	return nil
}

func RunTicketSubtaskFailed() error {
	name := "test-ticket-subtask-failed-dummy"
	dir := "tickets/" + name
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	content := `# Ticket: Dummy
## SUBTASK: Task A
- name: task-a
- status: todo
`
	if err := os.WriteFile(filepath.Join(dir, "ticket.md"), []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to create dummy ticket: %v", err)
	}

	// 1. Run without progress.txt. expect progress error
	cmd := exec.Command("./dialtone.sh", "ticket", "subtask", "failed", name, "task-a")
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err == nil {
		return fmt.Errorf("FAIL: 'failed' should have failed due to missing progress.txt")
	}
	if !strings.Contains(output, "has not been updated") {
		return fmt.Errorf("FAIL: Expected progress.txt error, got: %s", output)
	}

	// 2. Create progress.txt. Run without flag. expect git clean error
	if err := os.WriteFile(filepath.Join(dir, "progress.txt"), []byte("Failed reason"), 0644); err != nil {
		return fmt.Errorf("failed to create progress.txt: %v", err)
	}

	cmd = exec.Command("./dialtone.sh", "ticket", "subtask", "failed", name, "task-a")
	out, err = cmd.CombinedOutput()
	output = string(out)

	if err == nil {
		return fmt.Errorf("FAIL: 'failed' should have failed due to dirty git")
	}
	if !strings.Contains(output, "Git repository is not clean") {
		return fmt.Errorf("FAIL: Expected Git clean error, got: %s", output)
	}

	// 3. Run WITH flag. Should succeed.
	cmd = exec.Command("./dialtone.sh", "ticket", "subtask", "failed", name, "task-a")
	cmd.Env = append(os.Environ(), "DIALTONE_DISABLE_GIT_CHECKS=1")
	out, err = cmd.CombinedOutput()
	output = string(out)

	if err != nil {
		return fmt.Errorf("FAIL: 'failed' failed with override flag: %v. Output: %s", err, output)
	}

	// Verify status updated in ticket.md
	newContent, err := os.ReadFile(filepath.Join(dir, "ticket.md"))
	if err != nil {
		return fmt.Errorf("failed to read updated ticket.md: %v", err)
	}
	if !strings.Contains(string(newContent), "status: failed") {
		return fmt.Errorf("FAIL: Status not updated to failed. Content:\n%s", string(newContent))
	}

	fmt.Println("PASS: Ticket subtask failed verified successfully")
	return nil
}

func RunTicketDone() error {
	name := "test-ticket-done-dummy"
	dir := "tickets/" + name
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	content := `# Ticket: Dummy
## SUBTASK: Task A
- name: task-a
- status: done

## SUBTASK: Ticket Done
- name: ticket-done
- status: todo
`
	if err := os.WriteFile(filepath.Join(dir, "ticket.md"), []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to create dummy ticket: %v", err)
	}

	// 1. Validate failure (git checks)
	cmd := exec.Command("./dialtone.sh", "ticket", "done", name)
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err == nil {
		return fmt.Errorf("FAIL: 'ticket done' should have failed due to git/progress checks")
	}
	if !strings.Contains(output, "has not been updated") && !strings.Contains(output, "Git repository is not clean") {
		return fmt.Errorf("FAIL: Expected git validation error, got: %s", output)
	}

	// 2. Validate bypass and progression to push
	cmd = exec.Command("./dialtone.sh", "ticket", "done", name)
	cmd.Env = append(os.Environ(), "DIALTONE_DISABLE_GIT_CHECKS=1")
	out, err = cmd.CombinedOutput()
	output = string(out)

	// We expect failure at "Pushing latest changes...", or "Failed to push"
	// because we haven't set up a git remote for this dummy path.
	// Actually `git push` might fail because no upstream.
	if !strings.Contains(output, "Failed to push changes") && !strings.Contains(output, "Pushing local changes") {
		// If it succeeded? Implies git push worked? Unlikely.
		// If it failed earlier?
		if strings.Contains(output, "Git repository is not clean") {
			return fmt.Errorf("FAIL: Bypass didn't work. Output: %s", output)
		}
		// If it says "Pushing latest changes" but then fails...
		// ticket.go: logInfo("Pushing latest changes to origin...")
		if strings.Contains(output, "Pushing latest changes to origin") {
			fmt.Println("PASS: Ticket done verified (reached push stage)")
			return nil
		}
		return fmt.Errorf("FAIL: Unexpected output from ticket done bypass: %s", output)
	}

	fmt.Println("PASS: Ticket done verified (reached push stage)")
	return nil
}

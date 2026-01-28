package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const ticketV2Dir = "src/tickets_v2"
const testDataDir = "src/plugins/ticket_v2/test"

func main() {
	// Check git hygiene
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusOutput, _ := statusCmd.Output()
	if len(strings.TrimSpace(string(statusOutput))) > 0 {
		fmt.Println("FATAL: Git status is not clean. Please commit or stash changes before running integration tests.")
		os.Exit(1)
	}

	initialBranch := getCurrentBranch()
	fmt.Printf("=== Starting ticket_v2 Granular Integration Tests (Initial Branch: %s) ===\n", initialBranch)

	defer func() {
		fmt.Printf("\n=== Restoring Initial Branch: %s ===\n", initialBranch)
		exec.Command("git", "checkout", "-f", initialBranch).Run()
	}()

	runTest("ticket_v2 add", TestAddGranular)
	runTest("ticket_v2 start", TestStartGranular)
	runTest("ticket_v2 next", TestNextGranular)
	runTest("ticket_v2 validate", TestValidateGranular)
	runTest("ticket_v2 done", TestDoneGranular)
	runTest("subtask basics", TestSubtaskBasicsGranular)
	runTest("subtask done/failed", TestSubtaskDoneFailedGranular)

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

func getUniqueName(base string) string {
	return fmt.Sprintf("%s-%d", base, time.Now().Unix())
}

func restoreBranch(branch string) {
	exec.Command("git", "checkout", "-f", branch).Run()
}

func cleanupRemote(branch string) {
	fmt.Printf("--- Cleaning up remote branch and PR for %s ---\n", branch)
	exec.Command("gh", "pr", "close", branch, "--delete-branch").Run()
	exec.Command("git", "push", "origin", "--delete", branch).Run()
}

func TestAddGranular() error {
	initialBranch := getCurrentBranch()
	name := getUniqueName("test-add")
	os.RemoveAll(filepath.Join(ticketV2Dir, name))
	defer os.RemoveAll(filepath.Join(ticketV2Dir, name))
	
	output := runCmd("./dialtone.sh", "ticket_v2", "add", name)
	if !strings.Contains(output, "Created") {
		return fmt.Errorf("expected 'Created' message")
	}

	// Verify files exist
	if _, err := os.Stat(filepath.Join(ticketV2Dir, name, "ticket.md")); err != nil {
		return fmt.Errorf("ticket.md missing")
	}
	if _, err := os.Stat(filepath.Join(ticketV2Dir, name, "test", "test.go")); err != nil {
		return fmt.Errorf("test/test.go missing")
	}

	// Verify NO branch change
	if getCurrentBranch() != initialBranch {
		return fmt.Errorf("branch should NOT have changed")
	}

	return nil
}

func TestStartGranular() error {
	initialBranch := getCurrentBranch()
	name := getUniqueName("test-start")
	os.RemoveAll(filepath.Join(ticketV2Dir, name))
	defer os.RemoveAll(filepath.Join(ticketV2Dir, name))
	defer exec.Command("git", "branch", "-D", name).Run()
	defer cleanupRemote(name)
	defer restoreBranch(initialBranch)

	output := runCmd("./dialtone.sh", "ticket_v2", "start", name)
	
	checks := []string{
		"Branching to " + name,
		"Pushing branch " + name,
		"Creating Draft Pull Request",
		"Ticket " + name + " started successfully",
	}
	for _, c := range checks {
		if !strings.Contains(output, c) {
			return fmt.Errorf("missing log check: %s", c)
		}
	}

	// Verify we are on the branch
	if getCurrentBranch() != name {
		return fmt.Errorf("not on expected branch: %s", getCurrentBranch())
	}

	return nil
}

func TestNextGranular() error {
	initialBranch := getCurrentBranch()
	name := getUniqueName("test-next")
	os.RemoveAll(filepath.Join(ticketV2Dir, name))
	defer os.RemoveAll(filepath.Join(ticketV2Dir, name))
	defer restoreBranch(initialBranch)

	runCmd("./dialtone.sh", "ticket_v2", "add", name)

	// Sub-item 2: Dependency Check & Auto-Promotion
	ticketPath := filepath.Join(ticketV2Dir, name, "ticket.md")
	content := fmt.Sprintf(`# Name: %s
# Goal
Granular next test
## SUBTASK: Task1
- name: t1
- status: todo
## SUBTASK: Task2
- name: t2
- dependencies: t1
- status: todo
`, name)
	os.WriteFile(ticketPath, []byte(content), 0644)

	fmt.Println("--- Checking Auto-Promotion/Execution ---")
	output := runCmd("./dialtone.sh", "ticket_v2", "next", name)
	if !strings.Contains(output, "Promoting subtask t1 to progress") {
		return fmt.Errorf("failed auto-promotion")
	}
	if !strings.Contains(output, "Subtask t1 failed") {
		return fmt.Errorf("expected failure since no test logic added yet")
	}
	if !strings.Contains(output, "Fail-Timestamp:") {
		return fmt.Errorf("missing fail-timestamp")
	}

	// Sub-item 5: Auto-commit on Pass
	fmt.Println("--- Checking Auto-commit on Pass ---")
	testGoPath := filepath.Join(ticketV2Dir, name, "test", "test.go")
	os.WriteFile(testGoPath, []byte(fmt.Sprintf(`package test
import "dialtone/cli/src/dialtest"
func init() {
	dialtest.RegisterTicket("%s")
	dialtest.AddSubtaskTest("t1", func() error { return nil }, nil)
}
`, name)), 0644)

	output = runCmd("./dialtone.sh", "ticket_v2", "next", name)
	if !strings.Contains(output, "Subtask t1 passed") {
		return fmt.Errorf("expected pass message")
	}
	
	cmd := exec.Command("git", "log", "-1", "--pretty=format:%s")
	logMsg, _ := cmd.Output()
	if !strings.Contains(string(logMsg), "docs: subtask t1 passed") {
		return fmt.Errorf("failed auto-commit check: got %q", string(logMsg))
	}

	return nil
}

func TestValidateGranular() error {
	fmt.Println("--- Checking Timestamp Regression ---")
	name := getUniqueName("test-validate")
	os.MkdirAll(filepath.Join(ticketV2Dir, name), 0755)
	defer os.RemoveAll(filepath.Join(ticketV2Dir, name))
	os.WriteFile(filepath.Join(ticketV2Dir, name, "ticket.md"), []byte("# Name: "+name+"\n\n## SUBTASK: R\n- name: r\n- pass-timestamp: 2026-01-27T10:00:00Z\n- fail-timestamp: 2026-01-27T11:00:00Z\n- status: done\n"), 0644)
	
	output := runCmd("./dialtone.sh", "ticket_v2", "validate", name)
	if !strings.Contains(output, "[REGRESSION]") {
		return fmt.Errorf("failed regression detection")
	}

	return nil
}

func TestDoneGranular() error {
	initialBranch := getCurrentBranch()
	name := getUniqueName("test-done")
	os.RemoveAll(filepath.Join(ticketV2Dir, name))
	defer os.RemoveAll(filepath.Join(ticketV2Dir, name))
	defer exec.Command("git", "branch", "-D", name).Run()
	defer cleanupRemote(name)
	defer restoreBranch(initialBranch)

	runCmd("./dialtone.sh", "ticket_v2", "start", name)
	runCmd("./dialtone.sh", "ticket_v2", "subtask", "done", name, "init")

	// Hygiene check
	fmt.Println("--- Checking Git Hygiene (Expected Failure) ---")
	os.WriteFile("dirty.txt", []byte("trash"), 0644)
	output := runCmd("./dialtone.sh", "ticket_v2", "done")
	if !strings.Contains(output, "Git status is not clean") {
		os.Remove("dirty.txt")
		return fmt.Errorf("failed hygiene check")
	}
	os.Remove("dirty.txt")

	// Success check
	fmt.Println("--- Checking Success ---")
	output = runCmd("./dialtone.sh", "ticket_v2", "done")
	checks := []string{
		"Pushing final changes",
		"Marking PR as ready for review",
		"Switching back to main branch",
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
	os.RemoveAll(filepath.Join(ticketV2Dir, name))
	defer os.RemoveAll(filepath.Join(ticketV2Dir, name))
	runCmd("./dialtone.sh", "ticket_v2", "add", name)
	
	// subtask list
	output := runCmd("./dialtone.sh", "ticket_v2", "subtask", "list", name)
	if !strings.Contains(output, "Subtasks for "+name) {
		return fmt.Errorf("failed subtask list")
	}
	
	return nil
}

func TestSubtaskDoneFailedGranular() error {
	initialBranch := getCurrentBranch()
	name := getUniqueName("test-sub-state")
	os.RemoveAll(filepath.Join(ticketV2Dir, name))
	defer os.RemoveAll(filepath.Join(ticketV2Dir, name))
	defer exec.Command("git", "branch", "-D", name).Run()
	defer cleanupRemote(name)
	defer restoreBranch(initialBranch)

	runCmd("./dialtone.sh", "ticket_v2", "start", name)

	// Hygiene
	os.WriteFile("dirty.txt", []byte("trash"), 0644)
	output := runCmd("./dialtone.sh", "ticket_v2", "subtask", "done", name, "init")
	if !strings.Contains(output, "Git status is not clean") {
		os.Remove("dirty.txt")
		return fmt.Errorf("subtask hygiene fail")
	}
	os.Remove("dirty.txt")

	// Auto-commit
	runCmd("./dialtone.sh", "ticket_v2", "subtask", "done", name, "init")
	cmd := exec.Command("git", "log", "-1", "--pretty=format:%s")
	logMsg, _ := cmd.Output()
	if !strings.Contains(string(logMsg), "docs: subtask init done") {
		return fmt.Errorf("subtask auto-commit fail")
	}

	return nil
}

func getCurrentBranch() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, _ := cmd.Output()
	return strings.TrimSpace(string(output))
}

func runCmd(name string, args ...string) string {
	fmt.Printf("> %s %v\n", name, args)
	cmd := exec.Command(name, args...)
	output, _ := cmd.CombinedOutput()
	fmt.Println(string(output))
	return string(output)
}

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func logInfo(format string, args ...interface{}) {
	fmt.Printf("[ticket] "+format+"\n", args...)
}

func logFatal(format string, args ...interface{}) {
	fmt.Printf("[ticket] FATAL: "+format+"\n", args...)
	os.Exit(1)
}

// RunStart handles 'ticket start <ticket-name>'
func RunStart(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ticket start <ticket-name> [--plugin <plugin-name>]")
	}

	arg := args[0]

	ticketName := GetTicketName(arg)

	// 1. Create/Switch Git Branch
	// Check if branch exists
	cmd := exec.Command("git", "branch", "--list", ticketName)
	output, err := cmd.Output()
	if err != nil {
		logFatal("Failed to check git branches: %v", err)
	}

	if strings.TrimSpace(string(output)) != "" {
		logInfo("Switching to existing branch: %s", ticketName)
		if err := exec.Command("git", "checkout", ticketName).Run(); err != nil {
			logFatal("Failed to checkout branch: %v", err)
		}
	} else {
		logInfo("Creating new branch: %s", ticketName)
		if err := exec.Command("git", "checkout", "-b", ticketName).Run(); err != nil {
			logFatal("Failed to create branch: %v", err)
		}
	}

	// 2. Ticket Scaffolding
	ticketDir := filepath.Join("tickets", ticketName)
	ensureDir(ticketDir)
	ensureDir(filepath.Join(ticketDir, "code"))
	ensureDir(filepath.Join(ticketDir, "test"))

	// Create test templates
	createTestTemplates(filepath.Join(ticketDir, "test"), ticketName)

	// Copy ticket.md template
	ticketMd := filepath.Join(ticketDir, "ticket.md")
	if _, err := os.Stat(ticketMd); os.IsNotExist(err) {
		templatePath := filepath.Join("docs", "ticket-template.md")
		content, err := os.ReadFile(templatePath)
		if err == nil {
			// Replace placeholders
			newContent := strings.ReplaceAll(string(content), "<branch-name>", ticketName)
			newContent = strings.ReplaceAll(newContent, "<ticket-name>", ticketName)
			
			if err := os.WriteFile(ticketMd, []byte(newContent), 0644); err != nil {
				logFatal("Failed to write ticket.md: %v", err)
			}
			logInfo("Created %s from %s", ticketMd, templatePath)
		} else {
			logInfo("Warning: Template %s not found, skipping ticket.md creation.", templatePath)
		}
	}

	progressTxt := filepath.Join(ticketDir, "progress.txt")
	if _, err := os.Stat(progressTxt); os.IsNotExist(err) {
		content := fmt.Sprintf("Progress log for %s\n\n", ticketName)
		if err := os.WriteFile(progressTxt, []byte(content), 0644); err != nil {
			logFatal("Failed to create progress.txt: %v", err)
		}
		logInfo("Created %s", progressTxt)
	}

	// 2.5 Audit & Commit
	logInfo("Committing scaffolding...")
	if err := exec.Command("git", "add", ".").Run(); err != nil {
		logFatal("Failed to git add: %v", err)
	}
	commitMsg := fmt.Sprintf("chore: start ticket %s", ticketName)
	if err := exec.Command("git", "commit", "-m", commitMsg).Run(); err != nil {
		logFatal("Failed to commit: %v", err)
	}

	// 3. Git Push & PR
	logInfo("Pushing branch to origin...")
	pushCmd := exec.Command("git", "push", "-u", "origin", ticketName)
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr
	if err := pushCmd.Run(); err != nil {
		logFatal("Failed to push branch: %v", err)
	}

	logInfo("Waiting for GitHub to sync...")
	// Wait a moment for GitHub to register the new branch before creating PR
	time.Sleep(2 * time.Second)

	logInfo("Creating Pull Request...")
	prCmd := exec.Command("./dialtone.sh", "github", "pr")
	prCmd.Stdout = os.Stdout
	prCmd.Stderr = os.Stderr
	if err := prCmd.Run(); err != nil {
		logFatal("Failed to create PR: %v", err)
	}

	logInfo("Ticket %s started successfully", ticketName)
	logReminder(ticketName)
}



// RunTest handles 'ticket test <ticket-name>'
func RunTest(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ticket test <ticket-name>")
	}
	ticketName := args[0]
	testDir := filepath.Join("tickets", ticketName, "test")

	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		logFatal("Test directory not found: %s", testDir)
	}

	logInfo("Running tests in %s...", testDir)
	cmd := exec.Command("go", "test", "-v", "./"+testDir+"/...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logFatal("Tests failed: %v", err)
	}
	logInfo("All tests passed.")
	logReminder(ticketName)
}

// RunDone handles 'ticket done <ticket-name>'
// RunDone handles 'ticket done <ticket-name>'
func RunDone(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ticket done <ticket-name>")
	}
	ticketName := args[0]

	// 1. Verify all subtasks are done (except 'ticket-done')
	subtasks, err := parseSubtasks(ticketName)
	if err != nil {
		logFatal("Failed to parse subtasks: %v", err)
	}
	for _, st := range subtasks {
		if st.Name != "ticket-done" && st.Status != "done" {
			logFatal("Subtask '%s' is not done (status: %s). All subtasks must be done before completing the ticket.", st.Name, st.Status)
		}
	}
	logInfo("All subtasks verified as done (excluding 'ticket-done').")

	// 2. Run all tests
	logInfo("Running all tests...")
	testCmd := exec.Command("./dialtone.sh", "test")
	testCmd.Stdout = os.Stdout
	testCmd.Stderr = os.Stderr
	if err := testCmd.Run(); err != nil {
		logFatal("Tests failed: %v", err)
	}
	logInfo("All tests passed.")

	// 3. Verify git status
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		logFatal("Failed to run git status: %v", err)
	}
	if len(strings.TrimSpace(string(output))) > 0 {
		logFatal("Uncommitted changes detected. Please commit or stash them before running ticket done.\n%s", string(output))
	}
	logInfo("Git status clean.")

	// 4. GitHub PR
	logInfo("Updating Pull Request...")
	prCmd := exec.Command("./dialtone.sh", "github", "pr")
	prCmd.Stdout = os.Stdout
	prCmd.Stderr = os.Stderr
	if err := prCmd.Run(); err != nil {
		logFatal("Failed to update PR: %v", err)
	}

	// 5. Mark 'ticket-done' as done
	// We'll proceed even if it doesn't exist, but if it does, it will now be marked done.
	// Since parseSubtasks worked, we know if it exists.
	for _, st := range subtasks {
		if st.Name == "ticket-done" {
			RunSubtaskDone(ticketName, "ticket-done")
			logInfo("Marked 'ticket-done' subtask as done.")
			break
		}
	}

	logInfo("Ticket %s done setup complete.", ticketName)
	logReminder(ticketName)
}

func logReminder(ticketName string) {
	fmt.Printf("\nREMINDER: remember to update tickets/%s/progress.txt with important notes\n", ticketName)
}

func ensureDir(path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		logFatal("Failed to create directory %s: %v", path, err)
	}
}

// GetTicketName parses the ticket name from an argument (name or file path)
func GetTicketName(arg string) string {
	if strings.HasSuffix(arg, ".md") {
		if _, err := os.Stat(arg); err == nil {
			// It's a file, try to parse Branch
			content, err := os.ReadFile(arg)
			if err != nil {
				// Should not happen if Stat succeeded, but log just in case
				logInfo("Failed to read ticket file: %v", err)
				return ""
			}
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "# Branch:") {
					name := strings.TrimSpace(strings.TrimPrefix(line, "# Branch:"))
					logInfo("Parsed ticket name from file: %s", name)
					return name
				}
			}

			logInfo("No '# Branch:' found in %s, using filename as ticket name", arg)
			base := filepath.Base(arg)
			return strings.TrimSuffix(base, filepath.Ext(base))
		} else {
			// Ends in .md but doesn't exist
			logInfo("Ticket name '%s' ends in .md but file not found; treating as ticket name.", arg)
			return arg
		}
	}
	return arg
}

func createTestTemplates(testDir, ticketName string) {
	// Clean up ticket name for package (replace - with _)
	pkgName := strings.ReplaceAll(ticketName, "-", "_")

	templates := map[string]string{
		"unit_test.go": fmt.Sprintf(`package test

import "testing"

func TestUnit_Example(t *testing.T) {
	t.Log("Unit test for %s")
	t.Fatal("Not implemented")
}
`, pkgName),
		"integration_test.go": fmt.Sprintf(`package test

import "testing"

func TestIntegration_Example(t *testing.T) {
	t.Log("Integration test for %s")
	t.Fatal("Not implemented")
}
`, pkgName),
		"e2e_test.go": fmt.Sprintf(`package test

import "testing"

func TestE2E_Example(t *testing.T) {
	t.Log("E2E test for %s")
	t.Fatal("Not implemented")
}
`, pkgName),
	}

	for filename, content := range templates {
		fullPath := filepath.Join(testDir, filename)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				logFatal("Failed to create test file %s: %v", filename, err)
			}
			logInfo("Created test template: %s", fullPath)
		}
	}
}

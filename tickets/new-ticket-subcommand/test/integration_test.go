package test

import (
	"dialtone/cli/src/plugins/ticket/cli"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplateCopy(t *testing.T) {
	// 1. Setup temporary workspace or just use a unique ticket name
	ticketName := "test-ticket-creation"
	// Root of repo
	cwd, _ := os.Getwd()
	repoRoot, err := filepath.Abs(filepath.Join(cwd, "..", "..", ".."))
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	ticketDir := filepath.Join(repoRoot, "tickets", ticketName)
	ticketMd := filepath.Join(ticketDir, "ticket.md")

	// Ensure cleanup
	defer os.RemoveAll(ticketDir)

	// 2. Run implementation (RunNew)
	// We need to be careful with paths since RunNew uses relative paths
	// In the test, we'll run it from the repo root if possible,
	// but RunNew assumes being at the root.

	// Create a mock template if it doesn't exist (it should in reality)
	templateDir := filepath.Join(repoRoot, "tickets", "template-ticket")
	os.MkdirAll(templateDir, 0755)
	templateFile := filepath.Join(templateDir, "ticket.md")
	if _, err := os.Stat(templateFile); os.IsNotExist(err) {
		content := "# Branch: ticket-short-name\n## #SUBTASK: Test\n- status: todo"
		os.WriteFile(templateFile, []byte(content), 0644)
	}

	// Move to repo root for the test
	os.Chdir(repoRoot)
	defer os.Chdir(cwd)

	// Refresh ticketName path relative to repo root
	ticketDir = filepath.Join("tickets", ticketName)
	ticketMd = filepath.Join(ticketDir, "ticket.md")

	cli.RunNew([]string{ticketName})

	// 3. Verify
	if _, err := os.Stat(ticketMd); os.IsNotExist(err) {
		t.Fatalf("ticket.md was not created at %s", ticketMd)
	}

	content, err := os.ReadFile(ticketMd)
	if err != nil {
		t.Fatalf("Failed to read created ticket: %v", err)
	}

	if !strings.Contains(string(content), "# Branch: "+ticketName) {
		t.Errorf("Placeholder not replaced. Content: %s", string(content))
	}
}

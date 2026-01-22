package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegration_TicketStartScaffolding(t *testing.T) {
	// 1. Setup
	ticketName := "verification-scaffold"
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Assuming we run this from project root or having access to dialtone-dev.go
	// However, tests are usually run from the package directory.
	// We need to locate dialtone-dev.go.
	// Let's assume project root is 4 levels up from src/plugins/ticket/test
	projectRoot := filepath.Join(cwd, "../../../..")
	dialtoneDev := filepath.Join(projectRoot, "src/cmd/dev/main.go")

	if _, err := os.Stat(dialtoneDev); os.IsNotExist(err) {
		// Fallback: maybe we are running from project root
		if _, err := os.Stat("src/cmd/dev/main.go"); err == nil {
			projectRoot = "."
			dialtoneDev = "src/cmd/dev/main.go"
		} else {
			t.Skip("Could not locate src/cmd/dev/main.go for integration test")
		}
	}

	// Cleanup before start
	cleanup(t, projectRoot, ticketName)
	defer cleanup(t, projectRoot, ticketName)

	// 2. Run ticket start
	cmd := exec.Command("go", "run", dialtoneDev, "ticket", "start", ticketName)
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to run ticket start: %v", err)
	}

	// 3. Verify Files Exist
	expectedFiles := []string{
		filepath.Join("tickets", ticketName, "test", "unit_test.go"),
		filepath.Join("tickets", ticketName, "test", "integration_test.go"),
		filepath.Join("tickets", ticketName, "test", "e2e_test.go"),
	}

	for _, relPath := range expectedFiles {
		fullPath := filepath.Join(projectRoot, relPath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("Expected file not found: %s", fullPath)
		}
	}

	// 4. Verify Content (Simple check)
	unitTestPath := filepath.Join(projectRoot, "tickets", ticketName, "test", "unit_test.go")
	content, err := os.ReadFile(unitTestPath)
	if err != nil {
		t.Fatalf("Failed to read unit_test.go: %v", err)
	}
	if !strings.Contains(string(content), "verification_scaffold") {
		t.Errorf("Expected package/test name to contain 'verification_scaffold', got: %s", string(content))
	}
}

func TestIntegration_TicketDoneStatus(t *testing.T) {
	// 1. Setup
	ticketName := "verification-done-status"
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	projectRoot := filepath.Join(cwd, "../../../..")
	dialtoneDev := filepath.Join(projectRoot, "src/cmd/dev/main.go")

	if _, err := os.Stat(dialtoneDev); os.IsNotExist(err) {
		// Fallback
		if _, err := os.Stat("src/cmd/dev/main.go"); err == nil {
			projectRoot = "."
			dialtoneDev = "src/cmd/dev/main.go"
		} else {
			t.Skip("Could not locate src/cmd/dev/main.go for integration test")
		}
	}

	cleanup(t, projectRoot, ticketName)
	defer cleanup(t, projectRoot, ticketName)

	// Create ticket dir
	ticketDir := filepath.Join(projectRoot, "tickets", ticketName)
	testDir := filepath.Join(ticketDir, "test")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create ticket structure: %v", err)
	}

	// 2. Failure Case: Incomplete Status
	ticketMd := filepath.Join(ticketDir, "ticket.md")
	incompleteContent := []byte("# Branch: foo\n\n## Subtask 1\n- status: todo\n")
	if err := os.WriteFile(ticketMd, incompleteContent, 0644); err != nil {
		t.Fatalf("Failed to create incomplete ticket.md: %v", err)
	}

	// Create dummy test file so "go test" doesn't fail due to no files
	dummyTest := filepath.Join(testDir, "dummy_test.go")
	dummyContent := []byte("package test\nimport \"testing\"\nfunc TestDummy(t *testing.T) { t.Log(\"Dummy\") }\n")
	if err := os.WriteFile(dummyTest, dummyContent, 0644); err != nil {
		t.Fatalf("Failed to create dummy test: %v", err)
	}

	cmd := exec.Command("go", "run", dialtoneDev, "ticket", "done", ticketName)
	cmd.Dir = projectRoot
	if err := cmd.Run(); err == nil {
		t.Errorf("Expected 'ticket done' to fail with incomplete status, but it succeeded")
	}

	// 3. Success Case: Complete Status
	completeContent := []byte("# Branch: foo\n\n## Subtask 1\n- status: done\n")
	if err := os.WriteFile(ticketMd, completeContent, 0644); err != nil {
		t.Fatalf("Failed to update ticket.md: %v", err)
	}

	cmd = exec.Command("go", "run", dialtoneDev, "ticket", "done", ticketName)
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Errorf("Expected 'ticket done' to succeed with complete status, but failed: %v", err)
	}
}

func cleanup(t *testing.T, root, ticketName string) {
	ticketDir := filepath.Join(root, "tickets", ticketName)
	if err := os.RemoveAll(ticketDir); err != nil {
		t.Logf("Failed to cleanup %s: %v", ticketDir, err)
	}
	// Also delete branch if possible, but might fail if active.
	// Ignoring git branch cleanup for simplicity in this test as it runs local commands.
}

package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2E_TicketNew(t *testing.T) {
	// Root of repo
	cwd, _ := os.Getwd()
	// tickets/new-ticket-subcommand/test -> 3 levels up to root
	repoRoot, err := filepath.Abs(filepath.Join(cwd, "..", "..", ".."))
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	ticketName := "e2e-test-ticket"
	ticketDir := filepath.Join(repoRoot, "tickets", ticketName)
	ticketMd := filepath.Join(ticketDir, "ticket.md")

	// Cleanup before/after
	os.RemoveAll(ticketDir)
	defer os.RemoveAll(ticketDir)

	// Run command with absolute path to dialtone.sh
	dialtoneSh := filepath.Join(repoRoot, "dialtone.sh")
	cmd := exec.Command(dialtoneSh, "ticket", "new", ticketName)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, string(output))
	}

	// Verify
	content, err := os.ReadFile(ticketMd)
	if err != nil {
		t.Fatalf("Failed to read ticket: %v", err)
	}

	if !strings.Contains(string(content), "# Branch: "+ticketName) {
		t.Errorf("Expected branch name not found in %s", ticketMd)
	}
}

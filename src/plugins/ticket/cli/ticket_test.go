package cli

import (
	"os"
	"testing"
)

func TestGetTicketName(t *testing.T) {
	// 1. Simple Name
	name := GetTicketName("simple-ticket")
	if name != "simple-ticket" {
		t.Errorf("Expected 'simple-ticket', got '%s'", name)
	}

	// 2. File Path with Branch
	tmpFile := "test_ticket_with_branch.md"
	content := []byte("# Branch: branch-from-file\n# Task: Test\n")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile)

	name = GetTicketName(tmpFile)
	if name != "branch-from-file" {
		t.Errorf("Expected 'branch-from-file', got '%s'", name)
	}

	// 3. File Path without Branch (derives from filename)
	tmpFile2 := "test_ticket_filename.md"
	content2 := []byte("# Task: Test\n")
	if err := os.WriteFile(tmpFile2, content2, 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile2)

	name = GetTicketName(tmpFile2)
	// Base name is test_ticket_filename.md -> test_ticket_filename
	if name != "test_ticket_filename" {
		t.Errorf("Expected 'test_ticket_filename', got '%s'", name)
	}

	// 4. File Path (non-existent)
	name = GetTicketName("non_existent_file.md")
	if name != "non_existent_file.md" {
		t.Errorf("Expected 'non_existent_file.md', got '%s'", name)
	}
}

package ticket_test

import (
	"fmt"
	"os"

	"dialtone/cli/src/plugins/ticket/cli"
	"dialtone/cli/src/core/logger"
)

// RunAll runs all tests in this package
func RunAll() error {
	if err := RunGetTicketNameTests(); err != nil {
		return fmt.Errorf("GetTicketName tests failed: %v", err)
	}
	return nil
}

func RunGetTicketNameTests() error {
	// 1. Simple Name
	name := cli.GetTicketName("simple-ticket")
	if name != "simple-ticket" {
		return fmt.Errorf("Expected 'simple-ticket', got '%s'", name)
	}

	// 2. File Path with Branch
	tmpFile := "test_ticket_with_branch.md"
	content := []byte("# Branch: branch-from-file\n# Task: Test\n")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		return fmt.Errorf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile)

	name = cli.GetTicketName(tmpFile)
	if name != "branch-from-file" {
		return fmt.Errorf("Expected 'branch-from-file', got '%s'", name)
	}

	// 3. File Path without Branch (derives from filename)
	tmpFile2 := "test_ticket_filename.md"
	content2 := []byte("# Task: Test\n")
	if err := os.WriteFile(tmpFile2, content2, 0644); err != nil {
		return fmt.Errorf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile2)

	name = cli.GetTicketName(tmpFile2)
	// Base name is test_ticket_filename.md -> test_ticket_filename
	if name != "test_ticket_filename" {
		return fmt.Errorf("Expected 'test_ticket_filename', got '%s'", name)
	}

	// 4. File Path (non-existent)
	name = cli.GetTicketName("non_existent_file.md")
	if name != "non_existent_file.md" {
		return fmt.Errorf("Expected 'non_existent_file.md', got '%s'", name)
	}
	
	logger.LogInfo("GetTicketName tests passed")
	return nil
}

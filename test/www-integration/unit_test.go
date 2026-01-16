package www_integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	dialtone "dialtone/cli/src"
)

// Unit tests: Simple tests that run locally without IO operations
// These tests should be fast and test individual functions/components

func TestUnit_Example(t *testing.T) {
	dialtone.LogInfo("Running unit test for www-integration")

	// Get project root
	cwd, _ := os.Getwd()
	projectRoot := filepath.Join(cwd, "..", "..")

	// Read dev.go to ensure keywords are there
	devGoPath := filepath.Join(projectRoot, "src", "dev.go")
	content, err := os.ReadFile(devGoPath)
	if err != nil {
		t.Fatalf("Failed to read dev.go: %v", err)
	}

	contents := string(content)
	if !strings.Contains(contents, "case \"www\":") {
		t.Error("www command case not found in dev.go")
	}

	if !strings.Contains(contents, "dialtone-dev www publish") {
		t.Error("www publish help text not found in dev.go")
	}
}

func TestUnit_Validation(t *testing.T) {
	// Test input validation, data parsing, etc.
	dialtone.LogInfo("Testing validation for www-integration")

	// TODO: Add validation tests
	t.Log("not yet implemented")
}

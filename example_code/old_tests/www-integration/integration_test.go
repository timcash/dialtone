package www_integration

import (
	"os"
	"path/filepath"
	"testing"

	dialtone "dialtone/cli/src"
)

// Integration tests: Test 2+ components together using test_data/
// These tests may use files, but should not require network or external services

func TestIntegration_Example(t *testing.T) {
	dialtone.LogInfo("Running integration test for www-integration")

	// Get project root for test data access
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	projectRoot := filepath.Join(cwd, "..", "..")

	wwwDir := filepath.Join(projectRoot, "dialtone-earth")
	if _, err := os.Stat(wwwDir); os.IsNotExist(err) {
		t.Fatalf("dialtone-earth directory does not exist at %s", wwwDir)
	}

	// Check for a known file in the integrated repo, e.g. package.json or README.md
	if _, err := os.Stat(filepath.Join(wwwDir, "README.md")); os.IsNotExist(err) {
		t.Error("Integrated repository is missing README.md")
	}
}

func TestIntegration_Components(t *testing.T) {
	// Test how multiple components work together
	dialtone.LogInfo("Testing component integration for www-integration")

	// TODO: Add component integration tests
	t.Log("not yet implemented")
}

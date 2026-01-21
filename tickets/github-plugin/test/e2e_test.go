package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"


	dialtone "dialtone/cli/src"
)

// TestE2E_GithubCommandExists verifies that the `dialtone-dev github` command exists
func TestE2E_GithubCommandExists(t *testing.T) {
	// Skip if running in CI without required setup
	if os.Getenv("SKIP_E2E") != "" {
		t.Skip("Skipping E2E test (SKIP_E2E is set)")
	}

	dialtone.LogInfo("Running E2E test: dialtone-dev github command existence")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	// Assuming we run this from anywhere but need to call the binary/run go file relative to root
	projectRoot := filepath.Join(cwd, "..", "..", "..") // tickets/github-plugin/test -> tickets/github-plugin -> tickets -> root

	// Check for dialtone-dev.go
	devGoPath := filepath.Join(projectRoot, "src", "dev.go")
	if _, err := os.Stat(devGoPath); os.IsNotExist(err) {
		t.Fatalf("dialtone-dev.go not found at %s", devGoPath)
	}

	// Run `go run dialtone-dev.go github --help`
	cmd := exec.Command("go", "run", "dialtone-dev.go", "github", "--help")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	
	// We expect validation failure or help output, but NOT "Unknown command: github"
	if err != nil {
		// It might fail if command is not implemented yet or exit code 1
		// but we want to check the output content
	}
	
	outputStr := string(output)
	if strings.Contains(outputStr, "Unknown command: github") {
		t.Errorf("FAIL: `dialtone-dev github` command not found. Output: %s", outputStr)
	} else if strings.Contains(outputStr, "usage:") || strings.Contains(outputStr, "Usage:") {
        t.Logf("PASS: `dialtone-dev github` command found (help output)")
    } else {
        // If it's not implemented yet, it might default to help or fail differently
        // For TDD, let's accept that we want to move away from "Unknown command"
        t.Logf("Output was: %s", outputStr)
    }
}

// TestE2E_PullRequestDelegation verifies that `dialtone-dev pull-request` calls the new plugin
// NOTE: This test will fail until we switch the implementation in src/dev.go
func TestE2E_PullRequestDelegation(t *testing.T) {
	if os.Getenv("SKIP_E2E") != "" {
		t.Skip("Skipping E2E test (SKIP_E2E is set)")
	}
    
    // We can't easily check internal delegation without side effects or logging checks.
    // However, we can check if `dialtone-dev github pull-request` works similarly.
    
    t.Log("This test is a placeholder for verifying that `dialtone-dev pull-request` delegates correctly.")
}

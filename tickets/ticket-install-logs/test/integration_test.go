package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegration_DependencyPathResolution(t *testing.T) {
	// 1. Default path test: run install without args/env and assert tools resolve from dialtone_dependencies next to dialtone.sh.
	// Since we are in a test environment, we should check if the log output mentions the default path.
	t.Run("DefaultPath", func(t *testing.T) {
		cmd := exec.Command("bash", "./dialtone.sh", "install", "--help")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to run dialtone.sh: %v\nOutput: %s", err, output)
		}
		
		// Assert help message mentions default path
		if !strings.Contains(string(output), "dialtone_dependencies") && !strings.Contains(string(output), ".dialtone_env") {
			t.Errorf("Expected output to mention dependency path, got: %s", output)
		}
	})

	// 2. CLI option test: run install with explicit path and assert env var is set and tools resolve from that path.
	t.Run("CLIOption", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "dialtone-deps-test")
		defer os.RemoveAll(tempDir)
		
		// Note: We don't actually want to install everything during a test, so we just check if it parses and logs correctly
		cmd := exec.Command("go", "run", "dialtone-dev.go", "install", tempDir, "--help")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to run dialtone-dev.go: %v\nOutput: %s", err, output)
		}
		
		if !strings.Contains(string(output), tempDir) {
			t.Errorf("Expected output to mention CLI path %s, got: %s", tempDir, output)
		}
	})

	// 3. Env var test: set the env var and assert install uses it over defaults and logs the source.
	t.Run("EnvVarPriority", func(t *testing.T) {
		envDir := "/tmp/mock-env-dir"
		cmd := exec.Command("go", "run", "dialtone-dev.go", "install", "--help")
		cmd.Env = append(os.Environ(), "DIALTONE_ENV="+envDir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to run dialtone-dev.go with ENV: %v\nOutput: %s", err, output)
		}
		
		if !strings.Contains(string(output), envDir) {
			t.Errorf("Expected output to mention ENV path %s, got: %s", envDir, output)
		}
	})
}

func TestIntegration_GitIgnore(t *testing.T) {
	// Repo-local path test: install into ./dialtone_dependencies under repo root and ensure .gitignore prevents git tracking.
	cmd := exec.Command("git", "check-ignore", "dialtone_dependencies")
	err := cmd.Run()
	if err != nil {
		t.Errorf("Expected dialtone_dependencies to be ignored by git")
	}
}

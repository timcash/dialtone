package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegration_DependencyPathResolution(t *testing.T) {
	root, err := findRoot()
	if err != nil {
		t.Fatalf("Could not find project root: %v", err)
	}
	t.Logf("Detected project root: %s", root)
	dialtoneSh := filepath.Join(root, "dialtone.sh")

	// 1. Default path test: run install without args/env and assert logs the default path.
	t.Run("DefaultPath", func(t *testing.T) {
		cmd := exec.Command(dialtoneSh, "install", "--help")
		cmd.Dir = root
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to run %s: %v\nOutput: %s", dialtoneSh, err, output)
		}
		
		// Assert output mentions dependency path
		if !strings.Contains(string(output), "dialtone_dependencies") && !strings.Contains(string(output), ".dialtone_env") {
			t.Errorf("Expected output to mention dependency path, got: %s", output)
		}
	})

	// 2. CLI option test: run install with explicit path and assert logs that path.
	t.Run("CLIOption", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "dialtone-deps-test")
		defer os.RemoveAll(tempDir)
		
		cmd := exec.Command(dialtoneSh, "install", tempDir, "--help")
		cmd.Dir = root
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to run %s: %v\nOutput: %s", dialtoneSh, err, output)
		}
		
		if !strings.Contains(string(output), tempDir) {
			t.Errorf("Expected output to mention CLI path %s, got: %s", tempDir, output)
		}
	})

	// 3. Env var test: set the env var and assert it's prioritized and logged.
	t.Run("EnvVarPriority", func(t *testing.T) {
		envDir := "/tmp/mock-env-dir"
		cmd := exec.Command(dialtoneSh, "install", "--help")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "DIALTONE_ENV="+envDir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to run %s with ENV: %v\nOutput: %s", dialtoneSh, err, output)
		}
		
		if !strings.Contains(string(output), envDir) {
			t.Errorf("Expected output to mention ENV path %s, got: %s", envDir, output)
		}
	})

	// 4. Clean test: assert --clean removes the directory.
	t.Run("Clean", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "dialtone-clean-test")
		// Create a mock file in it
		mockFile := filepath.Join(tempDir, "mock-dep")
		os.MkdirAll(tempDir, 0755)
		os.WriteFile(mockFile, []byte("test"), 0644)
		
		cmd := exec.Command(dialtoneSh, "install", tempDir, "--clean", "--help")
		cmd.Dir = root
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to run %s --clean: %v\nOutput: %s", dialtoneSh, err, output)
		}
		
		if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
			t.Errorf("Expected directory %s to be deleted by --clean", tempDir)
		}
	})
}

package ticket_install_logs

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegration_DependencyPathResolution(t *testing.T) {
	// Need to be at project root to run dialtone.sh or dialtone-dev.go
	if err := os.Chdir("../.."); err != nil {
		t.Fatalf("Failed to chdir to project root: %v", err)
	}

	// 1. Default path test: run install without args/env and assert logs the default path.
	t.Run("DefaultPath", func(t *testing.T) {
		cmd := exec.Command("bash", "./dialtone.sh", "install", "--help")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to run dialtone.sh: %v\nOutput: %s", err, output)
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
		
		cmd := exec.Command("go", "run", "dialtone-dev.go", "install", tempDir, "--help")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to run dialtone-dev.go: %v\nOutput: %s", err, output)
		}
		
		if !strings.Contains(string(output), tempDir) {
			t.Errorf("Expected output to mention CLI path %s, got: %s", tempDir, output)
		}
	})

	// 3. Env var test: set the env var and assert it's prioritized and logged.
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

	// 4. Clean test: assert --clean removes the directory.
	t.Run("Clean", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "dialtone-clean-test")
		// Create a mock file in it
		mockFile := filepath.Join(tempDir, "mock-dep")
		os.MkdirAll(tempDir, 0755)
		os.WriteFile(mockFile, []byte("test"), 0644)
		
		cmd := exec.Command("go", "run", "dialtone-dev.go", "install", tempDir, "--clean", "--help")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to run dialtone-dev.go --clean: %v\nOutput: %s", err, output)
		}
		
		if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
			t.Errorf("Expected directory %s to be deleted by --clean", tempDir)
		}
	})
}


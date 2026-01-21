package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestE2E_PluginCreate(t *testing.T) {
	// Setup
	pluginName := "test-plugin-e2e"
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get CWD: %v", err)
	}

	// Assuming we are running from tickets/plugin-dev/test or root
	// We need to find the project root. 
	// If running via `dialtone.sh ticket test`, the CWD might be the root or the test dir.
	// But `go test` sets CWD to the package directory.
	// So we need to go up 3 levels: tickets/plugin-dev/test -> ... -> plugin-dev -> tickets -> root
	
	rootDir := filepath.Join(cwd, "..", "..", "..")
	
	// Verify we are at root by checking for dialtone-dev.go
	if _, err := os.Stat(filepath.Join(rootDir, "src", "dev.go")); os.IsNotExist(err) {
		// Fallback: try to find root by looking for src/dev.go
        // Actually, let's just make the user run it from root or assume standard structure
		t.Logf("Warning: Could not find src/dev.go at expected root %s. CWD is %s", rootDir, cwd)
	}

	pluginDir := filepath.Join(rootDir, "src", "plugins", pluginName)
	
	// Cleanup before test (just in case)
	os.RemoveAll(pluginDir)
	defer os.RemoveAll(pluginDir) // Cleanup after test

	// Run dialtone-dev plugin create
	cmd := exec.Command("go", "run", "dialtone-dev.go", "plugin", "create", pluginName)
	cmd.Dir = rootDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run plugin create command: %v\nOutput: %s", err, output)
	}
	t.Logf("Command output: %s", output)

	// Verify Structure
	filesToCheck := []string{
		filepath.Join(pluginDir, "app"),
		filepath.Join(pluginDir, "cli"),
		filepath.Join(pluginDir, "test"),
		filepath.Join(pluginDir, "README.md"),
		filepath.Join(pluginDir, "test", "unit_test.go"),
		filepath.Join(pluginDir, "test", "integration_test.go"),
		filepath.Join(pluginDir, "test", "e2e_test.go"),
	}

	for _, path := range filesToCheck {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file/dir does not exist: %s", path)
		}
	}
}

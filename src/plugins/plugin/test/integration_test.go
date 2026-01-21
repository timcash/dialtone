package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIntegration_PluginCreate(t *testing.T) {
	// Setup
	pluginName := "test-plugin-integration"
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get CWD: %v", err)
	}
	
	// We are in src/plugins/plugin/test
	// Root is ../../../..
	rootDir := filepath.Join(cwd, "..", "..", "..", "..")
	
	pluginDir := filepath.Join(rootDir, "src", "plugins", pluginName)
	os.RemoveAll(pluginDir)
	defer os.RemoveAll(pluginDir)

	// Run command
	// We use the same approach as E2E but strictly checking file creation logic
	// Since we can't easily import the main package's private functions, we run the CLI
	// Ideally we would refactor plugin.go to have a public CreatePlugin function to test directly.
	// For now, testing via CLI execution is a valid integration test.
	
	cmd := exec.Command("go", "run", "dialtone-dev.go", "plugin", "create", pluginName)
	cmd.Dir = rootDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	// Verify
	files := []string{
		"README.md",
		"app",
		"cli",
		"test/unit_test.go",
	}

	for _, f := range files {
		path := filepath.Join(pluginDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Missing expected file: %s", f)
		}
	}
}

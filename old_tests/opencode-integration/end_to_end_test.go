package opencode_integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	dialtone "dialtone/cli/src"
)

// End-to-end tests: Browser and CLI tests on a live system or simulator
// These tests may require network, external services, or user interaction setup

func TestE2E_CLICommand(t *testing.T) {
	dialtone.LogInfo("Running E2E CLI test for opencode-integration")
	
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	projectRoot := filepath.Join(cwd, "..", "..")
	
	// Test dialtone-dev help output for opencode
	cmd := exec.Command("go", "run", "dialtone-dev.go", "help")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run dialtone-dev help: %v\nOutput: %s", err, output)
	}
	
	if !strings.Contains(string(output), "opencode <subcmd>") {
		t.Errorf("dialtone-dev help output missing opencode command")
	}

	// Test dialtone start help output for --opencode flag
	cmd = exec.Command("go", "run", "dialtone.go", "start", "--help")
	cmd.Dir = projectRoot
	output, _ = cmd.CombinedOutput() // --help might exit non-zero
	
	if !strings.Contains(string(output), "-opencode") {
		t.Errorf("dialtone start help output missing --opencode flag")
	}
}

func TestE2E_FullWorkflow(t *testing.T) {
	if os.Getenv("SKIP_E2E") != "" {
		t.Skip("Skipping E2E test (SKIP_E2E is set)")
	}
	
	dialtone.LogInfo("Running full workflow E2E test for opencode-integration")
	
	// TODO: Test complete user workflows
	t.Log("not yet implemented")
}

func TestE2E_BinaryExists(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	projectRoot := filepath.Join(cwd, "..", "..")
	binPath := filepath.Join(projectRoot, "bin", "dialtone")
	
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skip("Binary not built - run 'dialtone build' first")
	}
	
	// Verify binary runs
	cmd := exec.Command(binPath, "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// --help might exit non-zero, check output instead
		if !strings.Contains(string(output), "dialtone") {
			t.Errorf("Binary output doesn't contain 'dialtone': %s", output)
		}
	}
	
	dialtone.LogInfo("Binary exists and runs for opencode-integration tests")
}

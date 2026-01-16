package move_dev_commands

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
	// Skip if running without required setup
	if os.Getenv("SKIP_E2E") != "" {
		t.Skip("Skipping E2E test (SKIP_E2E is set)")
	}
	
	dialtone.LogInfo("Running E2E CLI test for command migration")
	
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	projectRoot := filepath.Join(cwd, "..", "..")

	// 1. Verify dialtone only shows 'start'
	cmdDialtone := exec.Command("go", "run", "dialtone.go", "--help")
	cmdDialtone.Dir = projectRoot
	outDialtone, _ := cmdDialtone.CombinedOutput()
	output := string(outDialtone)

	if !strings.Contains(output, "  start") {
		t.Error("dialtone help missing 'start' command")
	}
	if strings.Contains(output, "  install") || strings.Contains(output, "  build") {
		t.Error("dialtone help still contains dev commands")
	}

	// 2. Verify dialtone-dev shows all commands
	cmdDev := exec.Command("go", "run", "dialtone-dev.go", "--help")
	cmdDev.Dir = projectRoot
	outDev, err := cmdDev.CombinedOutput()
	if err != nil {
		t.Fatalf("dialtone-dev --help failed: %v\n%s", err, outDev)
	}
	outputDev := string(outDev)

	commands := []string{"install", "build", "full-build", "deploy", "clone", "plan", "branch", "test", "pull-request", "issue", "www"}
	for _, cmd := range commands {
		if !strings.Contains(outputDev, "  "+cmd) {
			t.Errorf("dialtone-dev help missing '%s' command", cmd)
		}
	}
}

func TestE2E_FullWorkflow(t *testing.T) {
	if os.Getenv("SKIP_E2E") != "" {
		t.Skip("Skipping E2E test (SKIP_E2E is set)")
	}
	
	dialtone.LogInfo("Running full workflow E2E test for move-dev-commands")
	
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
	
	dialtone.LogInfo("Binary exists and runs for move-dev-commands tests")
}

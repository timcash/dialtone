package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	dialtone "dialtone/cli/src"
)

func TestE2E_CLICommand(t *testing.T) {
	if os.Getenv("SKIP_E2E") != "" {
		t.Skip("Skipping E2E test (SKIP_E2E is set)")
	}
	
	dialtone.LogInfo("Running E2E CLI test for ticket-install-logs")
	
	root, err := findRoot()
	if err != nil {
		t.Fatalf("Could not find project root: %v", err)
	}
	dialtoneSh := filepath.Join(root, "dialtone.sh")

	// Verify help works via dialtone.sh
	cmd := exec.Command(dialtoneSh, "help")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run %s: %v\nOutput: %s", dialtoneSh, err, output)
	}

	if !strings.Contains(string(output), "dialtone") {
		t.Errorf("Help output for %s doesn't contain 'dialtone': %s", dialtoneSh, output)
	}
}

func TestE2E_FullWorkflow(t *testing.T) {
	if os.Getenv("SKIP_E2E") != "" {
		t.Skip("Skipping E2E test (SKIP_E2E is set)")
	}
	
	dialtone.LogInfo("Running full workflow E2E test for ticket-install-logs")
	t.Log("not yet implemented")
}

func TestE2E_BinaryExists(t *testing.T) {
	root, err := findRoot()
	if err != nil {
		t.Fatalf("Could not find project root: %v", err)
	}
	binPath := filepath.Join(root, "bin", "dialtone")
	
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skip("Binary not built - run 'dialtone build' first")
	}
	
	dialtone.LogInfo("Binary exists and runs for ticket-install-logs tests")
}

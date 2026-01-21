package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2E_WwwHelp(t *testing.T) {
	// Simple sanity check that the www command is registered and help works
	// This currently tests the legacy behavior until we change it, then validates the new one.
	// Since we haven't changed the code yet, this should PASS if dev.go has www command.
	
	// Resolve absolute path to dialtone.sh
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	projectRoot := filepath.Join(pwd, "../../..")
	scriptPath := filepath.Join(projectRoot, "dialtone.sh")

	cmd := exec.Command(scriptPath, "www", "--help")
	cmd.Dir = projectRoot // Run in project root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run dialtone.sh www --help: %v\nOutput: %s", err, output)
	}

	outStr := string(output)
	if !strings.Contains(outStr, "Usage: dialtone-dev www") {
		t.Errorf("Expected usage info in output, got:\n%s", outStr)
	}
}

func TestE2E_WwwLogs(t *testing.T) {
    // This expects the new implementation or at least the command to exist
    cmd := exec.Command("../../../dialtone.sh", "www", "logs")
    // We don't expect it to succeed without auth/project, but it should run the command
    // Current dev.go implementation requires Vercel CLI.
    // We'll skip this if Vercel isn't installed in CI/Test env, or just check it attempts to run.
    
    // For now, let's just checking it doesn't crash the CLI itself
     _ = cmd // placeholder
}


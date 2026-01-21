package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegration_JaxDemo(t *testing.T) {
	// Find the app directory relative to this test file
	// src/plugins/jax-demo/test/integration_test.go -> src/plugins/jax-demo/app
	appDir, _ := filepath.Abs("../app")

	t.Logf("Running pixi run start in %s", appDir)

	// Try to find pixi binary
	pixiBin, err := exec.LookPath("pixi")
	if err != nil {
		// Try default installation path
		home, _ := os.UserHomeDir()
		pixiBin = filepath.Join(home, ".pixi", "bin", "pixi")
		if _, err := os.Stat(pixiBin); err != nil {
			t.Fatalf("pixi binary not found in PATH or in %s. Please install it first.", pixiBin)
		}
	}

	// Run pixi run start
	cmd := exec.Command(pixiBin, "run", "start")
	cmd.Dir = appDir
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		t.Fatalf("pixi run start failed: %v\nOutput: %s", err, string(output))
	}

	outputStr := string(output)
	t.Logf("Output: %s", outputStr)

	// Verify expected output
	if !strings.Contains(outputStr, "Calculated ") || !strings.Contains(outputStr, " distances") {
		t.Errorf("Output did not contain expected success message. Got: %s", outputStr)
	}

	if !strings.Contains(outputStr, "Warm start took:") {
		t.Errorf("Output did not contain benchmark results. Got: %s", outputStr)
	}

	if !strings.Contains(outputStr, "Geospatial Benchmark") {
		t.Errorf("Output did not contain benchmark table header. Got: %s", outputStr)
	}
}

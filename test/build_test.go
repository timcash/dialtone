package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildTools_Go(t *testing.T) {
	cmd := exec.Command("go", "version")
	if err := cmd.Run(); err != nil {
		t.Errorf("Go is not installed or not in PATH: %v", err)
	}
}

func TestBuildTools_NPM(t *testing.T) {
	cmd := exec.Command("npm", "--version")
	if err := cmd.Run(); err != nil {
		t.Errorf("NPM is not installed or not in PATH: %v", err)
	}
}

func TestWebUI_BuildOutput(t *testing.T) {
	// Check if the web build directory exists (this is what gets embedded)
	info, err := os.Stat("../src/web_build")
	if err != nil {
		if os.IsNotExist(err) {
			t.Log("Warning: src/web_build does not exist.")
			return
		}
		t.Fatalf("Failed to stat src/web_build: %v", err)
	}

	if !info.IsDir() {
		t.Error("src/web_build should be a directory")
	}
}

func TestBuildScript_PowerShell(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping PowerShell test in CI")
	}

	// Move to root for running build script
	err := os.Chdir("..")
	if err != nil {
		t.Fatalf("Failed to change dir to root: %v", err)
	}
	defer os.Chdir("test")

	cmd := exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-File", "build.ps1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build.ps1 failed: %v\nOutput: %s", err, string(output))
	}

	// Verify binary exists
	binaryPath := filepath.Join("bin", "dialtone.exe")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Errorf("Binary %s not created by build.ps1", binaryPath)
	}
}

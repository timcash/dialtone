package main

import (
	"os"
	"os/exec"
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
	info, err := os.Stat("web_build")
	if err != nil {
		if os.IsNotExist(err) {
			t.Log("Warning: src/web_build does not exist. Run './build_and_deploy.ps1' to build and prepare assets.")
			return
		}
		t.Fatalf("Failed to stat src/web_build: %v", err)
	}

	if !info.IsDir() {
		t.Error("src/web_build should be a directory")
	}
}

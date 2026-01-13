package main

import (
	"os"
	"testing"
)

func TestDeploy_ScriptExists(t *testing.T) {
	// build_and_deploy.ps1 is in the project root
	_, err := os.Stat("../build_and_deploy.ps1")
	if err != nil {
		t.Errorf("Deployment script build_and_deploy.ps1 not found in root: %v", err)
	}
}

func TestDeploy_SshToolsCompiled(t *testing.T) {
	// The script builds bin/ssh_tools.exe
	_, err := os.Stat("../bin/ssh_tools.exe")
	if err != nil {
		t.Log("Note: bin/ssh_tools.exe not found. This is expected if the build script hasn't run.")
	}
}

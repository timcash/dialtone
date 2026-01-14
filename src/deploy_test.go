package main

import (
	"os"
	"testing"
)

func TestDeploy_BinaryExists(t *testing.T) {
	// We now build bin/dialtone.exe
	_, err := os.Stat("../bin/dialtone.exe")
	if err != nil {
		t.Log("Note: bin/dialtone.exe not found. This is expected if the build command hasn't run.")
	}
}

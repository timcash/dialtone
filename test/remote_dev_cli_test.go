package test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestCLICommands verifies that the new CLI commands are registered and print help
func TestCLICommands(t *testing.T) {
	// Ensure the binary is built (we built it in previous steps, but good to be safe or assume it's there)
	// We assume "bin/dialtone.exe" exists from the "go build" command run earlier.
	
	binaryPath := "../bin/dialtone.exe"

	tests := []struct {
		name    string
		command string
		want    []string
	}{
		{
			name:    "InstallDepsHelp",
			command: "install-deps",
			want:    []string{"Usage of install-deps:", "-host", "-user", "-pass"},
		},
		{
			name:    "SyncCodeHelp",
			command: "sync-code",
			want:    []string{"Usage of sync-code:", "-host", "-user", "-pass"},
		},
		{
			name:    "RemoteBuildHelp",
			command: "remote-build",
			want:    []string{"Usage of remote-build:", "-host", "-user", "-pass"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tt.command, "-help")
			output, err := cmd.CombinedOutput()
			// Exit code might be 0 or 2 depending on flag parsing for -help, usually 0 or 2.
			// flag.ExitOnError usually exits with 2 if parsing fails, but -help is special.
			// Actually flag.ExitOnError with -help prints usage and exits 0 or 2. 
			// We mainly care that output contains usage.
			
			// If binary doesn't exist, skip or fail
			if err != nil && !strings.Contains(string(output), "Usage") {
				// Verify if it's just exit code 2 (flag help default)
				if exitErr, ok := err.(*exec.ExitError); ok {
					if exitErr.ExitCode() != 2 && exitErr.ExitCode() != 0 {
						t.Errorf("Command failed with code %d: %v", exitErr.ExitCode(), err)
					}
				} else {
					t.Fatalf("Failed to run command: %v", err)
				}
			}

			outStr := string(output)
			for _, w := range tt.want {
				if !strings.Contains(outStr, w) {
					t.Errorf("Output missing expected string %q. Got:\n%s", w, outStr)
				}
			}
		})
	}
}

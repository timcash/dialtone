//go:build linux

package test

import (
	dialtone "dialtone/cli/src"
	"os"
	"os/exec"
	"testing"
)

func TestWSLCameraSupport(t *testing.T) {
	dialtone.LogInfo("=== WSL Camera Support Verification ===")

	// 1. Check for V4L2 headers
	headerPath := "/usr/include/linux/videodev2.h"
	if _, err := os.Stat(headerPath); os.IsNotExist(err) {
		t.Errorf("V4L2 headers not found at %s. Please run 'dialtone install-deps --linux-wsl'", headerPath)
	} else {
		dialtone.LogInfo("[+] V4L2 headers found at %s", headerPath)
	}

	// 2. Attempt to compile the project with CGO enabled
	dialtone.LogInfo("Attempting to compile Dialtone with CGO_ENABLED=1...")
	
	// We use go build -o /dev/null . to check if it compiles
	cmd := exec.Command("go", "build", "-o", "/dev/null", ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("Compilation failed: %v\nOutput: %s", err, string(output))
	} else {
		dialtone.LogInfo("[+] Compilation successful with CGO_ENABLED=1")
	}
}

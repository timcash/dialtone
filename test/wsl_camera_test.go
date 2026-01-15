//go:build linux

package test

import (
	dialtone "dialtone/cli/src"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWSLCameraSupport(t *testing.T) {
	dialtone.LogInfo("=== WSL Camera Support Verification ===")

	// 1. Check for V4L2 headers
	headerPath := "/usr/include/linux/videodev2.h"
	homeDir, _ := os.UserHomeDir()
	localHeaderPath := filepath.Join(homeDir, ".dialtone_env", "usr", "include", "linux", "videodev2.h")
	
	if _, err := os.Stat(headerPath); os.IsNotExist(err) {
		if _, err := os.Stat(localHeaderPath); os.IsNotExist(err) {
			t.Errorf("V4L2 headers not found at %s or %s. Please run 'dialtone install-deps --linux-wsl'", headerPath, localHeaderPath)
		} else {
			dialtone.LogInfo("[+] Found V4L2 headers in local environment: %s", localHeaderPath)
		}
	} else {
		dialtone.LogInfo("[+] V4L2 headers found at %s", headerPath)
	}

	// 2. Attempt to compile the project with CGO enabled
	dialtone.LogInfo("Attempting to compile Dialtone with CGO_ENABLED=1...")
	
	// We use go build -o /dev/null . to check if it compiles
	cmd := exec.Command("go", "build", "-o", "/dev/null", ".")
	cmd.Dir = ".." // Run from root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")

	// Ensure we use the local compiler and headers if available
	depsDir := filepath.Join(homeDir, ".dialtone_env")
	if _, err := os.Stat(depsDir); err == nil {
		zigPath := filepath.Join(depsDir, "zig", "zig")
		if _, err := os.Stat(zigPath); err == nil {
			cmd.Env = append(cmd.Env, fmt.Sprintf("CC=%s cc -target x86_64-linux-gnu", zigPath))
		}
		includePath := filepath.Join(depsDir, "usr", "include")
		multiarchInclude := filepath.Join(includePath, "x86_64-linux-gnu")
		cmd.Env = append(cmd.Env, fmt.Sprintf("CGO_CFLAGS=-I%s -I%s", includePath, multiarchInclude))
		
		// Ensure go is in PATH
		goBin := filepath.Join(depsDir, "go", "bin")
		cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s:%s", goBin, os.Getenv("PATH")))
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Compilation failed: %v\nOutput: %s", err, string(output))
	} else {
		dialtone.LogInfo("[+] Compilation successful with CGO_ENABLED=1")
	}
}

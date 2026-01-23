package test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func getDepsDir(t *testing.T) string {
	// Find project root first
	root := ""
	cwd, _ := os.Getwd()
	t.Logf("getDepsDir: Starting search for project root from CWD=%s", cwd)
	for {
		shPath := filepath.Join(cwd, "dialtone.sh")
		if _, err := os.Stat(shPath); err == nil {
			t.Logf("getDepsDir: Found project root at %s", cwd)
			root = cwd
			break
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			t.Logf("getDepsDir: Reached root, stopping search")
			break
		}
		cwd = parent
	}

	env := os.Getenv("DIALTONE_ENV")
	if env != "" {
		t.Logf("getDepsDir: DIALTONE_ENV is set to %s", env)
		if !filepath.IsAbs(env) && root != "" {
			absPath := filepath.Join(root, env)
			t.Logf("getDepsDir: Resolved relative DIALTONE_ENV to %s", absPath)
			return absPath
		}
		absPath, _ := filepath.Abs(env)
		return absPath
	}

	if root != "" {
		localPath := filepath.Join(root, "dialtone_dependencies")
		if _, err := os.Stat(localPath); err == nil {
			t.Logf("getDepsDir: Found dialtone_dependencies at %s", localPath)
			return localPath
		}
	}

	home, _ := os.UserHomeDir()
	absPath := filepath.Join(home, ".dialtone_env")
	t.Logf("getDepsDir: Falling back to %s", absPath)
	return absPath
}

func TestWSLDependencies(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping WSL dependency check on non-linux platform")
	}

	depsDir := getDepsDir(t)
	t.Logf("Final dependencies directory: %s", depsDir)

	bins := []string{
		filepath.Join(depsDir, "go", "bin", "go"),
		filepath.Join(depsDir, "node", "bin", "node"),
		filepath.Join(depsDir, "gh", "bin", "gh"),
		filepath.Join(depsDir, "pixi", "pixi"),
		filepath.Join(depsDir, "zig", "zig"),
		filepath.Join(depsDir, "gcc-aarch64", "bin", "aarch64-none-linux-gnu-gcc"),
		filepath.Join(depsDir, "gcc-armhf", "bin", "arm-none-linux-gnueabihf-gcc"),
	}

	for _, bin := range bins {
		t.Logf("Checking for binary: %s", bin)
		if _, err := os.Stat(bin); err != nil {
			t.Errorf("Missing required binary: %s", bin)
		} else {
			t.Logf("Binary FOUND: %s", bin)
		}
	}
}

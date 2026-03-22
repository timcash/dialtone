package shellv1_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestShellV1Layout(t *testing.T) {
	root := currentDir(t)
	for _, rel := range []string{
		"README.md",
		"mod.json",
		"nix.packages",
		filepath.Join("cli", "main.go"),
		filepath.Join("cli", "main_test.go"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("expected %s in shell/v1: %v", rel, err)
		}
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	if !strings.Contains(string(readme), "## Test Result") {
		t.Fatalf("expected shell/v1 README to contain a Test Result section")
	}
}

func currentDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}

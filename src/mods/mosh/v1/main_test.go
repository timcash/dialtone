package moshv1_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMoshV1Layout(t *testing.T) {
	root := currentDir(t)
	for _, rel := range []string{
		"README.md",
		"nix.packages",
		filepath.Join("cli", "main.go"),
		filepath.Join("cli", "main_test.go"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("expected %s in mosh/v1: %v", rel, err)
		}
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

package ghosttyv1_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGhosttyV1Layout(t *testing.T) {
	root := currentDir(t)
	for _, rel := range []string{
		"README.md",
		"mod.json",
		filepath.Join("cli", "main.go"),
		filepath.Join("cli", "main_test.go"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("expected %s in ghostty/v1: %v", rel, err)
		}
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	readmeText := string(readme)
	if !strings.Contains(readmeText, "## Quick Start") {
		t.Fatalf("expected ghostty/v1 README to contain Quick Start")
	}
	if !strings.Contains(readmeText, "## DIALTONE>") {
		t.Fatalf("expected ghostty/v1 README to contain DIALTONE>")
	}
	if !strings.Contains(readmeText, "## Dependencies") {
		t.Fatalf("expected ghostty/v1 README to contain Dependencies")
	}
	if !strings.Contains(readmeText, "## Test Results") {
		t.Fatalf("expected ghostty/v1 README to contain Test Results")
	}
	if first := strings.Index(readmeText, "## Test Results"); first == -1 || strings.LastIndex(readmeText, "## Test Results") != first {
		t.Fatalf("expected exactly one Test Results section")
	}
	if strings.LastIndex(readmeText, "\n## ") > strings.Index(readmeText, "## Test Results") {
		t.Fatalf("expected Test Results to be the last README section")
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

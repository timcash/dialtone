package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Run01Preflight(_ *testCtx) (string, error) {
	repoRoot, err := os.Getwd()
	if err != nil {
		return "", err
	}

	commands := [][]string{
		{"logs", "fmt", "src_v1"},
		{"logs", "vet", "src_v1"},
		{"logs", "go-build", "src_v1"},
		{"logs", "install", "src_v1"},
		{"logs", "lint", "src_v1"},
		{"logs", "format", "src_v1"},
		{"logs", "build", "src_v1"},
	}

	for _, args := range commands {
		cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), args...)
		cmd.Dir = repoRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", err
		}
	}

	return "Ran preflight pipeline (fmt, vet, go-build, install, lint, format, build) for logs src_v1.", nil
}

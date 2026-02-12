package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Run04UILint() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "template", "lint", "src_v3")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Run06UIBuild() error {
	repoRoot, err := testRepoRoot()
	if err != nil {
		return err
	}

	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "cloudflare", "src_v1", "build")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

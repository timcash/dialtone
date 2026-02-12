package src_v3

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Build() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	uiDir := filepath.Join(cwd, "src", "plugins", "template", "src_v3", "ui")

	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "build")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

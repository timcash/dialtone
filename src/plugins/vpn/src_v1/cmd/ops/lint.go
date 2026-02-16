package ops

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Lint() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	uiDir := filepath.Join(cwd, "src", "plugins", "vpn", "src_v1", "ui")

	uiLint := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "lint")
	uiLint.Dir = cwd
	uiLint.Stdout = os.Stdout
	uiLint.Stderr = os.Stderr
	return uiLint.Run()
}

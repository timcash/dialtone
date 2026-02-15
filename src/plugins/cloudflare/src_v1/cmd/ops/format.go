package ops

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Format() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	uiDir := filepath.Join(cwd, "src", "plugins", "cloudflare", "src_v1", "ui")

	uiFmt := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "format")
	uiFmt.Dir = cwd
	uiFmt.Stdout = os.Stdout
	uiFmt.Stderr = os.Stderr
	return uiFmt.Run()
}

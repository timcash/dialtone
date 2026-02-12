package src_v3

import (
	"os"
	"os/exec"
	"path/filepath"
)

func GoBuild() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "build", "./src/plugins/template/src_v3/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

package ops

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Fmt() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "fmt", "./src/plugins/vpn/src_v1/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

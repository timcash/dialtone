package ops

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Serve() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "run", "src/plugins/robot/src_v1/cmd/main.go")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

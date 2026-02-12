package src_v3

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

func UIRun(port int) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if port == 0 {
		port = 3000
	}
	uiDir := filepath.Join(cwd, "src", "plugins", "template", "src_v3", "ui")

	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(port))
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

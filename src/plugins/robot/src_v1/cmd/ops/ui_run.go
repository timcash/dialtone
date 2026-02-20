package ops

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
	repoRoot := cwd
	if filepath.Base(cwd) == "src" {
		repoRoot = filepath.Dir(cwd)
	}

	if port == 0 {
		port = 3000
	}
	uiDir := filepath.Join(repoRoot, "src", "plugins", "robot", "src_v1", "ui")

	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(port))
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

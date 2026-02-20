package ops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"dialtone/dev/plugins/go/src_v1/go"
)

func Build(flags ...string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	repoRoot := cwd
	if filepath.Base(cwd) == "src" {
		repoRoot = filepath.Dir(cwd)
	}
	uiDir := filepath.Join(repoRoot, "src", "plugins", "robot", "src_v1", "ui")

	fmt.Printf(">> [Robot] Building UI: src_v1\n")
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "build")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	robotBinDir := filepath.Join("src", "plugins", "robot", "bin")
	fmt.Printf(">> [Robot] Building Dialtone Binary into %s\n", robotBinDir)

	args := []string{"build", "--output-dir", robotBinDir, "--skip-web", "--skip-www"}
	args = append(args, flags...)
	return go_plugin.RunGo(args...)
}

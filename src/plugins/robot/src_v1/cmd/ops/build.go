package ops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"dialtone/dev/core/build"
)

func Build(flags ...string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	uiDir := filepath.Join(cwd, "src", "plugins", "robot", "src_v1", "ui")

	fmt.Printf(">> [Robot] Building UI: src_v1\n")
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "build")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	robotBinDir := filepath.Join("src", "plugins", "robot", "bin")
	fmt.Printf(">> [Robot] Building Dialtone Binary into %s\n", robotBinDir)

	args := []string{"--output-dir", robotBinDir, "--skip-web", "--skip-www"}
	args = append(args, flags...)
	build.RunBuild(args)
	return nil
}

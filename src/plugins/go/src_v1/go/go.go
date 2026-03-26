package go_plugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

func RunGo(args ...string) error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	goBin := os.Getenv("DIALTONE_GO_BIN")
	if goBin == "" {
		dialtoneEnv := os.Getenv("DIALTONE_ENV")
		if dialtoneEnv == "" {
			dialtoneEnv = configv1.DefaultDialtoneEnv()
		}
		goBin = configv1.ManagedGoBinPath(dialtoneEnv)
	}

	cmd := exec.Command(goBin, args...)
	cmd.Dir = filepath.Join(repoRoot, "src")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if configv1.HasDialtoneScript(cwd) {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", fmt.Errorf("repo root not found from %s", cwd)
		}
		cwd = parent
	}
}

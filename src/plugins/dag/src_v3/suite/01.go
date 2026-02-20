package suite

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Run01Preflight(_ *testCtx) (string, error) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return "", err
	}

	commands := [][]string{
		{"dag", "fmt", "src_v3"},
		{"dag", "vet", "src_v3"},
		{"dag", "go-build", "src_v3"},
		{"dag", "install", "src_v3"},
		{"dag", "lint", "src_v3"},
		{"dag", "format", "src_v3"},
		{"dag", "build", "src_v3"},
	}

	for _, args := range commands {
		cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), args...)
		cmd.Dir = repoRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", err
		}
	}

	return "Ran preflight pipeline (`fmt`, `vet`, `go-build`, `install`, `lint`, `format`, `build`) to verify toolchain and UI build health before browser steps.", nil
}

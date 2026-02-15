package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

func RunLint(versionDir string) error {
	fmt.Printf(">> [DAG] Lint: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	uiDir := filepath.Join(cwd, "src", "plugins", "dag", versionDir, "ui")
	cmd := runBun(cwd, uiDir, "run", "lint")
	return cmd.Run()
}

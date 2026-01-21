package test

import (
	"os"
	"path/filepath"
)

func findRoot() (string, error) {
	cwd, _ := os.Getwd()
	root := cwd
	for {
		if _, err := os.Stat(filepath.Join(root, "dialtone.sh")); err == nil {
			return root, nil
		}
		parent := filepath.Dir(root)
		if parent == root {
			return "", os.ErrNotExist
		}
		root = parent
	}
}

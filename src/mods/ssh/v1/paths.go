package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func locateRepoRoot() (string, error) {
	if envRoot := os.Getenv("DIALTONE_REPO_ROOT"); envRoot != "" {
		candidate := filepath.Clean(envRoot)
		if _, err := os.Stat(filepath.Join(candidate, "dialtone_mod")); err == nil {
			return candidate, nil
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	cwd = filepath.Clean(cwd)
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone_mod")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}
	return "", fmt.Errorf("unable to locate repository root from %s", cwd)
}


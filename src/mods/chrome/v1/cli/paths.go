package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func locateRepoRoot() (string, error) {
	if envRoot := os.Getenv("DIALTONE_REPO_ROOT"); envRoot != "" {
		candidate := filepath.Clean(envRoot)
		if isRepoRoot(candidate) {
			return candidate, nil
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	cwd = filepath.Clean(cwd)
	for {
		if isRepoRoot(cwd) {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}
	return "", fmt.Errorf("unable to locate repo root from %s", cwd)
}

func isRepoRoot(candidate string) bool {
	if _, err := os.Stat(filepath.Join(candidate, "dialtone_mod")); err != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(candidate, "src", "go.mod"))
	return err == nil
}

func locateModRoot(repoRoot string) (string, error) {
	if envSrc := os.Getenv("DIALTONE_SRC_ROOT"); envSrc != "" {
		candidate := filepath.Join(envSrc, "mods", "chrome", "v1")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	if repoRoot != "" {
		candidate := filepath.Join(repoRoot, "src", "mods", "chrome", "v1")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	cwd = filepath.Clean(cwd)
	for {
		candidate := filepath.Join(cwd, "src", "mods", "chrome", "v1")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}
	return "", fmt.Errorf("unable to locate chrome v1 root")
}

func locateCliRoot(repoRoot string) (string, error) {
	modRoot, err := locateModRoot(repoRoot)
	if err != nil {
		return "", err
	}
	cliRoot := filepath.Join(modRoot, "cli")
	if _, err := os.Stat(cliRoot); err != nil {
		return "", fmt.Errorf("cli root missing: %s", cliRoot)
	}
	return cliRoot, nil
}

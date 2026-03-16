package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func locateRepoRoot() (string, error) {
	if envRoot := strings.TrimSpace(os.Getenv("DIALTONE_REPO_ROOT")); envRoot != "" {
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

func locateModRoot(repoRoot string) (string, error) {
	if envSrc := strings.TrimSpace(os.Getenv("DIALTONE_SRC_ROOT")); envSrc != "" {
		candidate := filepath.Join(envSrc, "mods", "repl", "v1")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	if strings.TrimSpace(repoRoot) != "" {
		candidate := filepath.Join(repoRoot, "src", "mods", "repl", "v1")
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
		candidate := filepath.Join(cwd, "src", "mods", "repl", "v1")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}
	return "", fmt.Errorf("unable to locate repl v1 root")
}

func locateSrcRoot(repoRoot string) (string, error) {
	root := strings.TrimSpace(repoRoot)
	if root == "" {
		var err error
		root, err = locateRepoRoot()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(root, "src"), nil
}

func isRepoRoot(candidate string) bool {
	if _, err := os.Stat(filepath.Join(candidate, "dialtone_mod")); err != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(candidate, "src", "go.mod"))
	return err == nil
}

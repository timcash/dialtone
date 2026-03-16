package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func locateRepoRoot(repoRoot string) (string, error) {
	if strings.TrimSpace(repoRoot) != "" && isRepoRoot(repoRoot) {
		return filepath.Clean(repoRoot), nil
	}
	if envRoot := strings.TrimSpace(os.Getenv("DIALTONE_REPO_ROOT")); envRoot != "" && isRepoRoot(envRoot) {
		return filepath.Clean(envRoot), nil
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

	root, err := locateRepoRoot(repoRoot)
	if err == nil {
		candidate := filepath.Join(root, "src", "mods", "repl", "v1")
		if _, statErr := os.Stat(candidate); statErr == nil {
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

func isRepoRoot(candidate string) bool {
	if _, err := os.Stat(filepath.Join(candidate, "dialtone_mod")); err != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(candidate, "src", "go.mod"))
	return err == nil
}

func defaultPromptName() string {
	for _, key := range []string{"DIALTONE_USER", "USER", "USERNAME"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return "local"
}

func newSessionID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "session"
	}
	return hex.EncodeToString(buf[:])
}

func filepathJoin(parts ...string) string {
	return filepath.Join(parts...)
}

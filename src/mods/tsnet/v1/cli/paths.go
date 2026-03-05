package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func locateRepoRoot() (string, error) {
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

func resolveFilePath(repoRoot, value, fallback string) (string, string) {
	path := value
	if path == "" {
		path = fallback
	}
	if filepath.IsAbs(path) {
		return path, path
	}
	if path == "" {
		return "", fallback
	}
	return filepath.Join(repoRoot, path), path
}

func sanitizeHost(host string) string {
	host = strings.TrimSpace(host)
	for i := 0; i < len(host); i++ {
		c := host[i]
		if c >= 'A' && c <= 'Z' {
			c = c + ('a' - 'A')
		}
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-' {
			continue
		}
		host = strings.ReplaceAll(host, string(c), "")
	}
	if host == "" {
		host = "dialtone"
	}
	return host
}

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	tsnetv1 "dialtone/dev/plugins/tsnet/src_v1/go"
)

func Run(args []string) error {
	if len(args) == 0 {
		tsnetv1.PrintUsage()
		return nil
	}

	command := args[0]
	rest := args[1:]

	switch command {
	case "help", "-h", "--help":
		tsnetv1.PrintUsage()
		return nil
	case "test":
		return runTests(rest)
	default:
		return tsnetv1.Run(args)
	}
}

func runTests(args []string) error {
	version := getLatestVersionDir()
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		version = args[0]
	}
	if version != "src_v1" {
		return fmt.Errorf("unsupported version %s", version)
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	cmd := exec.Command("go", "run", "./plugins/tsnet/src_v1/test/cmd/main.go")
	cmd.Dir = filepath.Join(repoRoot, "src")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", logs.Errorf("repo root not found")
		}
		cwd = parent
	}
}

func getLatestVersionDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "src_v1"
	}
	pluginDir := filepath.Join(cwd, "src", "plugins", "tsnet")
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return "src_v1"
	}
	maxVer := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "src_v") {
			continue
		}
		version, err := strconv.Atoi(strings.TrimPrefix(name, "src_v"))
		if err != nil {
			continue
		}
		if version > maxVer {
			maxVer = version
		}
	}
	if maxVer == 0 {
		return "src_v1"
	}
	return fmt.Sprintf("src_v%d", maxVer)
}

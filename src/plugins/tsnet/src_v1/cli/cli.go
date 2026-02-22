package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	tsnetv1 "dialtone/dev/plugins/tsnet/src_v1/go"
)

func Run(args []string) error {
	if len(args) == 0 {
		tsnetv1.PrintUsage()
		return nil
	}

	if isHelpArg(args[0]) {
		tsnetv1.PrintUsage()
		return nil
	}

	version := strings.TrimSpace(args[0])
	if !strings.HasPrefix(version, "src_v") {
		return fmt.Errorf("expected version as first tsnet argument (for example: ./dialtone.sh tsnet src_v1 <command>)")
	}
	if version != "src_v1" {
		return fmt.Errorf("unsupported version %s", version)
	}
	if len(args) < 2 {
		return fmt.Errorf("missing command (usage: ./dialtone.sh tsnet %s <command> [args])", version)
	}

	command := args[1]
	rest := args[2:]
	switch command {
	case "help", "-h", "--help":
		tsnetv1.PrintUsage()
		return nil
	case "test":
		return runTests(version)
	default:
		return tsnetv1.Run(append([]string{command}, rest...))
	}
}

func runTests(version string) error {
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

func isHelpArg(s string) bool {
	switch strings.TrimSpace(s) {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
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

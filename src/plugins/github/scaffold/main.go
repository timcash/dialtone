package main

import (
	"os"
	"os/exec"
	"path/filepath"

	githubv1 "dialtone/dev/plugins/github/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	if len(os.Args) < 2 {
		githubv1.PrintUsage()
		return
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "test":
		runTests(args)
	case "help", "-h", "--help":
		githubv1.PrintUsage()
	default:
		if err := githubv1.Run(append([]string{cmd}, args...)); err != nil {
			logs.Error("github command failed: %v", err)
			os.Exit(1)
		}
	}
}

func runTests(args []string) {
	version := "src_v1"
	if len(args) > 0 && args[0] != "" {
		version = args[0]
	}
	if version != "src_v1" {
		logs.Error("unsupported version %s", version)
		os.Exit(1)
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		logs.Error("%v", err)
		os.Exit(1)
	}

	cmd := exec.Command("go", "run", "./plugins/github/src_v1/test/cmd/main.go")
	cmd.Dir = filepath.Join(repoRoot, "src")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
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

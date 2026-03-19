package main

import (
	"os"
	"os/exec"
	"strings"
)

func nixDevelopCommand(repoRoot string, command ...string) *exec.Cmd {
	if os.Getenv("DIALTONE_NIX_ACTIVE") == "1" {
		argv := append([]string{}, command...)
		if len(argv) > 0 && argv[0] == "go" {
			if goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN")); goBin != "" {
				argv[0] = goBin
			}
		}
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Env = append(
			os.Environ(),
			"DIALTONE_REPO_ROOT="+repoRoot,
			"DIALTONE_NIX_SHELL=repl-v1",
		)
		return cmd
	}
	args := []string{}
	if strings.TrimSpace(os.Getenv("DIALTONE_NIX_OFFLINE")) == "1" {
		args = append(args, "--offline")
	}
	args = append(args,
		"--extra-experimental-features",
		"nix-command flakes",
		"develop",
		"path:"+repoRoot+"#repl-v1",
		"--command",
	)
	args = append(args, command...)
	cmd := exec.Command("nix", args...)
	cmd.Dir = repoRoot
	cmd.Env = append(
		os.Environ(),
		"DIALTONE_NIX_ACTIVE=1",
		"DIALTONE_NIX_OFFLINE="+strings.TrimSpace(os.Getenv("DIALTONE_NIX_OFFLINE")),
		"DIALTONE_NIX_SHELL=repl-v1",
		"DIALTONE_REPO_ROOT="+repoRoot,
	)
	return cmd
}

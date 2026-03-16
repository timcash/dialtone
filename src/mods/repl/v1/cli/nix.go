package main

import (
	"os"
	"os/exec"
)

func nixDevelopCommand(repoRoot string, command ...string) *exec.Cmd {
	if os.Getenv("DIALTONE_NIX_ACTIVE") == "1" {
		cmd := exec.Command(command[0], command[1:]...)
		cmd.Env = append(os.Environ(), "DIALTONE_REPO_ROOT="+repoRoot)
		return cmd
	}
	args := []string{
		"--extra-experimental-features",
		"nix-command flakes",
		"shell",
		"nixpkgs#bashInteractive",
		"nixpkgs#git",
		"nixpkgs#go_1_24",
		"--command",
	}
	args = append(args, command...)
	cmd := exec.Command("nix", args...)
	cmd.Dir = repoRoot
	cmd.Env = append(
		os.Environ(),
		"DIALTONE_NIX_ACTIVE=1",
		"DIALTONE_REPO_ROOT="+repoRoot,
	)
	return cmd
}

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var modV1NixPackages = []string{
	"nixpkgs#bashInteractive",
	"nixpkgs#git",
	"nixpkgs#go",
	"nixpkgs#openssh",
}

func runInstall(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("mod install does not accept arguments")
	}

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}

	if _, err := os.Stat(repoRoot); err != nil {
		return fmt.Errorf("repo root unavailable at %s: %w", repoRoot, err)
	}

	base := []string{
		"--extra-experimental-features",
		"nix-command flakes",
		"shell",
	}
	pkgs, nixExpr := modV1NixPackagesAndSource()
	if nixExpr != "" {
		base = append(base, "-f", nixExpr)
		base = append(base, pkgs...)
	} else {
		base = append(base, modV1NixPackages...)
	}
	base = append(base, "--command", "bash", "-lc", "command -v bash >/dev/null && command -v git >/dev/null && command -v go >/dev/null && command -v ssh >/dev/null")
	cmd := exec.Command("bash", "-lc", "command -v bash >/dev/null && command -v git >/dev/null && command -v go >/dev/null && command -v ssh >/dev/null")
	if os.Getenv("DIALTONE_NIX_ACTIVE") != "1" {
		cmd = exec.Command("nix", base...)
		cmd.Env = append(
			os.Environ(),
			"DIALTONE_REPO_ROOT="+repoRoot,
		)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mod v1 install failed: %w", err)
	}

	fmt.Printf("mod v1 install complete (nix env: %s)\n", strings.Join(modV1NixPackages, ", "))
	return nil
}

func modV1NixPackagesAndSource() ([]string, string) {
	nixpkgsURL := strings.TrimSpace(os.Getenv("NIXPKGS_URL"))
	pkgs := make([]string, len(modV1NixPackages))
	copy(pkgs, modV1NixPackages)
	if nixpkgsURL == "" {
		return pkgs, ""
	}
	for i, p := range pkgs {
		pkgs[i] = strings.TrimPrefix(p, "nixpkgs#")
	}
	return pkgs, nixpkgsURL
}

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var moshInstallPackages = []string{
	"nixpkgs#bashInteractive",
	"nixpkgs#mosh",
	"nixpkgs#openssh",
}

func runInstall(args []string) error {
	cfg := parseInstallArgs(args)

	repoRoot, _ := locateRepoRoot()
	pkgs, nixExpr := moshNixPackagesAndSource(cfg.nixpkgsURL)

	nixBin, err := locateNixBinary()
	if err != nil {
		return err
	}

	if err := runMoshAvailabilityCheck(nixBin, repoRoot, pkgs, nixExpr); err != nil {
		if !cfg.ensure {
			return fmt.Errorf("mosh install check failed: %w", err)
		}

		if err := runMoshProfileInstall(nixBin, pkgs, nixExpr); err != nil {
			return fmt.Errorf("mosh install --ensure failed: %w", err)
		}

		if err := runMoshAvailabilityCheck(nixBin, repoRoot, pkgs, nixExpr); err != nil {
			return fmt.Errorf("mosh install --ensure verification failed: %w", err)
		}
		fmt.Println("mosh install complete: mosh and mosh-server available")
		return nil
	}

	fmt.Println("mosh install check complete")
	return nil
}

func runMoshAvailabilityCheck(nixBin, repoRoot string, pkgs []string, nixExpr string) error {
	base := []string{
		"--extra-experimental-features", "nix-command",
		"--extra-experimental-features", "flakes",
		"shell",
	}
	if nixExpr != "" {
		base = append(base, "-f", nixExpr)
	}
	base = append(base, pkgs...)
	base = append(base, "--command", "bash", "-lc", "command -v mosh >/dev/null && command -v mosh-server >/dev/null")

	cmd := exec.Command(nixBin, base...)
	if strings.TrimSpace(repoRoot) != "" {
		cmd.Dir = repoRoot
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("checking mosh availability failed: %w", err)
	}

	return nil
}

func runMoshProfileInstall(nixBin string, pkgs []string, nixExpr string) error {
	base := []string{
		"--extra-experimental-features", "nix-command",
		"--extra-experimental-features", "flakes",
		"profile",
		"install",
	}
	if nixExpr != "" {
		base = append(base, "-f", nixExpr)
	}

	installPkgs := make([]string, len(pkgs))
	copy(installPkgs, pkgs)
	if nixExpr != "" {
		for i, p := range installPkgs {
			installPkgs[i] = strings.TrimPrefix(p, "nixpkgs#")
		}
	}
	base = append(base, installPkgs...)

	cmd := exec.Command(nixBin, base...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nix profile install failed: %w", err)
	}
	return nil
}

func locateNixBinary() (string, error) {
	if p := strings.TrimSpace(os.Getenv("NIX_BIN")); p != "" {
		return p, nil
	}
	if p, err := exec.LookPath("nix"); err == nil {
		return p, nil
	}

	candidates := []string{
		"/usr/local/bin/nix",
		"/nix/var/nix/profiles/default/bin/nix",
		filepath.Join(os.Getenv("HOME"), ".nix-profile/bin/nix"),
		"/run/current-system/sw/bin/nix",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	matches, err := filepath.Glob("/nix/store/*-nix-*/bin/nix")
	if err == nil && len(matches) > 0 {
		return matches[len(matches)-1], nil
	}

	return "", errors.New("nix executable not found. Set NIX_BIN or install nix")
}

type installOptions struct {
	nixpkgsURL string
	ensure     bool
}

func parseInstallArgs(argv []string) installOptions {
	fs := flag.NewFlagSet("mosh v1 install", flag.ContinueOnError)
	nixpkgs := fs.String("nixpkgs-url", "", "Optional nix expression URL")
	ensure := fs.Bool("ensure", false, "Install mosh packages with nix profile install")
	_ = fs.Parse(argv)
	return installOptions{
		nixpkgsURL: *nixpkgs,
		ensure:     *ensure,
	}
}

func moshNixPackagesAndSource(nixpkgsURL string) ([]string, string) {
	pkgs := make([]string, len(moshInstallPackages))
	copy(pkgs, moshInstallPackages)
	url := strings.TrimSpace(nixpkgsURL)
	if url == "" {
		return pkgs, ""
	}
	return pkgs, url
}

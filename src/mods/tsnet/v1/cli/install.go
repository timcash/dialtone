package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var tsnetInstallPackages = []string{
	"nixpkgs#tailscale",
}

func runInstall(args []string) error {
	opts := parseInstallArgs(args)
	pkgs, nixExpr := tsnetNixPackagesAndSource(opts.nixpkgsURL)

	if err := runNixShellCommand("tailscale", "--version", pkgs, nixExpr); err != nil {
		return fmt.Errorf("tsnet install failed: %w", err)
	}

	if !opts.ensure {
		fmt.Printf("tsnet install check complete: %s\n", joinPackages(pkgs))
		return nil
	}

	if err := runNixProfileInstall(pkgs, nixExpr); err != nil {
		return fmt.Errorf("tsnet install --ensure failed: %w", err)
	}
	fmt.Println("tsnet install complete: tailscale installed in profile")
	return nil
}

func parseInstallArgs(argv []string) installOptions {
	fs := flag.NewFlagSet("tsnet v1 install", flag.ContinueOnError)
	nixpkgs := fs.String("nixpkgs-url", "", "Optional nix expression URL")
	ensure := fs.Bool("ensure", false, "Install tailscale with `nix profile install`")
	_ = fs.Parse(argv)
	return installOptions{
		nixpkgsURL: strings.TrimSpace(*nixpkgs),
		ensure:     *ensure,
	}
}

type installOptions struct {
	nixpkgsURL string
	ensure     bool
}

func runNixShellCommand(command string, arg string, pkgs []string, nixExpr string) error {
	if os.Getenv("DIALTONE_NIX_ACTIVE") == "1" {
		cmd := exec.Command(command, arg)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	base := []string{
		"--extra-experimental-features",
		"nix-command flakes",
		"shell",
	}
	if nixExpr != "" {
		base = append(base, "-f", nixExpr)
	}
	base = append(base, pkgs...)
	base = append(base, "--command", command, arg)
	cmd := exec.Command("nix", base...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runNixProfileInstall(pkgs []string, nixExpr string) error {
	base := []string{
		"profile",
		"install",
	}
	for _, p := range pkgs {
		if nixExpr != "" {
			base = append(base, strings.TrimPrefix(p, "nixpkgs#"))
		} else {
			base = append(base, p)
		}
	}
	cmd := exec.Command("nix", base...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func tsnetNixPackagesAndSource(nixpkgsURL string) ([]string, string) {
	pkgs := make([]string, len(tsnetInstallPackages))
	copy(pkgs, tsnetInstallPackages)
	url := strings.TrimSpace(nixpkgsURL)
	if url == "" {
		url = strings.TrimSpace(os.Getenv("NIXPKGS_URL"))
	}
	if url == "" {
		return pkgs, ""
	}
	for i, p := range pkgs {
		pkgs[i] = strings.TrimPrefix(p, "nixpkgs#")
	}
	return pkgs, url
}

func joinPackages(pkgs []string) string {
	return strings.Join(pkgs, ", ")
}

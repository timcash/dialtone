package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var chromeInstallPackages = []string{
	"nixpkgs#bashInteractive",
}

func runInstall(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("chrome install does not accept positional arguments")
	}

	repoRoot, _ := locateRepoRoot()
	platformPkgs, checkCommand := chromeInstallSpec()
	pkgs, nixExpr := chromeNixPackagesAndSource(platformPkgs)
	base := []string{
		"--extra-experimental-features",
		"nix-command flakes",
		"shell",
	}
	if nixExpr != "" {
		base = append(base, "-f", nixExpr)
	}
	base = append(base, pkgs...)
	base = append(base, "--command", "bash", "-lc", checkCommand)

	cmd := exec.Command("bash", "-lc", checkCommand)
	if os.Getenv("DIALTONE_NIX_ACTIVE") != "1" {
		cmd = exec.Command("nix", base...)
		if strings.TrimSpace(repoRoot) != "" {
			cmd.Dir = repoRoot
		}
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("chrome install failed: %w", err)
	}

	fmt.Printf("chrome install check complete: %s\n", strings.Join(pkgs, ", "))
	return nil
}

func chromeNixPackagesAndSource(basePkgs []string) ([]string, string) {
	pkgs := make([]string, len(basePkgs))
	copy(pkgs, basePkgs)

	nixpkgsURL := strings.TrimSpace(os.Getenv("NIXPKGS_URL"))
	if nixpkgsURL == "" {
		return pkgs, ""
	}
	for i, p := range pkgs {
		pkgs[i] = strings.TrimPrefix(p, "nixpkgs#")
	}
	return pkgs, nixpkgsURL
}

func chromeInstallSpec() ([]string, string) {
	pkgs := make([]string, len(chromeInstallPackages))
	copy(pkgs, chromeInstallPackages)
	if runtime.GOOS == "darwin" {
		return pkgs, "true"
	}
	pkgs = append(pkgs, "nixpkgs#chromium")
	return pkgs, "command -v chromium >/dev/null || command -v chromium-browser >/dev/null || command -v google-chrome >/dev/null || command -v google-chrome-stable >/dev/null"
}

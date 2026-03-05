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

func runSetup(args []string) error {
	cfg := parseSetupArgs(args)

	if cfg.host == "" {
		if err := ensureLocalMoshServer(cfg.ensure); err != nil {
			return err
		}
		fmt.Println("mosh setup complete for local host")
		return nil
	}

	if err := ensureRemoteMoshServer(cfg.host, cfg.ensure); err != nil {
		return err
	}
	fmt.Printf("mosh setup complete for host %s\n", cfg.host)
	return nil
}

type setupOptions struct {
	host   string
	ensure bool
}

func parseSetupArgs(argv []string) setupOptions {
	fs := flag.NewFlagSet("mosh v1 setup", flag.ContinueOnError)
	host := fs.String("host", "", "Remote host name")
	ensure := fs.Bool("ensure", false, "Try to install mosh on remote/local target using nix profile")
	_ = fs.Parse(argv)
	return setupOptions{host: strings.TrimSpace(*host), ensure: *ensure}
}

func ensureLocalMoshServer(ensureInstall bool) error {
	if err := checkMoshClient(); err != nil {
		if !ensureInstall {
			return err
		}
		if err := profileInstallMosh("local"); err != nil {
			return err
		}
		if err := checkMoshClient(); err != nil {
			return fmt.Errorf("install mosh succeeded but server not available locally: %w", err)
		}
	}
	return nil
}

func ensureRemoteMoshServer(host string, ensureInstall bool) error {
	if isLocalHost(host) {
		return ensureLocalMoshServer(ensureInstall)
	}

	if _, err := exec.LookPath("ssh"); err != nil {
		return errors.New("ssh unavailable; cannot setup remote mosh server")
	}

	cmdCheck := "command -v mosh-server >/dev/null || [ -x \"$HOME/.nix-profile/bin/mosh-server\" ] || [ -x \"$HOME/.local/bin/mosh-server\" ]"
	if err := runSSHCommand(host, cmdCheck); err == nil {
		return nil
	}
	if !ensureInstall {
		return fmt.Errorf("mosh-server not found on %s", host)
	}

	if err := profileInstallMosh(host); err != nil {
		return err
	}
	return runSSHCommand(host, cmdCheck)
}

func checkMoshClient() error {
	if !isExecutableAvailable("mosh") {
		return errors.New("mosh client not found; run ./dialtone_mod mosh v1 setup --ensure")
	}
	if !isExecutableAvailable("mosh-server") {
		return errors.New("mosh-server not found locally")
	}
	return nil
}

func isExecutableAvailable(name string) bool {
	if _, err := exec.LookPath(name); err == nil {
		return true
	}

	home := os.Getenv("HOME")
	candidates := []string{
		filepath.Join(home, ".nix-profile/bin", name),
		filepath.Join(home, ".local/bin", name),
		filepath.Join("/nix/var/nix/profiles/default/bin", name),
		filepath.Join("/run/current-system/sw/bin", name),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return true
		}
	}
	return false
}

func profileInstallMosh(target string) error {
	if target == "local" {
		nixBin, err := locateNixBinary()
		if err != nil {
			return fmt.Errorf("nix missing on local host; cannot install mosh: %w", err)
		}
		return profileInstallMoshWithCommand(nixBin)
	}

	script := buildRemoteNixInstallScript(target)
	return runSSHCommand(target, script)
}

func buildRemoteNixInstallScript(target string) string {
	return fmt.Sprintf(`set -e
for p in /usr/local/bin/nix /nix/var/nix/profiles/default/bin/nix /home/user/.nix-profile/bin/nix /run/current-system/sw/bin/nix /nix/store/*-nix-*/bin/nix; do
  if [ -x "$p" ]; then
    if "$p" --extra-experimental-features nix-command --extra-experimental-features flakes profile install nixpkgs#mosh; then
      exit 0
    fi
  fi
done
echo "nix unavailable on remote target %s; cannot install mosh"
exit 2`, target)
}

func profileInstallMoshWithCommand(nixBin string) error {
	cmd := exec.Command(
		nixBin,
		"--extra-experimental-features", "nix-command",
		"--extra-experimental-features", "flakes",
		"profile", "install", "nixpkgs#mosh",
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install mosh locally with nix profile: %w", err)
	}
	return nil
}

func runSSHCommand(host, command string) error {
	if isLocalHost(host) {
		cmd := exec.Command("bash", "-lc", command)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	opts := []string{
		"-F", "/dev/null",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "GSSAPIAuthentication=no",
	}
	args := make([]string, 0, len(opts)+3)
	args = append(args, opts...)
	args = append(args, host, "bash", "-lc", command)
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

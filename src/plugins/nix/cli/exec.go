package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func RunExec(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ./dialtone.sh nix exec <nix args...>")
	}
	nixBin, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("nix executable not found in PATH")
	}
	nixArgs := ensureExperimentalFeatures(args)
	cmd := exec.Command(nixBin, nixArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func RunInstallable(installable string, args []string) error {
	installable = strings.TrimSpace(installable)
	if installable == "" {
		return fmt.Errorf("usage: ./dialtone.sh nix run <installable> [-- <args...>]")
	}
	nixArgs := []string{"run", installable}
	if len(args) > 0 {
		nixArgs = append(nixArgs, "--")
		nixArgs = append(nixArgs, args...)
	}
	return RunExec(nixArgs)
}

func ensureExperimentalFeatures(args []string) []string {
	if len(args) >= 2 && args[0] == "--extra-experimental-features" {
		return args
	}
	out := []string{"--extra-experimental-features", "nix-command flakes"}
	out = append(out, args...)
	return out
}

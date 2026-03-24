package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"dialtone/dev/internal/modcli"
)

func runInstall(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("shell install does not accept positional arguments")
	}
	repoRoot, err := modcli.FindRepoRoot()
	if err != nil {
		return err
	}
	cmd := modcli.NixDevelopCommand(repoRoot, modcli.DefaultShell, "bash", "-lc", "command -v bash >/dev/null && command -v go >/dev/null && command -v tmux >/dev/null")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("shell install failed: %w", err)
	}
	fmt.Println("shell v1 install complete")
	return nil
}

func runBuild(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("shell build does not accept positional arguments")
	}
	repoRoot, err := modcli.FindRepoRoot()
	if err != nil {
		return err
	}
	binPath, err := modcli.BuildOutputPath(repoRoot, "shell", "v1", "shell")
	if err != nil {
		return err
	}
	cmd := modcli.GoBuildCommand(repoRoot, modcli.DefaultShell, binPath, "./mods/shell/v1/cli")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("shell build failed: %w", err)
	}
	fmt.Printf("built shell v1 binary: %s\n", binPath)
	return nil
}

func runFormat(args []string) error {
	targetDir, err := parseFormatArgs(args)
	if err != nil {
		return err
	}
	repoRoot, err := modcli.FindRepoRoot()
	if err != nil {
		return err
	}
	if targetDir == "" {
		targetDir = modcli.ModDir(repoRoot, "shell", "v1")
	}
	if !filepath.IsAbs(targetDir) {
		targetDir = filepath.Join(repoRoot, targetDir)
	}
	files, err := modcli.CollectGoFiles(targetDir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	cmd := modcli.NixDevelopCommand(repoRoot, modcli.DefaultShell, append([]string{"gofmt", "-w"}, files...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("shell format failed: %w", err)
	}
	return nil
}

func parseFormatArgs(argv []string) (string, error) {
	fs := flag.NewFlagSet("shell v1 format", flag.ContinueOnError)
	dir := fs.String("dir", "", "Directory to format (default: src/mods/shell/v1)")
	if err := fs.Parse(argv); err != nil {
		return "", err
	}
	return modcli.NormalizeOptionalPathArg(*dir), nil
}

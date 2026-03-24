package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"dialtone/dev/internal/modcli"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "help", "-h", "--help":
		printUsage()
	case "install":
		exitIfErr(runInstall(args), "ssh install")
	case "build":
		exitIfErr(runBuild(args), "ssh build")
	case "format":
		exitIfErr(runFormat(args), "ssh format")
	case "test":
		if len(args) == 0 {
			exitIfErr(runPackageTests(), "ssh test")
			return
		}
		exitIfErr(runRuntime(append([]string{command}, args...)), "ssh runtime")
	default:
		exitIfErr(runRuntime(append([]string{command}, args...)), "ssh runtime")
	}
}

func runInstall(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("ssh install does not accept positional arguments")
	}
	repoRoot, err := modcli.FindRepoRoot()
	if err != nil {
		return err
	}
	cmd := modcli.NixDevelopCommand(repoRoot, modcli.DefaultShell, "bash", "-lc", "command -v go >/dev/null && command -v ssh >/dev/null")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh install failed: %w", err)
	}
	fmt.Println("ssh v1 install complete")
	return nil
}

func runBuild(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("ssh build does not accept positional arguments")
	}
	repoRoot, err := modcli.FindRepoRoot()
	if err != nil {
		return err
	}
	binPath, err := modcli.BuildOutputPath(repoRoot, "ssh", "v1", "ssh")
	if err != nil {
		return err
	}
	cmd := modcli.GoBuildCommand(repoRoot, modcli.DefaultShell, binPath, "./mods/ssh/v1/cli")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh build failed: %w", err)
	}
	fmt.Printf("built ssh v1 binary: %s\n", binPath)
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
		targetDir = modcli.ModDir(repoRoot, "ssh", "v1")
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
		return fmt.Errorf("ssh format failed: %w", err)
	}
	return nil
}

func runPackageTests() error {
	repoRoot, err := modcli.FindRepoRoot()
	if err != nil {
		return err
	}
	cmd := modcli.GoTestCommand(repoRoot, modcli.DefaultShell, "./mods/ssh/v1/...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh test failed: %w", err)
	}
	return nil
}

func runRuntime(args []string) error {
	repoRoot, err := modcli.FindRepoRoot()
	if err != nil {
		return err
	}
	cmd := modcli.NixDevelopCommand(repoRoot, modcli.DefaultShell, append([]string{"go", "run", "./mods/ssh/v1"}, args...)...)
	cmd.Dir = modcli.SrcRoot(repoRoot)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	return nil
}

func parseFormatArgs(argv []string) (string, error) {
	fs := flag.NewFlagSet("ssh v1 format", flag.ContinueOnError)
	dir := fs.String("dir", "", "Directory to format (default: src/mods/ssh/v1)")
	if err := fs.Parse(argv); err != nil {
		return "", err
	}
	return modcli.NormalizeOptionalPathArg(*dir), nil
}

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod ssh v1 <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  install")
	fmt.Println("       Verify Go plus ssh are available in the default nix shell")
	fmt.Println("  build")
	fmt.Println("       Build the ssh v1 CLI wrapper to <repo-root>/bin/mods/ssh/v1/ssh")
	fmt.Println("  format [--dir DIR]")
	fmt.Println("       Run gofmt on ssh v1 Go files")
	fmt.Println("  test")
	fmt.Println("       With no args, run go test for ssh v1. With ssh runtime flags, delegate to the runtime reachability test")
	fmt.Println("  run|--host ...")
	fmt.Println("       Delegate to the existing ssh runtime entrypoint")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s error: %v\n", context, err)
	os.Exit(1)
}

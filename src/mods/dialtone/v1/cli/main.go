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
		exitIfErr(runInstall(args), "dialtone install")
	case "build":
		exitIfErr(runBuild(args), "dialtone build")
	case "format":
		exitIfErr(runFormat(args), "dialtone format")
	case "test":
		exitIfErr(runTest(args), "dialtone test")
	default:
		exitIfErr(runRuntime(append([]string{command}, args...)), "dialtone runtime")
	}
}

func runInstall(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("dialtone install does not accept positional arguments")
	}
	repoRoot, err := modcli.FindRepoRoot()
	if err != nil {
		return err
	}
	cmd := modcli.NixDevelopCommand(repoRoot, modcli.DefaultShell, "bash", "-lc", "command -v go >/dev/null && command -v tmux >/dev/null")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("dialtone install failed: %w", err)
	}
	fmt.Println("dialtone v1 install complete")
	return nil
}

func runBuild(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("dialtone build does not accept positional arguments")
	}
	repoRoot, err := modcli.FindRepoRoot()
	if err != nil {
		return err
	}
	binPath, err := modcli.BuildOutputPath(repoRoot, "dialtone", "v1", "dialtone")
	if err != nil {
		return err
	}
	cmd := modcli.GoBuildCommand(repoRoot, modcli.DefaultShell, binPath, "./mods/dialtone/v1")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("dialtone build failed: %w", err)
	}
	fmt.Printf("built dialtone v1 binary: %s\n", binPath)
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
		targetDir = modcli.ModDir(repoRoot, "dialtone", "v1")
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
		return fmt.Errorf("dialtone format failed: %w", err)
	}
	return nil
}

func runTest(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("dialtone test does not accept positional arguments")
	}
	repoRoot, err := modcli.FindRepoRoot()
	if err != nil {
		return err
	}
	cmd := modcli.GoTestCommand(repoRoot, modcli.DefaultShell, dialtoneTestPackages()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("dialtone test failed: %w", err)
	}
	return nil
}

func dialtoneTestPackages() []string {
	return []string{
		"./internal/modstate",
		"./mods/dialtone/v1/...",
		"./mods/shared/dispatch",
		"./mods/shared/router",
		"./mods/shared/sqlitestate",
	}
}

func runRuntime(args []string) error {
	repoRoot, err := modcli.FindRepoRoot()
	if err != nil {
		return err
	}
	cmd := modcli.NixDevelopCommand(repoRoot, modcli.DefaultShell, append([]string{"go", "run", "./mods/dialtone/v1"}, args...)...)
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
	fs := flag.NewFlagSet("dialtone v1 format", flag.ContinueOnError)
	dir := fs.String("dir", "", "Directory to format (default: src/mods/dialtone/v1)")
	if err := fs.Parse(argv); err != nil {
		return "", err
	}
	return modcli.NormalizeOptionalPathArg(*dir), nil
}

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod dialtone v1 <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  install")
	fmt.Println("       Verify Go and tmux are available in the default nix shell")
	fmt.Println("  build")
	fmt.Println("       Build the standalone dialtone daemon to <repo-root>/bin/mods/dialtone/v1/dialtone")
	fmt.Println("  format [--dir DIR]")
	fmt.Println("       Run gofmt on dialtone v1 Go files")
	fmt.Println("  test")
	fmt.Println("       Run go test for dialtone v1 plus queue/state helper packages")
	fmt.Println("  ensure|serve|status|state|bootstrap|queue|paths|processes|commands|command|log|logs|protocol-runs|protocol-run|test-runs|test-run")
	fmt.Println("       Delegate to the dialtone daemon runtime entrypoint")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s error: %v\n", context, err)
	os.Exit(1)
}

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
		exitIfErr(runInstall(args), "db install")
	case "build":
		exitIfErr(runBuild(args), "db build")
	case "format":
		exitIfErr(runFormat(args), "db format")
	case "test":
		exitIfErr(runTest(args), "db test")
	case "run", "exec":
		exitIfErr(runBinary(args), "db run")
	default:
		if strings.HasPrefix(command, "-") {
			exitIfErr(runBinary(append([]string{command}, args...)), "db run")
			return
		}
		exitIfErr(fmt.Errorf("unknown db command: %s", command), "db")
	}
}

func runInstall(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("db install does not accept positional arguments")
	}
	cmd, err := zigShellCommand("zig", "version")
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("db install failed: %w", err)
	}
	fmt.Println("db v1 install complete")
	return nil
}

func runBuild(args []string) error {
	cmd, err := zigShellCommand(append([]string{"bash", "./build.sh"}, args...)...)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("db build failed: %w", err)
	}
	repoRoot, err := modcli.FindRepoRoot()
	if err != nil {
		return err
	}
	fmt.Printf("built db v1 binary: %s\n", filepath.Join(repoRoot, "bin", "mods", "db", "v1", "dialtone_db"))
	return nil
}

func runFormat(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("db format does not accept positional arguments")
	}
	cmd, err := zigShellCommand("bash", "-lc", "zig fmt build.zig main.zig zig_test/build.zig zig_test/src/*.zig")
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("db format failed: %w", err)
	}
	return nil
}

func runTest(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("db test does not accept positional arguments")
	}
	if err := runBuild(nil); err != nil {
		return err
	}
	cmd, err := zigShellCommand("bash", "./zig_test/test.sh")
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("db zig tests failed: %w", err)
	}
	return nil
}

func runBinary(args []string) error {
	repoRoot, err := modcli.FindRepoRoot()
	if err != nil {
		return err
	}
	binary := filepath.Join(repoRoot, "bin", "mods", "db", "v1", "dialtone_db")
	if _, err := os.Stat(binary); err != nil {
		if buildErr := runBuild(nil); buildErr != nil {
			return buildErr
		}
	}
	cmd := exec.Command(binary, args...)
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

func zigShellCommand(args ...string) (*exec.Cmd, error) {
	repoRoot, err := modcli.FindRepoRoot()
	if err != nil {
		return nil, err
	}
	modRoot := modcli.ModDir(repoRoot, "db", "v1")
	if os.Getenv("DIALTONE_NIX_ACTIVE") == "1" {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = modRoot
		return cmd, nil
	}
	nixArgs := []string{
		"--extra-experimental-features", "nix-command flakes",
		"shell", "nixpkgs#zig",
		"--command",
	}
	nixArgs = append(nixArgs, args...)
	cmd := exec.Command("nix", nixArgs...)
	cmd.Dir = modRoot
	return cmd, nil
}

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod db v1 <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  install")
	fmt.Println("       Verify zig is available through nix")
	fmt.Println("  build")
	fmt.Println("       Build dialtone_db into <repo-root>/bin/mods/db/v1/dialtone_db")
	fmt.Println("  format")
	fmt.Println("       Run zig fmt on the db v1 Zig sources")
	fmt.Println("  test")
	fmt.Println("       Run the Zig test harness for db v1")
	fmt.Println("  run|exec [dialtone_db args...]")
	fmt.Println("       Run the built dialtone_db binary, building it first if needed")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s error: %v\n", context, err)
	os.Exit(1)
}

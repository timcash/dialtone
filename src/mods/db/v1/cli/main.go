package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dialtone/dev/internal/modcli"
)

const dbArtifactName = "dialtone_db"

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
	repoRoot, err := modcli.FindRepoRoot()
	if err != nil {
		return err
	}
	cmd := dbShellCommand(repoRoot, "bash", "-lc", "command -v zig >/dev/null")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("db install failed: %w", err)
	}
	fmt.Println("db v1 install complete")
	return nil
}

func runBuild(args []string) error {
	repoRoot, err := modcli.FindRepoRoot()
	if err != nil {
		return err
	}
	cmd := dbShellCommand(repoRoot, append([]string{"bash", "./build.sh"}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("db build failed: %w", err)
	}
	binary, err := dbBinaryPath(repoRoot)
	if err != nil {
		return err
	}
	fmt.Printf("built db v1 binary: %s\n", binary)
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
		targetDir = modcli.ModDir(repoRoot, "db", "v1")
	}
	if !filepath.IsAbs(targetDir) {
		targetDir = filepath.Join(repoRoot, targetDir)
	}
	files, err := collectZigFiles(targetDir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	cmd := dbShellCommand(repoRoot, append([]string{"zig", "fmt"}, files...)...)
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
	repoRoot, err := modcli.FindRepoRoot()
	if err != nil {
		return err
	}
	cmd := dbShellCommand(repoRoot, "bash", "./zig_test/test.sh")
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
	binary, err := dbBinaryPath(repoRoot)
	if err != nil {
		return err
	}
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

func dbBinaryPath(repoRoot string) (string, error) {
	return modcli.BuildOutputPath(repoRoot, "db", "v1", dbArtifactName)
}

func dbShellCommand(repoRoot string, args ...string) *exec.Cmd {
	cmd := modcli.NixDevelopCommand(repoRoot, modcli.DefaultShell, args...)
	cmd.Dir = modcli.ModDir(repoRoot, "db", "v1")
	return cmd
}

func parseFormatArgs(argv []string) (string, error) {
	fs := flag.NewFlagSet("db v1 format", flag.ContinueOnError)
	dir := fs.String("dir", "", "Directory to format (default: src/mods/db/v1)")
	if err := fs.Parse(argv); err != nil {
		return "", err
	}
	return modcli.NormalizeOptionalPathArg(*dir), nil
}

func collectZigFiles(root string) ([]string, error) {
	files := []string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".zig-cache", "zig-out":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".zig" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod db v1 <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  install")
	fmt.Println("       Verify zig is available in the default nix shell")
	fmt.Println("  build [zig-build args...]")
	fmt.Println("       Build dialtone_db into <repo-root>/bin/mods/db/v1/dialtone_db")
	fmt.Println("  format [--dir DIR]")
	fmt.Println("       Run zig fmt on db v1 Zig files")
	fmt.Println("  test")
	fmt.Println("       Build dialtone_db and run the Zig test harness")
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

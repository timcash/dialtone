package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

// RunGo handles versioned go commands.
func RunGo(args []string) {
	if len(args) == 0 {
		printGoUsage()
		return
	}

	normalized, warnedOldOrder, err := normalizeArgs(args)
	if err != nil {
		logs.Error("%v", err)
		printGoUsage()
		return
	}
	if warnedOldOrder {
		logs.Warn("old go CLI order is deprecated. Use: ./dialtone.sh go src_v1 <command> [args]")
	}

	subcommand := normalized[0]
	restArgs := normalized[1:]

	switch subcommand {
	case "install":
		runInstall(restArgs)
	case "lint":
		runLint(restArgs)
	case "test":
		runTest(restArgs)
	case "exec", "run":
		runExec(restArgs)
	case "pb-dump":
		runPbDump(restArgs)
	case "help", "-h", "--help":
		printGoUsage()
	default:
		logs.Error("Unknown go command: %s", subcommand)
		printGoUsage()
		os.Exit(1)
	}
}

func printGoUsage() {
	logs.Raw("Usage: ./dialtone.sh go src_v1 <command> [options]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  install        Install Go toolchain to DIALTONE_ENV")
	logs.Raw("  lint           Run go vet ./... using local toolchain")
	logs.Raw("  test           Run go plugin integration tests")
	logs.Raw("  exec <args...> Run arbitrary go command using local toolchain")
	logs.Raw("  run <args...>  Alias for exec")
	logs.Raw("  pb-dump <file> Dump structure/strings of a protobuf file")
	logs.Raw("  help           Show this help message")
}

func runTest(args []string) {
	if len(args) > 0 {
		logs.Fatal("Usage: ./dialtone.sh go src_v1 test")
	}
	runPluginTests()
}

func runExec(args []string) {
	if len(args) == 0 {
		logs.Fatal("Usage: ./dialtone.sh go src_v1 exec <args...>")
	}

	depsDir := logs.GetDialtoneEnv()
	goDir := filepath.Join(depsDir, "go")
	goBin := filepath.Join(goDir, "bin", "go")

	if _, err := os.Stat(goBin); os.IsNotExist(err) {
		logs.Fatal("Go toolchain not found. Run './dialtone.sh go src_v1 install' first.")
	}

	// Ensure this toolchain uses its own libraries/binaries.
	_ = os.Setenv("GOROOT", goDir)
	newPath := filepath.Join(goDir, "bin") + string(os.PathListSeparator) + os.Getenv("PATH")
	_ = os.Setenv("PATH", newPath)

	cmd := exec.Command(goBin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
		logs.Fatal("Command failed: %v", err)
	}
}

func runPbDump(args []string) {
	if len(args) < 1 {
		logs.Fatal("Usage: ./dialtone.sh go src_v1 pb-dump <file.pb>")
	}

	toolPath := "src/plugins/go/tools/pb-dump/main.go"
	execArgs := append([]string{"run", toolPath}, args...)
	runExec(execArgs)
}

func runInstall(args []string) {
	_ = args
	logs.Info("Installing Go toolchain...")

	depsDir := logs.GetDialtoneEnv()
	if depsDir == "" {
		logs.Fatal("DIALTONE_ENV not set in env/.env or environment")
	}

	if err := os.MkdirAll(depsDir, 0o755); err != nil {
		logs.Fatal("Failed to create DIALTONE_ENV directory: %v", err)
	}

	goVersion := "1.25.5"
	goDir := filepath.Join(depsDir, "go")
	goBin := filepath.Join(goDir, "bin", "go")

	if _, err := os.Stat(goBin); err == nil {
		logs.Info("Go %s is already installed at %s", goVersion, goBin)
		return
	}

	osName := runtime.GOOS
	archName := runtime.GOARCH
	tarball := fmt.Sprintf("go%s.%s-%s.tar.gz", goVersion, osName, archName)
	downloadURL := fmt.Sprintf("https://go.dev/dl/%s", tarball)
	destTar := filepath.Join(depsDir, tarball)

	logs.Info("Downloading %s to %s...", downloadURL, destTar)

	var downloadCmd *exec.Cmd
	if _, err := exec.LookPath("curl"); err == nil {
		downloadCmd = exec.Command("curl", "-L", "-o", destTar, downloadURL)
	} else if _, err := exec.LookPath("wget"); err == nil {
		downloadCmd = exec.Command("wget", "-O", destTar, downloadURL)
	} else {
		logs.Fatal("Neither curl nor wget found in PATH")
	}

	downloadCmd.Stdout = os.Stdout
	downloadCmd.Stderr = os.Stderr
	if err := downloadCmd.Run(); err != nil {
		logs.Fatal("Failed to download Go: %v", err)
	}

	logs.Info("Extracting %s...", destTar)
	extractCmd := exec.Command("tar", "-C", depsDir, "-xzf", destTar)
	extractCmd.Stdout = os.Stdout
	extractCmd.Stderr = os.Stderr
	if err := extractCmd.Run(); err != nil {
		logs.Fatal("Failed to extract Go: %v", err)
	}

	if err := os.Remove(destTar); err != nil {
		logs.Warn("Failed to remove temporary tarball %s: %v", destTar, err)
	}

	logs.Info("Go toolchain installed successfully at %s", goDir)
}

func runLint(args []string) {
	_ = args
	logs.Info("Running Go lint (vet)...")

	depsDir := logs.GetDialtoneEnv()
	goBin := filepath.Join(depsDir, "go", "bin", "go")

	if _, err := os.Stat(goBin); os.IsNotExist(err) {
		if p, lookErr := exec.LookPath("go"); lookErr == nil {
			goBin = p
		} else {
			logs.Fatal("Go toolchain not found. Run './dialtone.sh go src_v1 install' first.")
		}
	}

	cmd := exec.Command(goBin, "vet", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logs.Fatal("Lint failed: %v", err)
	}
	logs.Info("Lint passed.")
}

func normalizeArgs(args []string) ([]string, bool, error) {
	if len(args) == 0 {
		return nil, false, fmt.Errorf("missing arguments")
	}
	if isHelpArg(args[0]) {
		return []string{"help"}, false, nil
	}
	if strings.HasPrefix(args[0], "src_v") {
		if args[0] != "src_v1" {
			return nil, false, fmt.Errorf("unsupported version %s", args[0])
		}
		if len(args) < 2 {
			return nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh go src_v1 <command> [args])")
		}
		return append([]string{args[1]}, args[2:]...), false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		if args[1] != "src_v1" {
			return nil, false, fmt.Errorf("unsupported version %s", args[1])
		}
		return append([]string{args[0]}, args[2:]...), true, nil
	}
	return nil, false, fmt.Errorf("expected version as first go argument (usage: ./dialtone.sh go src_v1 <command> [args])")
}

func isHelpArg(s string) bool {
	switch s {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
}

func runPluginTests() {
	repoRoot, err := findRepoRoot()
	if err != nil {
		logs.Fatal("%v", err)
	}
	cmd := exec.Command("go", "run", "./plugins/go/src_v1/test/cmd/main.go")
	cmd.Dir = filepath.Join(repoRoot, "src")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", fmt.Errorf("repo root not found")
		}
		cwd = parent
	}
}

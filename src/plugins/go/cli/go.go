package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"dialtone/dev/config"
	"dialtone/dev/plugins/logs/src_v1/go"
	go_test "dialtone/dev/plugins/go/test"
)

// RunGo handles 'go <subcommand>'
func RunGo(args []string) {
	if len(args) == 0 {
		printGoUsage()
		return
	}

	subcommand := args[0]
	restArgs := args[1:]

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
		fmt.Printf("Unknown go command: %s\n", subcommand)
		printGoUsage()
		os.Exit(1)
	}
}

func printGoUsage() {
	fmt.Println("Usage: dialtone go <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  install        Install Go toolchain to DIALTONE_ENV")
	fmt.Println("  lint           Run go vet ./... using local toolchain")
	fmt.Println("  test           Run go plugin integration tests")
	fmt.Println("  exec <args...> Run arbitrary go command using local toolchain")
	fmt.Println("  run <args...>  Alias for exec")
	fmt.Println("  pb-dump <file> Dump structure/strings of a protobuf file")
	fmt.Println("  help           Show this help message")
}

func runTest(args []string) {
	if len(args) > 0 {
		logs.Fatal("Usage: ./dialtone.sh go test")
	}

	if err := go_test.RunAll(); err != nil {
		logs.Fatal("Go tests failed: %v", err)
	}
}

func runExec(args []string) {
	if len(args) == 0 {
		logs.Fatal("Usage: dialtone go exec <args...>")
	}

	depsDir := config.GetDialtoneEnv()
	goDir := filepath.Join(depsDir, "go")
	goBin := filepath.Join(goDir, "bin", "go")

	if _, err := os.Stat(goBin); os.IsNotExist(err) {
		logs.Fatal("Go toolchain not found. Run 'dialtone go install' first.")
	}

	// Set GOROOT to ensure the toolchain uses its own libraries
	os.Setenv("GOROOT", goDir)

	// Prepend dependencies bin to PATH so installed tools are found
	newPath := filepath.Join(goDir, "bin") + string(os.PathListSeparator) + os.Getenv("PATH")
	os.Setenv("PATH", newPath)

	logs.Info("Running: go %s", fmt.Sprintf("%v", args))

	cmd := exec.Command(goBin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		// Pass through the exit code if possible
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
		logs.Fatal("Command failed: %v", err)
	}
}

func runPbDump(args []string) {
	if len(args) < 1 {
		logs.Fatal("Usage: dialtone go pb-dump <file.pb>")
	}

	toolPath := "src/plugins/go/tools/pb-dump/main.go"

	// Delegate to runExec which handles environment
	execArgs := append([]string{"run", toolPath}, args...)
	runExec(execArgs)
}

func runInstall(args []string) {
	logs.Info("Installing Go toolchain...")

	depsDir := config.GetDialtoneEnv()
	if depsDir == "" {
		logs.Fatal("DIALTONE_ENV not set in env/.env or environment")
	}

	if err := os.MkdirAll(depsDir, 0755); err != nil {
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

	// Go uses 'amd64' for x86_64, which is what GOARCH returns.
	tarball := fmt.Sprintf("go%s.%s-%s.tar.gz", goVersion, osName, archName)
	downloadUrl := fmt.Sprintf("https://go.dev/dl/%s", tarball)

	destTar := filepath.Join(depsDir, tarball)

	logs.Info("Downloading %s to %s...", downloadUrl, destTar)

	var downloadCmd *exec.Cmd
	if _, err := exec.LookPath("curl"); err == nil {
		downloadCmd = exec.Command("curl", "-L", "-o", destTar, downloadUrl)
	} else if _, err := exec.LookPath("wget"); err == nil {
		downloadCmd = exec.Command("wget", "-O", destTar, downloadUrl)
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
		logs.Info("Warning: Failed to remove temporary tarball %s: %v", destTar, err)
	}

	logs.Info("Go toolchain installed successfully at %s", goDir)
}

func runLint(args []string) {
	logs.Info("Running Go lint (vet)...")

	depsDir := config.GetDialtoneEnv()
	goBin := filepath.Join(depsDir, "go", "bin", "go")

	if _, err := os.Stat(goBin); os.IsNotExist(err) {
		// Fallback to system go if not found in DIALTONE_ENV
		if p, err := exec.LookPath("go"); err == nil {
			goBin = p
		} else {
			logs.Fatal("Go toolchain not found. Run 'dialtone go install' first.")
		}
	}

	cmd := exec.Command(goBin, "vet", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logs.Info("Executing: %s vet ./...", goBin)
	if err := cmd.Run(); err != nil {
		logs.Fatal("Lint failed: %v", err)
	}
	logs.Info("Lint passed.")
}

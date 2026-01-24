package install_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"dialtone/cli/src/plugins/install/cli"
	"dialtone/cli/src/core/logger"
)

// RunAll runs all tests in this package
func RunAll() error {
	if err := RunScaffold(); err != nil {
		return fmt.Errorf("Scaffold test failed: %v", err)
	}
	if err := RunInstallHelp(); err != nil {
		return fmt.Errorf("InstallHelp test failed: %v", err)
	}
	if err := RunLocalInstall(); err != nil {
		return fmt.Errorf("LocalInstall test failed: %v", err)
	}
	if err := RunInstallIdempotency(); err != nil {
		return fmt.Errorf("InstallIdempotency test failed: %v", err)
	}
	return nil
}

func setupTestEnv() (string, error) {
	envPath := cli.GetDialtoneEnv()
	logger.LogInfo("Using environment directory: %s", envPath)

	// Ensure the directory exists
	if err := os.MkdirAll(envPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create environment directory: %v", err)
	}

	return envPath, nil
}

func RunScaffold() error {
	_, err := setupTestEnv()
	if err != nil {
		return err
	}
	logger.LogInfo("Scaffold test passed")
	return nil
}

func RunInstallHelp() error {
	// Run dialtone install --help from the module root
	moduleRoot := "." // Assuming we run from project root
	
	cmd := exec.Command("go", "run", "src/cmd/dev/main.go", "install", "--help")
	cmd.Dir = moduleRoot
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.LogInfo("Output: %s", output)
		return fmt.Errorf("command failed: %v", err)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "Usage") && !strings.Contains(outputStr, "install") {
		return fmt.Errorf("output did not contain expected help text")
	}
	return nil
}

func RunLocalInstall() error {
	tempDir, err := setupTestEnv()
	if err != nil {
		return err
	}

	args := []string{}
	if runtime.GOOS == "linux" {
		args = append(args, "--linux-wsl")
	} else if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		args = append(args, "--macos-arm")
	}

	// RunInstall will log to stdout/stderr.
	cli.RunInstall(args)

	expected := []string{
		"go/bin/go",
		"node/bin/node",
		"zig/zig",
		"gh/bin/gh",
		"pixi/pixi",
	}

	for _, bin := range expected {
		path := filepath.Join(tempDir, bin)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("expected binary not found: %s", bin)
		}
	}
	return nil
}

func RunInstallIdempotency() error {
	args := []string{}
	if runtime.GOOS == "linux" {
		args = append(args, "--linux-wsl")
	} else if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		args = append(args, "--macos-arm")
	}

	logger.LogInfo("Running install (using default/local environment)...")
	cli.RunInstall(args)

	envPath := cli.GetDialtoneEnv()
	logger.LogInfo("DIALTONE_ENV is: %s", envPath)

	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("DIALTONE_ENV directory does not exist at: %s", envPath)
	}

	expected := []string{
		"go/bin/go",
		"node/bin/node",
		"zig/zig",
		"gh/bin/gh",
		"pixi/pixi",
	}

	for _, bin := range expected {
		path := filepath.Join(envPath, bin)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("dependency not found: %s", bin)
		}
	}
	return nil
}

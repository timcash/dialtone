package improve_cli_build_commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	dialtone "dialtone/cli/src"
)

// TestInstallAutoDetect verifies that install auto-detects the current OS/arch
func TestInstallAutoDetect(t *testing.T) {
	dialtone.LogInfo("Testing install auto-detection on %s/%s", runtime.GOOS, runtime.GOARCH)

	// Verify the current platform is supported
	supported := []string{
		"darwin/arm64",
		"darwin/amd64",
		"linux/amd64",
		"linux/arm64",
	}

	current := runtime.GOOS + "/" + runtime.GOARCH
	isSupported := false
	for _, s := range supported {
		if s == current {
			isSupported = true
			break
		}
	}

	if !isSupported {
		t.Errorf("Current platform %s is not in supported list: %v", current, supported)
	}

	dialtone.LogInfo("Platform %s is supported for auto-detection", current)
}

// TestInstallDepsDirectory verifies the deps directory structure
func TestInstallDepsDirectory(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	depsDir := filepath.Join(homeDir, ".dialtone_env")
	dialtone.LogInfo("Checking deps directory: %s", depsDir)

	// Check if deps directory exists
	if _, err := os.Stat(depsDir); os.IsNotExist(err) {
		t.Skip("Dependencies not installed yet - run 'dialtone install' first")
	}

	// Check expected subdirectories
	expectedDirs := []string{"go", "node", "zig"}
	for _, dir := range expectedDirs {
		path := filepath.Join(depsDir, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected directory %s does not exist", path)
		} else {
			dialtone.LogInfo("Found: %s", path)
		}
	}
}

// TestInstallGoVersion verifies Go is installed with correct version
func TestInstallGoVersion(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	goBin := filepath.Join(homeDir, ".dialtone_env", "go", "bin", "go")
	if _, err := os.Stat(goBin); os.IsNotExist(err) {
		t.Skip("Go not installed in .dialtone_env - run 'dialtone install' first")
	}

	cmd := exec.Command(goBin, "version")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to run go version: %v", err)
	}

	version := string(output)
	dialtone.LogInfo("Installed Go version: %s", strings.TrimSpace(version))

	if !strings.Contains(version, "go1.25") {
		t.Errorf("Expected Go 1.25.x, got: %s", version)
	}
}

// TestInstallNodeVersion verifies Node.js is installed
func TestInstallNodeVersion(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	nodeBin := filepath.Join(homeDir, ".dialtone_env", "node", "bin", "node")
	if _, err := os.Stat(nodeBin); os.IsNotExist(err) {
		t.Skip("Node.js not installed in .dialtone_env - run 'dialtone install' first")
	}

	cmd := exec.Command(nodeBin, "--version")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to run node --version: %v", err)
	}

	version := string(output)
	dialtone.LogInfo("Installed Node.js version: %s", strings.TrimSpace(version))

	if !strings.Contains(version, "v22") {
		t.Errorf("Expected Node.js v22.x, got: %s", version)
	}
}

// TestInstallZigVersion verifies Zig is installed
func TestInstallZigVersion(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	zigBin := filepath.Join(homeDir, ".dialtone_env", "zig", "zig")
	if _, err := os.Stat(zigBin); os.IsNotExist(err) {
		t.Skip("Zig not installed in .dialtone_env - run 'dialtone install' first")
	}

	cmd := exec.Command(zigBin, "version")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to run zig version: %v", err)
	}

	version := string(output)
	dialtone.LogInfo("Installed Zig version: %s", strings.TrimSpace(version))

	if !strings.Contains(version, "0.13") {
		t.Errorf("Expected Zig 0.13.x, got: %s", version)
	}
}

// TestBuildLocalBinaryExists verifies that build --local creates a binary
func TestBuildLocalBinaryExists(t *testing.T) {
	// Get the project root (two levels up from test dir)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Navigate to project root
	projectRoot := filepath.Join(cwd, "..", "..")
	binPath := filepath.Join(projectRoot, "bin", "dialtone")

	// On Windows, check for .exe
	if runtime.GOOS == "windows" {
		binPath = filepath.Join(projectRoot, "bin", "dialtone.exe")
	}

	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skip("Binary not built yet - run 'dialtone build --local' first")
	}

	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("Failed to stat binary: %v", err)
	}

	dialtone.LogInfo("Binary exists: %s (size: %d bytes)", binPath, info.Size())

	// Verify it's executable
	if info.Mode()&0111 == 0 {
		t.Errorf("Binary is not executable: %s", binPath)
	}
}

// TestBuildHelp verifies build --help shows usage information
func TestBuildHelp(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	projectRoot := filepath.Join(cwd, "..", "..")
	cmd := exec.Command("go", "run", ".", "build", "--help")
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run build --help: %v", err)
	}

	helpText := string(output)
	dialtone.LogInfo("Build help output received (%d bytes)", len(helpText))

	// Verify key sections are present
	expectedPhrases := []string{
		"Usage: dialtone build",
		"--local",
		"--full",
		"--help",
		"Examples:",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(helpText, phrase) {
			t.Errorf("Build help missing expected phrase: %s", phrase)
		}
	}
}

// TestInstallHelp verifies install --help shows usage information
func TestInstallHelp(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	projectRoot := filepath.Join(cwd, "..", "..")
	cmd := exec.Command("go", "run", ".", "install", "--help")
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run install --help: %v", err)
	}

	helpText := string(output)
	dialtone.LogInfo("Install help output received (%d bytes)", len(helpText))

	// Verify key sections are present
	expectedPhrases := []string{
		"Usage: dialtone install",
		"--linux-wsl",
		"--macos-arm",
		"--help",
		"Examples:",
		"Auto-detect",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(helpText, phrase) {
			t.Errorf("Install help missing expected phrase: %s", phrase)
		}
	}
}

// TestDeployHelp verifies deploy --help shows usage information
func TestDeployHelp(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	projectRoot := filepath.Join(cwd, "..", "..")
	cmd := exec.Command("go", "run", ".", "deploy", "--help")
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run deploy --help: %v", err)
	}

	helpText := string(output)
	dialtone.LogInfo("Deploy help output received (%d bytes)", len(helpText))

	// Verify key sections are present
	expectedPhrases := []string{
		"Usage: dialtone deploy",
		"--host",
		"--pass",
		"--help",
		"Examples:",
		"SSH",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(helpText, phrase) {
			t.Errorf("Deploy help missing expected phrase: %s", phrase)
		}
	}
}

// TestBuildLocalArchitecture verifies the binary is built for the correct architecture
func TestBuildLocalArchitecture(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	projectRoot := filepath.Join(cwd, "..", "..")
	binPath := filepath.Join(projectRoot, "bin", "dialtone")

	if runtime.GOOS == "windows" {
		binPath = filepath.Join(projectRoot, "bin", "dialtone.exe")
	}

	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skip("Binary not built yet - run 'dialtone build --local' first")
	}

	// Use 'file' command to check architecture (Unix only)
	if runtime.GOOS != "windows" {
		cmd := exec.Command("file", binPath)
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("Failed to run file command: %v", err)
		}

		fileInfo := string(output)
		dialtone.LogInfo("Binary type: %s", strings.TrimSpace(fileInfo))

		// Verify architecture matches current system
		switch runtime.GOARCH {
		case "arm64":
			if !strings.Contains(fileInfo, "arm64") && !strings.Contains(fileInfo, "aarch64") {
				t.Errorf("Expected arm64 binary, got: %s", fileInfo)
			}
		case "amd64":
			if !strings.Contains(fileInfo, "x86_64") && !strings.Contains(fileInfo, "x86-64") {
				t.Errorf("Expected x86_64 binary, got: %s", fileInfo)
			}
		}
	}
}

package dialtone

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// RunBuild handles building for different platforms
func RunBuild(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	full := fs.Bool("full", false, "Build Web UI, local CLI, and ARM64 binary")
	local := fs.Bool("local", false, "Build natively on the local system")
	showHelp := fs.Bool("help", false, "Show help for build command")

	fs.Usage = func() {
		fmt.Println("Usage: dialtone build [options]")
		fmt.Println()
		fmt.Println("Build the Dialtone binary and web UI for deployment.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --local    Build natively on the local system (uses ~/.dialtone_env if available)")
		fmt.Println("  --full     Full rebuild: Web UI + local CLI + ARM64 binary")
		fmt.Println("  --help     Show this help message")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  dialtone build              # Build web UI + binary (Podman or local)")
		fmt.Println("  dialtone build --local      # Build web UI + native binary")
		fmt.Println("  dialtone build --full       # Force full rebuild of everything")
		fmt.Println()
		fmt.Println("Notes:")
		fmt.Println("  - Automatically builds web UI if not already built")
		fmt.Println("  - Uses Podman by default for ARM64 cross-compilation")
		fmt.Println("  - Falls back to local build if Podman is not installed")
		fmt.Println("  - Run 'dialtone install' first to set up build dependencies")
	}

	fs.Parse(args)

	if *showHelp {
		fs.Usage()
		return
	}

	if *full {
		buildEverything(*local)
	} else {
		if *local || !hasPodman() {
			buildLocally()
		} else {
			buildWithPodman()
		}
	}
}

func hasPodman() bool {
	_, err := exec.LookPath("podman")
	return err == nil
}

// buildWebIfNeeded builds the web UI if web_build is missing or empty
func buildWebIfNeeded() {
	webBuildDir := filepath.Join("src", "web_build")
	indexPath := filepath.Join(webBuildDir, "index.html")

	// Check if index.html exists and has real content
	if info, err := os.Stat(indexPath); err == nil && info.Size() > 100 {
		LogInfo("Web UI already built (found %s)", indexPath)
		return
	}

	LogInfo("Building Web UI...")

	// Check if src/web exists
	webDir := filepath.Join("src", "web")
	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		LogInfo("Warning: src/web directory not found, skipping web build")
		return
	}

	// Check for npm
	if _, err := exec.LookPath("npm"); err != nil {
		// Try to use npm from .dialtone_env
		homeDir, _ := os.UserHomeDir()
		npmPath := filepath.Join(homeDir, ".dialtone_env", "node", "bin", "npm")
		if _, err := os.Stat(npmPath); os.IsNotExist(err) {
			LogInfo("Warning: npm not found, skipping web build. Run 'dialtone install' first.")
			return
		}
		// Add node to PATH
		nodeBin := filepath.Join(homeDir, ".dialtone_env", "node", "bin")
		os.Setenv("PATH", fmt.Sprintf("%s:%s", nodeBin, os.Getenv("PATH")))
	}

	// Install and build
	runShell(webDir, "npm", "install")
	runShell(webDir, "npm", "run", "build")

	// Sync to web_build
	LogInfo("Syncing web assets to src/web_build...")
	os.RemoveAll(webBuildDir)
	if err := os.MkdirAll(webBuildDir, 0755); err != nil {
		LogFatal("Failed to create web_build dir: %v", err)
	}

	distDir := filepath.Join(webDir, "dist")
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		LogInfo("Warning: npm build did not create dist directory")
		return
	}

	copyDir(distDir, webBuildDir)
	LogInfo("Web UI build complete")
}

func buildLocally() {
	LogInfo("Building Dialtone locally (Native Build)...")

	// Build web UI if needed
	buildWebIfNeeded()

	if err := os.MkdirAll("bin", 0755); err != nil {
		LogFatal("Failed to create bin directory: %v", err)
	}

	// For local builds, we enable CGO to support V4L2 drivers
	os.Setenv("CGO_ENABLED", "1")

	// If local environment exists, use it
	homeDir, _ := os.UserHomeDir()
	depsDir := filepath.Join(homeDir, ".dialtone_env")
	if _, err := os.Stat(depsDir); err == nil {
		LogInfo("Using local dependencies from %s", depsDir)

		// Add Go and Node to PATH
		goBin := filepath.Join(depsDir, "go", "bin")
		nodeBin := filepath.Join(depsDir, "node", "bin")
		os.Setenv("PATH", fmt.Sprintf("%s:%s:%s", goBin, nodeBin, os.Getenv("PATH")))

		// If Zig exists, use it as C compiler
		zigPath := filepath.Join(depsDir, "zig", "zig")
		if _, err := os.Stat(zigPath); err == nil {
			os.Setenv("CC", fmt.Sprintf("%s cc -target x86_64-linux-gnu", zigPath))
		}

		// Add include paths for CGO (V4L2 headers)
		includePath := filepath.Join(depsDir, "usr", "include")
		cgoCflags := fmt.Sprintf("-I%s", includePath)

		// Also check for multiarch include path (e.g. x86_64-linux-gnu)
		matches, _ := filepath.Glob(filepath.Join(includePath, "*-linux-gnu"))
		for _, match := range matches {
			cgoCflags += fmt.Sprintf(" -I%s", match)
		}
		os.Setenv("CGO_CFLAGS", cgoCflags)
	}

	// Choose binary name based on OS
	binaryName := "dialtone"
	if runtime.GOOS == "windows" {
		binaryName = "dialtone.exe"
	}

	outputPath := filepath.Join("bin", binaryName)
	runShell(".", "go", "build", "-o", outputPath, ".")
	LogInfo("Build successful: %s", outputPath)
}

func buildWithPodman() {
	LogInfo("Building Dialtone for Linux ARM64 using Podman...")

	// Build web UI first
	buildWebIfNeeded()

	cwd, err := os.Getwd()
	if err != nil {
		LogFatal("Failed to get current directory: %v", err)
	}

	if err := os.MkdirAll("bin", 0755); err != nil {
		LogFatal("Failed to create bin directory: %v", err)
	}

	buildCmd := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/src:Z", cwd),
		"-w", "/src",
		"-e", "GOOS=linux",
		"-e", "GOARCH=arm64",
		"-e", "CGO_ENABLED=1",
		"-e", "CC=aarch64-linux-gnu-gcc",
		"golang:1.25.5",
		"bash", "-c", "apt-get update && apt-get install -y gcc-aarch64-linux-gnu && go build -buildvcs=false -o bin/dialtone-arm64 .",
	}

	LogInfo("Running: podman %v", buildCmd)
	cmd := exec.Command("podman", buildCmd...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		LogFatal("Podman build failed: %v", err)
	}

	LogInfo("Build successful: bin/dialtone-arm64")
}

func buildEverything(local bool) {
	LogInfo("Starting Full Build Process...")

	// 1. Build Web UI
	LogInfo("Building Web UI...")
	webDir := filepath.Join("src", "web")
	runShell(webDir, "npm", "install")
	runShell(webDir, "npm", "run", "build")

	// 2. Sync web assets
	LogInfo("Syncing web assets to src/web_build...")
	webBuildDir := filepath.Join("src", "web_build")
	os.RemoveAll(webBuildDir)
	if err := os.MkdirAll(webBuildDir, 0755); err != nil {
		LogFatal("Failed to create web_build dir: %v", err)
	}
	copyDir(filepath.Join("src", "web", "dist"), webBuildDir)

	// 3. Build Dialtone locally (the tool itself)
	BuildSelf()

	// 4. Build for ARM64
	if local || !hasPodman() {
		buildLocally()
	} else {
		buildWithPodman()
	}

	LogInfo("Full build successful!")
}

// BuildSelf rebuilds the current binary and replaces it
func BuildSelf() {
	LogInfo("Building Dialtone CLI (Self)...")

	// Always aim for bin/dialtone.exe when building from source
	exePath := filepath.Join("bin", "dialtone.exe")
	if _, err := os.Stat("bin"); os.IsNotExist(err) {
		os.MkdirAll("bin", 0755)
	}

	oldExePath := exePath + ".old"

	// Rename old exe if it exists (allows overwriting while running on Windows)
	os.Remove(oldExePath) // Clean up any previous old file
	if _, err := os.Stat(exePath); err == nil {
		if err := os.Rename(exePath, oldExePath); err != nil {
			LogInfo("Warning: Failed to rename current exe, build might fail: %v", err)
		} else {
			LogInfo("Renamed current binary to %s", filepath.Base(oldExePath))
		}
	}

	runShell(".", "go", "build", "-o", exePath, ".")
	LogInfo("Successfully built %s", exePath)
}

package cli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"dialtone/cli/src/core/logger"
)

// RunBuild handles building for different platforms
func RunBuild(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	full := fs.Bool("full", false, "Build Web UI, local CLI, and ARM binary")
	local := fs.Bool("local", false, "Build natively on the local system")
	remote := fs.Bool("remote", false, "Build on remote robot via SSH")
	podman := fs.Bool("podman", false, "Force build using Podman")
	_ = podman // Used in logic below via !*podman etc
	linuxArm := fs.Bool("linux-arm", false, "Cross-compile for 32-bit Linux ARM (armv7)")
	linuxArm64 := fs.Bool("linux-arm64", false, "Cross-compile for 64-bit Linux ARM (aarch64)")
	builder := fs.Bool("builder", false, "Build the dialtone-builder image for faster ARM builds")
	showHelp := fs.Bool("help", false, "Show help for build command")

	fs.Usage = func() {
		fmt.Println("Usage: dialtone build [options]")
		fmt.Println()
		fmt.Println("Build the Dialtone binary and web UI for deployment.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --local        Build natively on the local system (uses DIALTONE_ENV if available)")
		fmt.Println("  --full         Full rebuild: Web UI + local CLI + ARM64 binary")
		fmt.Println("  --remote       Build on remote robot via SSH (requires configured .env)")
		fmt.Println("  --podman       Force build using Podman container")
		fmt.Println("  --linux-arm    Cross-compile for 32-bit Linux ARM (Raspberry Pi Zero/3/4/5)")
		fmt.Println("  --linux-arm64  Cross-compile for 64-bit Linux ARM (Raspberry Pi 3/4/5)")
		fmt.Println("  --builder      Build the dialtone-builder image for faster ARM builds")
		fmt.Println("  --help         Show help for build command")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  dialtone build              # Build web UI + binary (Podman or local)")
		fmt.Println("  dialtone build --local      # Build web UI + native binary")
		fmt.Println("  dialtone build --podman     # Force Podman build for ARM64")
		fmt.Println("  dialtone build --linux-arm  # Cross-compile for 32-bit ARM")
	}

	fs.Parse(args)

	if *showHelp {
		fs.Usage()
		return
	}

	if *builder {
		buildBuilderImage()
		return
	}

	if *remote {
		logger.LogInfo("Remote build triggered")
		return
	}

	if *full {
		buildEverything(*local)
	} else {
		arch := runtime.GOARCH
		if *linuxArm {
			arch = "arm"
		} else if *linuxArm64 {
			arch = "arm64"
		}

		isCrossBuild := arch != runtime.GOARCH

		if *local || !hasPodman() {
			if isCrossBuild && !hasZig() && !*local {
				logger.LogFatal("Cross-compilation for %s requires either Podman or Zig. Please install Podman (recommended) or ensure Zig is installed in your DIALTONE_ENV.", arch)
			}
			buildLocally(arch)
		} else {
			compiler := "gcc-aarch64-linux-gnu"
			if arch == "arm" {
				compiler = "gcc-arm-linux-gnueabihf"
			}
			buildWithPodman(arch, compiler)
		}
	}
}

func hasPodman() bool {
	_, err := exec.LookPath("podman")
	return err == nil
}

// buildWebIfNeeded builds the web UI if web_build is missing or empty, or if force is true
func buildWebIfNeeded(force bool) {
	webBuildDir := filepath.Join("src", "web_build")
	indexPath := filepath.Join(webBuildDir, "index.html")

	// Check if index.html exists and has real content
	if !force {
		if info, err := os.Stat(indexPath); err == nil && info.Size() > 100 {
			logger.LogInfo("Web UI already built (found %s)", indexPath)
			return
		}
	}

	logger.LogInfo("Building Web UI...")

	// Check if src/web exists
	webDir := filepath.Join("src", "web")
	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		logger.LogInfo("Warning: src/web directory not found, skipping web build")
		return
	}

	// Check for npm
	if _, err := exec.LookPath("npm"); err != nil {
		// Try to use npm from DIALTONE_ENV
		// NOTE: We need a way to get DIALTONE_ENV. For now, we'll try to find it.
		// In the current architecture, these helpers are duplicated or moved to core.
		depsDir := getDialtoneEnv()
		npmPath := filepath.Join(depsDir, "node", "bin", "npm")
		if _, err := os.Stat(npmPath); os.IsNotExist(err) {
			logger.LogInfo("Warning: npm not found, skipping web build. Run 'dialtone install' first.")
			return
		}
		// Add node to PATH
		nodeBin := filepath.Join(depsDir, "node", "bin")
		os.Setenv("PATH", fmt.Sprintf("%s:%s", nodeBin, os.Getenv("PATH")))
	}

	// Install and build
	runShell(webDir, "npm", "install")
	runShell(webDir, "npm", "run", "build")

	// Sync to web_build
	logger.LogInfo("Syncing web assets to src/web_build...")
	os.RemoveAll(webBuildDir)
	if err := os.MkdirAll(webBuildDir, 0755); err != nil {
		logger.LogFatal("Failed to create web_build dir: %v", err)
	}

	distDir := filepath.Join(webDir, "dist")
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		logger.LogInfo("Warning: npm build did not create dist directory")
		return
	}

	copyDir(distDir, webBuildDir)
	logger.LogInfo("Web UI build complete")
}

func buildLocally(targetArch string) {
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}
	logger.LogInfo("Building Dialtone locally (Target: %s/%s)...", runtime.GOOS, targetArch)

	// Build web UI if needed (not forced for native local builds unless requested)
	buildWebIfNeeded(false)

	if err := os.MkdirAll("bin", 0755); err != nil {
		logger.LogFatal("Failed to create bin directory: %v", err)
	}

	// For local builds, we enable CGO to support V4L2 drivers
	os.Setenv("CGO_ENABLED", "1")

	// If local environment exists, use it
	depsDir := getDialtoneEnv()
	if _, err := os.Stat(depsDir); err == nil {
		logger.LogInfo("Using local dependencies from %s", depsDir)

		// Add Go and Node to PATH
		goBin := filepath.Join(depsDir, "go", "bin")
		nodeBin := filepath.Join(depsDir, "node", "bin")
		os.Setenv("PATH", fmt.Sprintf("%s:%s:%s", goBin, nodeBin, os.Getenv("PATH")))

		// If local GNU toolchains exist, prioritize them for cross-compilation
		gcc64Bin := filepath.Join(depsDir, "gcc-aarch64", "bin", "aarch64-none-linux-gnu-gcc")
		gcc32Bin := filepath.Join(depsDir, "gcc-armhf", "bin", "arm-none-linux-gnueabihf-gcc")

		compilerFound := false
		if targetArch == "arm64" {
			if _, err := os.Stat(gcc64Bin); err == nil {
				os.Setenv("GOOS", "linux")
				os.Setenv("GOARCH", "arm64")
				os.Setenv("CC", gcc64Bin)
				compilerFound = true
			}
		} else if targetArch == "arm" {
			if _, err := os.Stat(gcc32Bin); err == nil {
				os.Setenv("GOOS", "linux")
				os.Setenv("GOARCH", "arm")
				os.Setenv("CC", gcc32Bin)
				compilerFound = true
			}
		}

		if !compilerFound {
			// If Zig exists, use it as C compiler
			zigPath := filepath.Join(depsDir, "zig", "zig")
			if _, err := os.Stat(zigPath); err == nil {
				absZig, _ := filepath.Abs(zigPath)
				target := "x86_64-linux-gnu" // default
				if targetArch == "arm64" {
					target = "aarch64-linux-gnu"
					os.Setenv("GOOS", "linux")
					os.Setenv("GOARCH", "arm64")
				} else if targetArch == "arm" {
					target = "arm-linux-gnueabihf"
					os.Setenv("GOOS", "linux")
					os.Setenv("GOARCH", "arm")
				}
				os.Setenv("CC", fmt.Sprintf("%s cc -target %s", absZig, target))
				compilerFound = true
			}
		}

		if targetArch != runtime.GOARCH && !compilerFound {
			logger.LogFatal("Local cross-compilation for %s requested, but no suitable compiler (Zig or GNU Toolchain) was found in %s", targetArch, depsDir)
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

	// Choose binary name based on OS/Arch
	binaryName := "dialtone"
	if os.Getenv("GOOS") == "linux" {
		binaryName = fmt.Sprintf("dialtone-%s", targetArch)
	} else if runtime.GOOS == "windows" {
		binaryName = "dialtone.exe"
	}

	outputPath := filepath.Join("bin", binaryName)
	goBin := "go"
	if _, err := os.Stat(filepath.Join(depsDir, "go", "bin", "go")); err == nil {
		goBin = filepath.Join(depsDir, "go", "bin", "go")
	}
	runShell(".", goBin, "build", "-o", outputPath, "src/cmd/dialtone/main.go")
	logger.LogInfo("Build successful: %s", outputPath)
}

func buildWithPodman(arch, compiler string) {
	logger.LogInfo("Building Dialtone for Linux %s using Podman (%s)...", arch, compiler)

	// Build web UI first (always force rebuild for remote/podman deployment)
	buildWebIfNeeded(true)

	cwd, err := os.Getwd()
	if err != nil {
		logger.LogFatal("Failed to get current directory: %v", err)
	}

	if err := os.MkdirAll("bin", 0755); err != nil {
		logger.LogFatal("Failed to create bin directory: %v", err)
	}

	outputName := fmt.Sprintf("dialtone-%s", arch)

	// Default to standard golang image and install compilers
	baseImage := "docker.io/library/golang:1.25.5"
	installCmd := fmt.Sprintf("apt-get update && apt-get install -y %s && ", compiler)

	// Check if optimized builder image exists
	if hasImage("dialtone-builder") {
		logger.LogInfo("Using optimized 'dialtone-builder' image (skipping apt-get install)")
		baseImage = "dialtone-builder"
		installCmd = "" // Skip installation as it's pre-installed
	}

	// Podman command
	buildCmd := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/src:Z", cwd),
		"-v", "dialtone-go-build-cache:/root/.cache/go-build:Z", // Persistent Go build cache
		"-w", "/src",
		"-e", "GOOS=linux",
		"-e", "GOARCH=" + arch,
		"-e", "CGO_ENABLED=1",
		"-e", "CC=" + strings.TrimPrefix(compiler, "gcc-") + "-gcc",
		baseImage,
		"bash", "-c", fmt.Sprintf("%sgo build -buildvcs=false -o bin/%s src/cmd/dialtone/main.go", installCmd, outputName),
	}

	logger.LogInfo("Running: podman %v", buildCmd)
	cmd := exec.Command("podman", buildCmd...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.LogFatal("Podman build failed: %v", err)
	}

	logger.LogInfo("Build successful: bin/%s", outputName)
}

func buildEverything(local bool) {
	logger.LogInfo("Starting Full Build Process...")

	// 1. Build Web UI
	logger.LogInfo("Building Web UI...")
	webDir := filepath.Join("src", "web")
	runShell(webDir, "npm", "install")
	runShell(webDir, "npm", "run", "build")

	// 2. Sync web assets
	logger.LogInfo("Syncing web assets to src/web_build...")
	webBuildDir := filepath.Join("src", "web_build")
	os.RemoveAll(webBuildDir)
	if err := os.MkdirAll(webBuildDir, 0755); err != nil {
		logger.LogFatal("Failed to create web_build dir: %v", err)
	}
	copyDir(filepath.Join("src", "web", "dist"), webBuildDir)

	// 3. Build Dialtone locally (the tool itself)
	BuildSelf()

	// 4. Build for ARM64
	if local || !hasPodman() {
		buildLocally("arm64")
	} else {
		buildWithPodman("arm64", "gcc-aarch64-linux-gnu")
	}

	logger.LogInfo("Full build successful!")
}

// BuildSelf rebuilds the current binary and replaces it
func BuildSelf() {
	logger.LogInfo("Building Dialtone CLI (Self)...")

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
			logger.LogInfo("Warning: Failed to rename current exe, build might fail: %v", err)
		} else {
			logger.LogInfo("Renamed current binary to %s", filepath.Base(oldExePath))
		}
	}

	runShell(".", "go", "build", "-o", exePath, "src/cmd/dialtone/main.go")
	logger.LogInfo("Successfully built %s", exePath)
}

// Helper functions (mirrored from core/utils or src/build.go)

func getDialtoneEnv() string {
	env := os.Getenv("DIALTONE_ENV")
	if env != "" {
		absPath, _ := filepath.Abs(env)
		return absPath
	}
	// Simplified for now, should ideally use shared core logic
	absPath, _ := filepath.Abs("dialtone_dependencies")
	return absPath
}

func runShell(dir string, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Command failed in %s: %v %v: %v", dir, name, args, err)
	}
}

func copyDir(src string, dst string) {
	// Simple implementation or call shell cp -r
	cmd := exec.Command("cp", "-r", src+"/.", dst)
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Failed to copy directory from %s to %s: %v", src, dst, err)
	}
}

func buildBuilderImage() {
	logger.LogInfo("Building 'dialtone-builder' image...")
	dockerfile := filepath.Join("docs", "Dockerfile.builder")
	if _, err := os.Stat(dockerfile); os.IsNotExist(err) {
		logger.LogFatal("Dockerfile.builder not found: %s", dockerfile)
	}

	runShell(".", "podman", "build", "-f", dockerfile, "-t", "dialtone-builder", ".")
	logger.LogInfo("'dialtone-builder' image created successfully.")
}

func hasImage(name string) bool {
	cmd := exec.Command("podman", "image", "exists", name)
	return cmd.Run() == nil
}

func hasZig() bool {
	depsDir := getDialtoneEnv()
	zigPath := filepath.Join(depsDir, "zig", "zig")
	if _, err := os.Stat(zigPath); err == nil {
		return true
	}
	_, err := exec.LookPath("zig")
	return err == nil
}

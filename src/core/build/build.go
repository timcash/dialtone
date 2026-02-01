package build

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
		targetOS := runtime.GOOS
		arch := runtime.GOARCH
		if *linuxArm {
			arch = "arm"
			targetOS = "linux"
		} else if *linuxArm64 {
			arch = "arm64"
			targetOS = "linux"
		}

		isCrossBuild := arch != runtime.GOARCH || targetOS != runtime.GOOS

		if *local || !hasPodman() {
			if isCrossBuild && !hasZig() && !*local {
				logger.LogFatal("Cross-compilation for %s/%s requires either Podman or Zig. Please install Podman (recommended) or ensure Zig is installed in your DIALTONE_ENV.", targetOS, arch)
			}
			buildLocally(targetOS, arch)
		} else {
			compiler := "gcc-aarch64-linux-gnu"
			cppCompiler := "g++-aarch64-linux-gnu"
			if arch == "arm" {
				compiler = "gcc-arm-linux-gnueabihf"
				cppCompiler = "g++-arm-linux-gnueabihf"
			}
			buildWithPodman(arch, compiler, cppCompiler)
		}
	}
}

func hasPodman() bool {
	_, err := exec.LookPath("podman")
	return err == nil
}

// buildWebIfNeeded builds the web UI if needed
func buildWebIfNeeded(force bool) {
	distIndexPath := filepath.Join("src", "core", "web", "dist", "index.html")

	// If not forcing, check if it already exists
	if !force {
		if info, err := os.Stat(distIndexPath); err == nil && info.Size() > 100 {
			logger.LogInfo("Web UI already built (found %s)", distIndexPath)
			return
		}
	}

	logger.LogInfo("Building Web UI (force=%v)...", force)

	// Check if src/core/web exists
	webDir := filepath.Join("src", "core", "web")
	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		logger.LogInfo("Warning: src/core/web directory not found, skipping web build")
		return
	}

	// NOTE: We do NOT RemoveAll(dist) here because it causes bootstrapping failures. 
	// If the dist directory is deleted, the 'dialtone' tool itself cannot be re-compiled 
	// by 'go run' during the build process because of the //go:embed pattern in dialtone.go.

	// Install and build via UI plugin (shell delegation for decoupling)
	logger.LogInfo("Delegating to UI plugin (install)...")
	runShell(".", "./dialtone.sh", "ui", "install")

	logger.LogInfo("Delegating to UI plugin (build)...")
	runShell(".", "./dialtone.sh", "ui", "build")

	// Verify build succeeded
	if info, err := os.Stat(distIndexPath); os.IsNotExist(err) {
		logger.LogFatal("Web UI build failed: %s not found after build", distIndexPath)
	} else {
		logger.LogInfo("Web UI build complete (size: %d bytes)", info.Size())
	}
}

func buildLocally(targetOS, targetArch string) {
	if targetOS == "" {
		targetOS = runtime.GOOS
	}
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}
	logger.LogInfo("Building Dialtone locally (Target: %s/%s)...", targetOS, targetArch)

	// Build web UI if needed (not forced for native local builds unless requested)
	buildWebIfNeeded(false)

	if err := os.MkdirAll("bin", 0755); err != nil {
		logger.LogFatal("Failed to create bin directory: %v", err)
	}

	// For local builds, we enable CGO to support V4L2 drivers
	os.Setenv("CGO_ENABLED", "1")

	// If local environment exists, use it
	depsDir := getDialtoneEnv()
	tags := []string{}

	if _, err := os.Stat(depsDir); err == nil {
		logger.LogInfo("Using local dependencies from %s", depsDir)

		// Prepend dependencies to PATH (Go, Node, Zig, Pixi, GH)
		paths := []string{
			filepath.Join(depsDir, "go", "bin"),
			filepath.Join(depsDir, "node", "bin"),
			filepath.Join(depsDir, "zig"),
			filepath.Join(depsDir, "gh", "bin"),
			filepath.Join(depsDir, "pixi"),
		}
		newPath := strings.Join(paths, string(os.PathListSeparator)) + string(os.PathListSeparator) + os.Getenv("PATH")
		os.Setenv("PATH", newPath)

		compilerFound := false
		nativeDarwin := targetOS == "darwin" && targetArch == runtime.GOARCH

		// Prioritize Zig for cross-compilation to allow specific GLIBC targeting
		isCrossBuild := targetArch != runtime.GOARCH || targetOS != runtime.GOOS
		if isCrossBuild && !nativeDarwin {
			zigPath := filepath.Join(depsDir, "zig", "zig")
			if _, err := os.Stat(zigPath); err == nil {
				zOS := targetOS
				if zOS == "darwin" {
					zOS = "macos"
				}

				zArch := targetArch
				if zArch == "amd64" {
					zArch = "x86_64"
				} else if zArch == "arm64" {
					zArch = "aarch64"
				}

				target := fmt.Sprintf("%s-%s", zArch, zOS)
				if zOS == "linux" {
					// Target GLIBC 2.36 for compatibility with Debian Bookworm (stable)
					target += "-gnu.2.36"
					// Exclude DuckDB and related tools for robot build to avoid C++ linking issues
					tags = append(tags, "no_duckdb")
				}

				// Configure Zig as CC/CXX
				os.Setenv("CC", fmt.Sprintf("%s cc -target %s", zigPath, target))
				os.Setenv("CXX", fmt.Sprintf("%s c++ -target %s", zigPath, target))

				// Set Zig cache directories to ensure they are writable in this environment
				zigCache := filepath.Join(depsDir, "zig-cache")
				os.MkdirAll(zigCache, 0755)
				os.Setenv("ZIG_LOCAL_CACHE_DIR", filepath.Join(zigCache, "local"))
				os.Setenv("ZIG_GLOBAL_CACHE_DIR", filepath.Join(zigCache, "global"))

				compilerFound = true
				logger.LogInfo("Using Zig as cross-compiler (Target: %s, Tags: %v)", target, tags)
			}
		}

		// Fallback to GNU toolchain if Zig not used or not found
		if !compilerFound {
			gcc64Bin := filepath.Join(depsDir, "gcc-aarch64", "bin", "aarch64-none-linux-gnu-gcc")
			gcc32Bin := filepath.Join(depsDir, "gcc-armhf", "bin", "arm-none-linux-gnueabihf-gcc")

			if targetArch == "arm64" {
				if _, err := os.Stat(gcc64Bin); err == nil {
					os.Setenv("GOOS", "linux")
					os.Setenv("GOARCH", "arm64")
					os.Setenv("CC", gcc64Bin)
					compilerFound = true
					logger.LogInfo("Using GNU aarch64 toolchain")
				}
			} else if targetArch == "arm" {
				if _, err := os.Stat(gcc32Bin); err == nil {
					os.Setenv("GOOS", "linux")
					os.Setenv("GOARCH", "arm")
					os.Setenv("CC", gcc32Bin)
					compilerFound = true
					logger.LogInfo("Using GNU armhf toolchain")
				}
			}
		}

		if !compilerFound && targetOS == runtime.GOOS && targetArch == runtime.GOARCH {
			// For native builds, if no compiler was found in env (or we skipped Zig for Darwin), let Go find the system compiler
			logger.LogInfo("Using system compiler for native build.")
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
	if targetOS == "linux" && targetArch != runtime.GOARCH {
		binaryName = fmt.Sprintf("dialtone-%s", targetArch)
	} else if targetOS == "linux" {
		binaryName = "dialtone"
	} else if targetOS == "windows" {
		binaryName = "dialtone.exe"
	}

	outputPath := filepath.Join("bin", binaryName)
	goBin := "go"
	if _, err := os.Stat(filepath.Join(depsDir, "go", "bin", "go")); err == nil {
		goBin = filepath.Join(depsDir, "go", "bin", "go")
	}

	// Set environment for build
	os.Setenv("GOOS", targetOS)
	os.Setenv("GOARCH", targetArch)

	buildArgs := []string{"build"}
	if len(tags) > 0 {
		buildArgs = append(buildArgs, "-tags", strings.Join(tags, ","))
	}
	buildArgs = append(buildArgs, "-o", outputPath, "src/cmd/dialtone/main.go")

	runShell(".", goBin, buildArgs...)
	logger.LogInfo("Build successful: %s", outputPath)
}

func buildWithPodman(arch, compiler, cppCompiler string) {
	logger.LogInfo("Building Dialtone for Linux %s using Podman (%s, %s)...", arch, compiler, cppCompiler)

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
	installCmd := fmt.Sprintf("apt-get update && apt-get install -y %s %s && ", compiler, cppCompiler)

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
		"-e", "CXX=" + strings.TrimPrefix(cppCompiler, "g++-") + "-g++",
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
	logger.LogInfo("Building Web UI via UI Plugin...")
	buildWebIfNeeded(true)

	// 3. Build AI components (shell delegation for decoupling)
	runShell(".", "./dialtone.sh", "ai", "build")

	// 4. Build Dialtone locally (the tool itself)
	BuildSelf()

	// 5. Build for ARM64
	if local || !hasPodman() {
		buildLocally("linux", "arm64")
	} else {
		buildWithPodman("arm64", "gcc-aarch64-linux-gnu", "g++-aarch64-linux-gnu")
	}

	logger.LogInfo("Full build successful!")
}

// BuildSelf rebuilds the current binary and replaces it
func BuildSelf() {
	logger.LogInfo("Building Dialtone CLI (Self)...")

	binaryName := "dialtone"
	if runtime.GOOS == "windows" {
		binaryName = "dialtone.exe"
	}

	if _, err := os.Stat("bin"); os.IsNotExist(err) {
		os.MkdirAll("bin", 0755)
	}

	outputPath := filepath.Join("bin", binaryName)

	// Force clean cache to avoid embed issues
	runShell(".", "go", "clean", "-cache")
	runShell(".", "go", "build", "-o", outputPath, "src/cmd/dialtone/main.go")
	logger.LogInfo("Successfully built %s", outputPath)
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

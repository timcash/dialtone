package cli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"dialtone/cli/src/core/logger"

	"golang.org/x/crypto/ssh"
)

const (
	GoVersion          = "1.25.5"
	NodeVersion        = "22.13.0"
	ZigVersion         = "0.13.0"
	GHVersion          = "2.66.1"
	PixiVersion        = "latest" // Using latest for pixi
	PodmanVersion      = "latest"
	ArmCompilerVersion = "13.3.rel1"
	Arm64CompilerUrl   = "https://developer.arm.com/-/media/Files/downloads/gnu/13.3.rel1/binrel/arm-gnu-toolchain-13.3.rel1-x86_64-aarch64-none-linux-gnu.tar.xz"
	ArmhfCompilerUrl   = "https://developer.arm.com/-/media/Files/downloads/gnu/13.3.rel1/binrel/arm-gnu-toolchain-13.3.rel1-x86_64-arm-none-linux-gnueabihf.tar.xz"
)

func logItemStatus(name, version, path string, alreadyInstalled bool) {
	status := "installed successfully"
	if alreadyInstalled {
		status = "is already installed"
	}
	logger.LogInfo("%s (%s) %s at %s", name, version, status, path)
}

// GetDialtoneEnv returns the directory where dependencies are installed.
func GetDialtoneEnv() string {
	env := os.Getenv("DIALTONE_ENV")
	if env != "" {
		if strings.HasPrefix(env, "~") {
			home, _ := os.UserHomeDir()
			env = filepath.Join(home, env[1:])
		}
		absEnv, _ := filepath.Abs(env)
		return absEnv
	}
	cwd, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			localPath := filepath.Join(cwd, "dialtone_dependencies")
			if _, err := os.Stat(localPath); err == nil {
				logger.LogInfo("DIALTONE_ENV not set, using repo-local path: %s", localPath)
				absPath, _ := filepath.Abs(localPath)
				return absPath
			}
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}
	home, _ := os.UserHomeDir()
	defaultPath := filepath.Join(home, ".dialtone_env")
	logger.LogInfo("DIALTONE_ENV not set, using default path: %s", defaultPath)
	absPath, _ := filepath.Abs(defaultPath)
	return absPath
}

func runSimpleShell(command string) {
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Command failed: %v", command, err)
	}
}

// RunInstall handles the 'install' command
func RunInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	linuxWSL := fs.Bool("linux-wsl", false, "Install dependencies natively on Linux/WSL (x86_64)")
	macosARM := fs.Bool("macos-arm", false, "Install dependencies natively on macOS ARM (Apple Silicon)")
	clean := fs.Bool("clean", false, "Remove all dependencies before installation")
	check := fs.Bool("check", false, "Check if dependencies are installed and exit")
	showHelp := fs.Bool("help", false, "Show help for install command")

	fs.Usage = func() {
		fmt.Println("Usage: dialtone install [options] [install-path]")
		fmt.Println()
		fmt.Println("Install development dependencies (Go, Node.js, Zig, GH CLI, Pixi) for building Dialtone.")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  [install-path]  Optional: Path where dependencies should be installed.")
		fmt.Println("                  Overrides DIALTONE_ENV and default locations.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --linux-wsl   Install for Linux/WSL x86_64")
		fmt.Println("  --macos-arm   Install for macOS ARM (Apple Silicon)")
		fmt.Println("  --host        SSH host for remote installation (user@host)")
		fmt.Println("  --port        SSH port (default: 22)")
		fmt.Println("  --user        SSH username")
		fmt.Println("  --pass        SSH password")
		fmt.Println("  --clean       Remove all dependencies before installation")
		fmt.Println("  --check       Check if dependencies are installed and exit")
		fmt.Println("  --help        Show this help message")
		fmt.Println()
		fmt.Println("Notes:")
		fmt.Println("  - Dependencies are installed to the directory specified by DIALTONE_ENV")
		fmt.Println("  - Default location is ./dialtone_dependencies (relative to repo) or ~/.dialtone_env")
	}

	// Handle flags
	var positionalArgs []string
	var flagArgs []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
		} else {
			positionalArgs = append(positionalArgs, arg)
		}
	}

	fs.Parse(flagArgs)

	depsDir := GetDialtoneEnv()

	if *check {
		CheckInstall(depsDir)
		return
	}

	// Handle positional argument for install path
	if len(positionalArgs) > 0 {
		installPath := positionalArgs[0]
		os.Setenv("DIALTONE_ENV", installPath)
		logger.LogInfo("Using environment directory from argument: %s", installPath)
	} else if env := os.Getenv("DIALTONE_ENV"); env != "" {
		logger.LogInfo("Using environment directory from DIALTONE_ENV: %s", env)
	}

	// Handle clean option
	if *clean {
		depsDir := GetDialtoneEnv()
		if _, err := os.Stat(depsDir); err == nil {
			logger.LogInfo("Cleaning dependencies directory: %s", depsDir)
			if err := os.RemoveAll(depsDir); err != nil {
				logger.LogFatal("Failed to remove dependencies directory: %v", err)
			}
			logger.LogInfo("Successfully removed %s", depsDir)
		} else {
			logger.LogInfo("Dependencies directory %s does not exist, nothing to clean", depsDir)
		}
	}

	if *showHelp {
		fs.Usage()
		return
	}

	// Explicit flags take priority
	if *linuxWSL {
		installLocalDepsWSL()
		return
	}

	if *macosARM {
		installLocalDepsMacOSARM()
		return
	}

	// Determine if host was explicitly set via flag
	hostSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "host" {
			hostSet = true
		}
	})

	// If no host specified via flag, auto-detect local OS/arch
	if !hostSet {
		installLocalAuto()
		return
	}

	// Remote install path
	if *host == "" || *pass == "" {
		logger.LogFatal("Error: -host (user@host) and -pass are required for remote install")
	}

	client, err := dialSSH(*host, *port, *user, *pass)
	if err != nil {
		logger.LogFatal("SSH connection failed: %v", err)
	}
	defer client.Close()

	logger.LogInfo("Installing dependencies on %s...", *host)

	// Install Go (Remote)
	goTarball := fmt.Sprintf("go%s.linux-arm64.tar.gz", GoVersion)
	installGoCmd := fmt.Sprintf(`
		if ! command -v go &> /dev/null; then
			echo "Installing Go %s..."
			wget https://go.dev/dl/%s
			echo "%s" | sudo -S rm -rf /usr/local/go
			echo "%s" | sudo -S tar -C /usr/local -xzf %s
			rm %s
			echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
			echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
		else
			echo "Go is already installed."
		fi
	`, GoVersion, goTarball, *pass, *pass, goTarball, goTarball)

	output, err := runSSHCommand(client, installGoCmd)
	if err != nil {
		logger.LogFatal("Failed to install Go: %v\nOutput: %s", err, output)
	}
	logger.LogInfo(output)

	// Install Node.js (Remote)
	installNodeCmd := fmt.Sprintf(`
		if ! command -v node &> /dev/null; then
			echo "Installing Node.js..."
			curl -fsSL https://deb.nodesource.com/setup_20.x | echo "%s" | sudo -S -E bash -
			echo "%s" | sudo -S apt-get install -y nodejs
		else
			echo "Node.js is already installed."
		fi
	`, *pass, *pass)
	output, err = runSSHCommand(client, installNodeCmd)
	if err != nil {
		logger.LogFatal("Failed to install Node.js: %v\nOutput: %s", err, output)
	}
	logger.LogInfo(output)
}

func installLocalAuto() {
	logger.LogInfo("Auto-detecting system: %s/%s", runtime.GOOS, runtime.GOARCH)
	switch {
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		installLocalDepsMacOSARM()
	case runtime.GOOS == "darwin" && runtime.GOARCH == "amd64":
		logger.LogInfo("macOS x86_64 detected. Installing with Rosetta-compatible deps...")
		installLocalDepsMacOSAMD64()
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		installLocalDepsWSL()
	case runtime.GOOS == "linux" && runtime.GOARCH == "arm64":
		logger.LogInfo("Linux ARM64 detected (likely Raspberry Pi).")
		installLocalDepsLinuxARM64()
	default:
		logger.LogFatal("Unsupported platform: %s/%s. Use --linux-wsl or --macos-arm explicitly.", runtime.GOOS, runtime.GOARCH)
	}
}

func installLocalDepsWSL() {
	logger.LogInfo("Installing local dependencies for Linux/WSL (User-Local, No Sudo)...")
	depsDir := GetDialtoneEnv()
	os.MkdirAll(depsDir, 0755)

	// 1. Install Go
	goDir := filepath.Join(depsDir, "go")
	goBin := filepath.Join(goDir, "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		logger.LogInfo("Step 1: Installing Go %s...", GoVersion)
		goTarball := fmt.Sprintf("go%s.linux-amd64.tar.gz", GoVersion)
		downloadUrl := fmt.Sprintf("https://go.dev/dl/%s", goTarball)
		runSimpleShell(fmt.Sprintf("wget -O %s/%s %s", depsDir, goTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("tar -C %s -xzf %s/%s", depsDir, depsDir, goTarball))
		os.Remove(filepath.Join(depsDir, goTarball))
		logItemStatus("Go", GoVersion, goBin, false)
	} else {
		logItemStatus("Go", GoVersion, goBin, true)
	}

	// 2. Install Node.js
	nodeDir := filepath.Join(depsDir, "node")
	nodeBin := filepath.Join(nodeDir, "bin", "node")
	if _, err := os.Stat(nodeBin); err != nil {
		logger.LogInfo("Step 2: Installing Node.js %s...", NodeVersion)
		nodeTarball := fmt.Sprintf("node-v%s-linux-x64.tar.xz", NodeVersion)
		downloadUrl := fmt.Sprintf("https://nodejs.org/dist/v%s/%s", NodeVersion, nodeTarball)
		runSimpleShell(fmt.Sprintf("wget -q -O %s/%s %s", depsDir, nodeTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", nodeDir, nodeDir, depsDir, nodeTarball))
		os.Remove(filepath.Join(depsDir, nodeTarball))
		logItemStatus("Node.js", NodeVersion, nodeBin, false)
	} else {
		logItemStatus("Node.js", NodeVersion, nodeBin, true)
	}

	// 2.1 Install Vercel CLI
	vercelBin := filepath.Join(nodeDir, "bin", "vercel")
	if _, err := os.Stat(vercelBin); err != nil {
		logger.LogInfo("Step 2.1: Installing Vercel CLI...")
		runSimpleShell(fmt.Sprintf("%s/bin/npm install -g --prefix %s vercel", nodeDir, nodeDir))
		logItemStatus("Vercel CLI", "latest", vercelBin, false)
	} else {
		logItemStatus("Vercel CLI", "latest", vercelBin, true)
	}

	// 2.2 Install GitHub CLI
	ghDir := filepath.Join(depsDir, "gh")
	ghBin := filepath.Join(ghDir, "bin", "gh")
	if _, err := os.Stat(ghBin); err != nil {
		logger.LogInfo("Step 2.2: Installing GitHub CLI %s...", GHVersion)
		ghTarball := fmt.Sprintf("gh_%s_linux_amd64.tar.gz", GHVersion)
		downloadUrl := fmt.Sprintf("https://github.com/cli/cli/releases/download/v%s/%s", GHVersion, ghTarball)
		runSimpleShell(fmt.Sprintf("wget -q -O %s/%s %s", depsDir, ghTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xzf %s/%s", ghDir, ghDir, depsDir, ghTarball))
		os.Remove(filepath.Join(depsDir, ghTarball))
		logItemStatus("GitHub CLI", GHVersion, ghBin, false)
	} else {
		logItemStatus("GitHub CLI", GHVersion, ghBin, true)
	}

	// 2.3 Install Pixi
	pixiDir := filepath.Join(depsDir, "pixi")
	pixiBin := filepath.Join(pixiDir, "pixi")
	if _, err := os.Stat(pixiBin); err != nil {
		logger.LogInfo("Step 2.3: Installing Pixi %s...", PixiVersion)
		downloadUrl := "https://github.com/prefix-dev/pixi/releases/latest/download/pixi-x86_64-unknown-linux-musl"
		runSimpleShell(fmt.Sprintf("mkdir -p %s && wget -q -O %s %s && chmod +x %s", pixiDir, pixiBin, downloadUrl, pixiBin))
		logItemStatus("Pixi", PixiVersion, pixiBin, false)
	} else {
		logItemStatus("Pixi", PixiVersion, pixiBin, true)
	}

	// 2.5 Install Zig
	zigDir := filepath.Join(depsDir, "zig")
	zigBin := filepath.Join(zigDir, "zig")
	if _, err := os.Stat(zigBin); err != nil {
		logger.LogInfo("Step 2.5: Installing Zig %s...", ZigVersion)
		zigTarball := fmt.Sprintf("zig-linux-x86_64-%s.tar.xz", ZigVersion)
		downloadUrl := fmt.Sprintf("https://ziglang.org/download/%s/%s", ZigVersion, zigTarball)
		runSimpleShell(fmt.Sprintf("wget -q -O %s/%s %s", depsDir, zigTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", zigDir, zigDir, depsDir, zigTarball))
		os.Remove(filepath.Join(depsDir, zigTarball))
		logItemStatus("Zig", ZigVersion, zigBin, false)
	} else {
		logItemStatus("Zig", ZigVersion, zigBin, true)
	}

	// 3. Install V4L2 headers
	includeDir := filepath.Join(depsDir, "usr", "include")
	headerFile := filepath.Join(includeDir, "linux", "videodev2.h")
	if _, err := os.Stat(headerFile); err != nil {
		logger.LogInfo("Step 3: Extracting V4L2 headers...")
		err := os.Chdir(depsDir)
		if err == nil {
			cmd := exec.Command("apt-get", "download", "libv4l-dev", "linux-libc-dev")
			if cmd.Run() != nil {
				runSimpleShell("wget -q http://archive.ubuntu.com/ubuntu/pool/main/v/v4l-utils/libv4l-dev_1.26.1-4build3_amd64.deb")
				runSimpleShell("wget -q http://archive.ubuntu.com/ubuntu/pool/main/l/linux/linux-libc-dev_6.8.0-31.31_amd64.deb")
			}
			runSimpleShell("dpkg -x libv4l-dev*.deb .")
			runSimpleShell("dpkg -x linux-libc-dev*.deb .")
			runSimpleShell("rm *.deb")
			home, _ := os.UserHomeDir()
			os.Chdir(home)
			logItemStatus("V4L2 Headers", "latest", headerFile, false)
		}
	} else {
		logItemStatus("V4L2 Headers", "latest", headerFile, true)
	}

	// 4. Install Cross-Compilation Tools (Local)
	if runtime.GOOS == "linux" {
		logger.LogInfo("Step 4: Installing ARM Cross-Compilation Toolchains locally...")

		// AArch64 Compiler
		gcc64Dir := filepath.Join(depsDir, "gcc-aarch64")
		gcc64Bin := filepath.Join(gcc64Dir, "bin", "aarch64-none-linux-gnu-gcc")
		if _, err := os.Stat(gcc64Bin); err != nil {
			logger.LogInfo("Step 4.1: Installing AArch64 Compiler %s...", ArmCompilerVersion)
			tarball := "gcc-aarch64.tar.xz"
			runSimpleShell(fmt.Sprintf("wget -q -O %s/%s %s", depsDir, tarball, Arm64CompilerUrl))
			runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", gcc64Dir, gcc64Dir, depsDir, tarball))
			os.Remove(filepath.Join(depsDir, tarball))
			logItemStatus("AArch64 Compiler", ArmCompilerVersion, gcc64Bin, false)
		} else {
			logItemStatus("AArch64 Compiler", ArmCompilerVersion, gcc64Bin, true)
		}

		// ARMhf Compiler
		gcc32Dir := filepath.Join(depsDir, "gcc-armhf")
		gcc32Bin := filepath.Join(gcc32Dir, "bin", "arm-none-linux-gnueabihf-gcc")
		if _, err := os.Stat(gcc32Bin); err != nil {
			logger.LogInfo("Step 4.2: Installing ARMhf Compiler %s...", ArmCompilerVersion)
			tarball := "gcc-armhf.tar.xz"
			runSimpleShell(fmt.Sprintf("wget -q -O %s/%s %s", depsDir, tarball, ArmhfCompilerUrl))
			runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", gcc32Dir, gcc32Dir, depsDir, tarball))
			os.Remove(filepath.Join(depsDir, tarball))
			logItemStatus("ARMhf Compiler", ArmCompilerVersion, gcc32Bin, false)
		} else {
			logItemStatus("ARMhf Compiler", ArmCompilerVersion, gcc32Bin, true)
		}

		// 5. Check for Podman
		if _, err := exec.LookPath("podman"); err != nil {
			logger.LogInfo("Step 5: Podman not found. Note: Local rootless Podman installation on WSL requires system-level setup. Please install it manually if needed: 'sudo apt-get install podman'")
		} else {
			logger.LogInfo("Step 5: Podman is already installed on the system.")
		}
	}

	printInstallComplete(depsDir)
}

func installLocalDepsMacOSAMD64() {
	logger.LogInfo("Installing local dependencies for macOS x86_64 (Intel)...")
	depsDir := GetDialtoneEnv()
	os.MkdirAll(depsDir, 0755)

	// 1. Install Go
	goDir := filepath.Join(depsDir, "go")
	goBin := filepath.Join(goDir, "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		logger.LogInfo("Step 1: Installing Go %s for macOS x86_64...", GoVersion)
		goTarball := fmt.Sprintf("go%s.darwin-amd64.tar.gz", GoVersion)
		downloadUrl := fmt.Sprintf("https://go.dev/dl/%s", goTarball)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/%s %s", depsDir, goTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("tar -C %s -xzf %s/%s", depsDir, depsDir, goTarball))
		os.Remove(filepath.Join(depsDir, goTarball))
		logItemStatus("Go", GoVersion, goBin, false)
	} else {
		logItemStatus("Go", GoVersion, goBin, true)
	}

	// 2. Install Node.js
	nodeDir := filepath.Join(depsDir, "node")
	nodeBin := filepath.Join(nodeDir, "bin", "node")
	if _, err := os.Stat(nodeBin); err != nil {
		logger.LogInfo("Step 2: Installing Node.js %s for macOS x86_64...", NodeVersion)
		nodeTarball := fmt.Sprintf("node-v%s-darwin-x64.tar.gz", NodeVersion)
		downloadUrl := fmt.Sprintf("https://nodejs.org/dist/v%s/%s", NodeVersion, nodeTarball)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/%s %s", depsDir, nodeTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xzf %s/%s", nodeDir, nodeDir, depsDir, nodeTarball))
		os.Remove(filepath.Join(depsDir, nodeTarball))
		logItemStatus("Node.js", NodeVersion, nodeBin, false)
	} else {
		logItemStatus("Node.js", NodeVersion, nodeBin, true)
	}

	// 2.2 Install GitHub CLI
	ghDir := filepath.Join(depsDir, "gh")
	ghBin := filepath.Join(ghDir, "bin", "gh")
	if _, err := os.Stat(ghBin); err != nil {
		logger.LogInfo("Step 2.2: Installing GitHub CLI %s...", GHVersion)
		ghZip := fmt.Sprintf("gh_%s_macOS_amd64.zip", GHVersion)
		downloadUrl := fmt.Sprintf("https://github.com/cli/cli/releases/download/v%s/%s", GHVersion, ghZip)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/%s %s", depsDir, ghZip, downloadUrl))
		runSimpleShell(fmt.Sprintf("unzip -q %s/%s -d %s && mv %s/gh_%s_macOS_amd64/* %s/ && rm -rf %s/gh_%s_macOS_amd64", depsDir, ghZip, ghDir, ghDir, GHVersion, ghDir, ghDir, GHVersion))
		os.Remove(filepath.Join(depsDir, ghZip))
		logItemStatus("GitHub CLI", GHVersion, ghBin, false)
	} else {
		logItemStatus("GitHub CLI", GHVersion, ghBin, true)
	}

	// 2.3 Install Pixi
	pixiDir := filepath.Join(depsDir, "pixi")
	pixiBin := filepath.Join(pixiDir, "pixi")
	if _, err := os.Stat(pixiBin); err != nil {
		logger.LogInfo("Step 2.3: Installing Pixi %s...", PixiVersion)
		downloadUrl := "https://github.com/prefix-dev/pixi/releases/latest/download/pixi-x86_64-apple-darwin"
		runSimpleShell(fmt.Sprintf("mkdir -p %s && curl -L -o %s %s && chmod +x %s", pixiDir, pixiBin, downloadUrl, pixiBin))
		logItemStatus("Pixi", PixiVersion, pixiBin, false)
	} else {
		logItemStatus("Pixi", PixiVersion, pixiBin, true)
	}

	// 3. Install Zig
	zigDir := filepath.Join(depsDir, "zig")
	zigBin := filepath.Join(zigDir, "zig")
	if _, err := os.Stat(zigBin); err != nil {
		logger.LogInfo("Step 3: Installing Zig %s for macOS x86_64...", ZigVersion)
		zigTarball := fmt.Sprintf("zig-macos-x86_64-%s.tar.xz", ZigVersion)
		downloadUrl := fmt.Sprintf("https://ziglang.org/download/%s/%s", ZigVersion, zigTarball)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/%s %s", depsDir, zigTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", zigDir, zigDir, depsDir, zigTarball))
		os.Remove(filepath.Join(depsDir, zigTarball))
		logItemStatus("Zig", ZigVersion, zigBin, false)
	} else {
		logItemStatus("Zig", ZigVersion, zigBin, true)
	}

	printInstallComplete(depsDir)
}

func installLocalDepsLinuxARM64() {
	logger.LogInfo("Installing local dependencies for Linux ARM64...")
	depsDir := GetDialtoneEnv()
	os.MkdirAll(depsDir, 0755)

	// 1. Install Go
	goDir := filepath.Join(depsDir, "go")
	goBin := filepath.Join(goDir, "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		logger.LogInfo("Step 1: Installing Go %s for Linux ARM64...", GoVersion)
		goTarball := fmt.Sprintf("go%s.linux-arm64.tar.gz", GoVersion)
		downloadUrl := fmt.Sprintf("https://go.dev/dl/%s", goTarball)
		runSimpleShell(fmt.Sprintf("wget -O %s/%s %s", depsDir, goTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("tar -C %s -xzf %s/%s", depsDir, depsDir, goTarball))
		os.Remove(filepath.Join(depsDir, goTarball))
		logItemStatus("Go", GoVersion, goBin, false)
	} else {
		logItemStatus("Go", GoVersion, goBin, true)
	}

	// 2. Install Node.js
	nodeDir := filepath.Join(depsDir, "node")
	nodeBin := filepath.Join(nodeDir, "bin", "node")
	if _, err := os.Stat(nodeBin); err != nil {
		logger.LogInfo("Step 2: Installing Node.js %s for Linux ARM64...", NodeVersion)
		nodeTarball := fmt.Sprintf("node-v%s-linux-arm64.tar.xz", NodeVersion)
		downloadUrl := fmt.Sprintf("https://nodejs.org/dist/v%s/%s", NodeVersion, nodeTarball)
		runSimpleShell(fmt.Sprintf("wget -O %s/%s %s", depsDir, nodeTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", nodeDir, nodeDir, depsDir, nodeTarball))
		os.Remove(filepath.Join(depsDir, nodeTarball))
		logItemStatus("Node.js", NodeVersion, nodeBin, false)
	} else {
		logItemStatus("Node.js", NodeVersion, nodeBin, true)
	}

	// 2.2 Install GitHub CLI
	ghDir := filepath.Join(depsDir, "gh")
	ghBin := filepath.Join(ghDir, "bin", "gh")
	if _, err := os.Stat(ghBin); err != nil {
		logger.LogInfo("Step 2.2: Installing GitHub CLI %s for ARM64...", GHVersion)
		ghTarball := fmt.Sprintf("gh_%s_linux_arm64.tar.gz", GHVersion)
		downloadUrl := fmt.Sprintf("https://github.com/cli/cli/releases/download/v%s/%s", GHVersion, ghTarball)
		runSimpleShell(fmt.Sprintf("wget -q -O %s/%s %s", depsDir, ghTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xzf %s/%s", ghDir, ghDir, depsDir, ghTarball))
		os.Remove(filepath.Join(depsDir, ghTarball))
		logItemStatus("GitHub CLI", GHVersion, ghBin, false)
	} else {
		logItemStatus("GitHub CLI", GHVersion, ghBin, true)
	}

	// 2.3 Install Pixi
	pixiDir := filepath.Join(depsDir, "pixi")
	pixiBin := filepath.Join(pixiDir, "pixi")
	if _, err := os.Stat(pixiBin); err != nil {
		logger.LogInfo("Step 2.3: Installing Pixi %s for ARM64...", PixiVersion)
		downloadUrl := "https://github.com/prefix-dev/pixi/releases/latest/download/pixi-aarch64-unknown-linux-musl"
		runSimpleShell(fmt.Sprintf("mkdir -p %s && wget -q -O %s %s && chmod +x %s", pixiDir, pixiBin, downloadUrl, pixiBin))
		logItemStatus("Pixi", PixiVersion, pixiBin, false)
	} else {
		logItemStatus("Pixi", PixiVersion, pixiBin, true)
	}

	// 3. Install Zig
	zigDir := filepath.Join(depsDir, "zig")
	zigBin := filepath.Join(zigDir, "zig")
	if _, err := os.Stat(zigBin); err != nil {
		logger.LogInfo("Step 3: Installing Zig %s for Linux ARM64...", ZigVersion)
		zigTarball := fmt.Sprintf("zig-linux-aarch64-%s.tar.xz", ZigVersion)
		downloadUrl := fmt.Sprintf("https://ziglang.org/download/%s/%s", ZigVersion, zigTarball)
		runSimpleShell(fmt.Sprintf("wget -O %s/%s %s", depsDir, zigTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", zigDir, zigDir, depsDir, zigTarball))
		os.Remove(filepath.Join(depsDir, zigTarball))
		logItemStatus("Zig", ZigVersion, zigBin, false)
	} else {
		logItemStatus("Zig", ZigVersion, zigBin, true)
	}
	printInstallComplete(depsDir)
}

func installLocalDepsMacOSARM() {
	logger.LogInfo("Installing local dependencies for macOS ARM (Apple Silicon)...")
	depsDir := GetDialtoneEnv()
	os.MkdirAll(depsDir, 0755)

	// 1. Install Go
	goDir := filepath.Join(depsDir, "go")
	goBin := filepath.Join(goDir, "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		logger.LogInfo("Step 1: Installing Go %s for macOS ARM64...", GoVersion)
		goTarball := fmt.Sprintf("go%s.darwin-arm64.tar.gz", GoVersion)
		downloadUrl := fmt.Sprintf("https://go.dev/dl/%s", goTarball)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/%s %s", depsDir, goTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("tar -C %s -xzf %s/%s", depsDir, depsDir, goTarball))
		os.Remove(filepath.Join(depsDir, goTarball))
		logItemStatus("Go", GoVersion, goBin, false)
	} else {
		logItemStatus("Go", GoVersion, goBin, true)
	}

	// 2. Install Node.js
	nodeDir := filepath.Join(depsDir, "node")
	nodeBin := filepath.Join(nodeDir, "bin", "node")
	if _, err := os.Stat(nodeBin); err != nil {
		logger.LogInfo("Step 2: Installing Node.js %s for macOS ARM64...", NodeVersion)
		nodeTarball := fmt.Sprintf("node-v%s-darwin-arm64.tar.gz", NodeVersion)
		downloadUrl := fmt.Sprintf("https://nodejs.org/dist/v%s/%s", NodeVersion, nodeTarball)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/%s %s", depsDir, nodeTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xzf %s/%s", nodeDir, nodeDir, depsDir, nodeTarball))
		os.Remove(filepath.Join(depsDir, nodeTarball))
		logItemStatus("Node.js", NodeVersion, nodeBin, false)
	} else {
		logItemStatus("Node.js", NodeVersion, nodeBin, true)
	}

	// 2.2 Install GitHub CLI
	ghDir := filepath.Join(depsDir, "gh")
	ghBin := filepath.Join(ghDir, "bin", "gh")
	if _, err := os.Stat(ghBin); err != nil {
		logger.LogInfo("Step 2.2: Installing GitHub CLI %s for macOS ARM64...", GHVersion)
		ghZip := fmt.Sprintf("gh_%s_macOS_arm64.zip", GHVersion)
		downloadUrl := fmt.Sprintf("https://github.com/cli/cli/releases/download/v%s/%s", GHVersion, ghZip)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/%s %s", depsDir, ghZip, downloadUrl))
		runSimpleShell(fmt.Sprintf("unzip -q %s/%s -d %s && mv %s/gh_%s_macOS_arm64/* %s/ && rm -rf %s/gh_%s_macOS_arm64", depsDir, ghZip, ghDir, ghDir, GHVersion, ghDir, ghDir, GHVersion))
		os.Remove(filepath.Join(depsDir, ghZip))
		logItemStatus("GitHub CLI", GHVersion, ghBin, false)
	} else {
		logItemStatus("GitHub CLI", GHVersion, ghBin, true)
	}

	// 2.3 Install Pixi
	pixiDir := filepath.Join(depsDir, "pixi")
	pixiBin := filepath.Join(pixiDir, "pixi")
	if _, err := os.Stat(pixiBin); err != nil {
		logger.LogInfo("Step 2.3: Installing Pixi %s for macOS ARM64...", PixiVersion)
		downloadUrl := "https://github.com/prefix-dev/pixi/releases/latest/download/pixi-aarch64-apple-darwin"
		runSimpleShell(fmt.Sprintf("mkdir -p %s && curl -L -o %s %s && chmod +x %s", pixiDir, pixiBin, downloadUrl, pixiBin))
		logItemStatus("Pixi", PixiVersion, pixiBin, false)
	} else {
		logItemStatus("Pixi", PixiVersion, pixiBin, true)
	}

	// 3. Install Zig
	zigDir := filepath.Join(depsDir, "zig")
	zigBin := filepath.Join(zigDir, "zig")
	if _, err := os.Stat(zigBin); err != nil {
		logger.LogInfo("Step 3: Installing Zig %s for macOS ARM64...", ZigVersion)
		zigTarball := fmt.Sprintf("zig-macos-aarch64-%s.tar.xz", ZigVersion)
		downloadUrl := fmt.Sprintf("https://ziglang.org/download/%s/%s", ZigVersion, zigTarball)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/%s %s", depsDir, zigTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", zigDir, zigDir, depsDir, zigTarball))
		os.Remove(filepath.Join(depsDir, zigTarball))
		logItemStatus("Zig", ZigVersion, zigBin, false)
	} else {
		logItemStatus("Zig", ZigVersion, zigBin, true)
	}

	printInstallComplete(depsDir)
}

func printInstallComplete(depsDir string) {
	logger.LogInfo("")
	logger.LogInfo("========================================")
	logger.LogInfo("Installation complete in %s", depsDir)
	logger.LogInfo("========================================")
	logger.LogInfo("")
	logger.LogInfo("Add to your shell profile (~/.zshrc or ~/.zshrc):")
	logger.LogInfo("  export PATH=\"%s/go/bin:%s/node/bin:%s/zig:%s/gh/bin:%s/pixi:$PATH\"", depsDir, depsDir, depsDir, depsDir, depsDir)
	logger.LogInfo("")
}

// CheckInstall verifies if all dependencies are correctly installed
func CheckInstall(depsDir string) {
	logger.LogInfo("Checking dependencies in %s...", depsDir)

	missing := 0

	// 1. Go
	goBin := filepath.Join(depsDir, "go", "bin", "go")
	if _, err := os.Stat(goBin); err == nil {
		logItemStatus("Go", GoVersion, goBin, true)
	} else {
		logger.LogInfo("Go (%s) is MISSING", GoVersion)
		missing++
	}

	// 2. Node.js
	nodeBin := filepath.Join(depsDir, "node", "bin", "node")
	if _, err := os.Stat(nodeBin); err == nil {
		logItemStatus("Node.js", NodeVersion, nodeBin, true)
	} else {
		logger.LogInfo("Node.js (%s) is MISSING", NodeVersion)
		missing++
	}

	// 2.1 Vercel (Optional for local dev)
	vercelBin := filepath.Join(depsDir, "node", "bin", "vercel")
	if _, err := os.Stat(vercelBin); err == nil {
		logItemStatus("Vercel CLI", "latest", vercelBin, true)
	} else {
		logger.LogInfo("Vercel CLI is MISSING (Optional)")
	}

	// 2.2 GitHub CLI
	ghBin := filepath.Join(depsDir, "gh", "bin", "gh")
	if _, err := os.Stat(ghBin); err == nil {
		logItemStatus("GitHub CLI", GHVersion, ghBin, true)
	} else {
		logger.LogInfo("GitHub CLI (%s) is MISSING", GHVersion)
		missing++
	}

	// 2.3 Pixi
	pixiBin := filepath.Join(depsDir, "pixi", "pixi")
	if _, err := os.Stat(pixiBin); err == nil {
		logItemStatus("Pixi", PixiVersion, pixiBin, true)
	} else {
		logger.LogInfo("Pixi (%s) is MISSING", PixiVersion)
		missing++
	}

	// 2.5 Zig
	zigBin := filepath.Join(depsDir, "zig", "zig")
	if _, err := os.Stat(zigBin); err == nil {
		logItemStatus("Zig", ZigVersion, zigBin, true)
	} else {
		logger.LogInfo("Zig (%s) is MISSING", ZigVersion)
		missing++
	}

	// 2.6 Podman check (System-level)
	if _, err := exec.LookPath("podman"); err == nil {
		logger.LogInfo("Podman (system) is present")
	} else if runtime.GOOS != "darwin" {
		logger.LogInfo("Podman is MISSING (Optional, recommended for ARM builds)")
	}

	// 2.7 ARM Cross-Compilers (Local)
	gcc64Bin := filepath.Join(depsDir, "gcc-aarch64", "bin", "aarch64-none-linux-gnu-gcc")
	if _, err := os.Stat(gcc64Bin); err == nil {
		logItemStatus("AArch64 Compiler", ArmCompilerVersion, gcc64Bin, true)
	} else {
		logger.LogInfo("AArch64 Compiler is MISSING")
		missing++
	}

	gcc32Bin := filepath.Join(depsDir, "gcc-armhf", "bin", "arm-none-linux-gnueabihf-gcc")
	if _, err := os.Stat(gcc32Bin); err == nil {
		logItemStatus("ARMhf Compiler", ArmCompilerVersion, gcc32Bin, true)
	} else {
		logger.LogInfo("ARMhf Compiler is MISSING")
		missing++
	}

	// 3. V4L2 Header (Linux only)
	headerFile := filepath.Join(depsDir, "usr", "include", "linux", "videodev2.h")
	if _, err := os.Stat(headerFile); err == nil {
		logItemStatus("V4L2 Headers", "latest", headerFile, true)
	} else if runtime.GOOS == "linux" {
		logger.LogInfo("V4L2 Headers are MISSING")
		missing++
	}

	if missing == 0 {
		logger.LogInfo("All dependencies are present.")
	} else {
		logger.LogFatal("%d dependencies are missing. Run './dialtone.sh install' to fix.", missing)
	}
}

func dialSSH(host, port, user, pass string) (*ssh.Client, error) {
	username := user
	hostname := host
	if username == "" {
		if i := strings.Index(host, "@"); i != -1 {
			username = host[:i]
			hostname = host[i+1:]
		}
	}
	if username == "" {
		username = os.Getenv("USER")
	}
	config := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	addr := fmt.Sprintf("%s:%s", hostname, port)
	return ssh.Dial("tcp", addr, config)
}

func runSSHCommand(client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	output, err := session.CombinedOutput(cmd)
	return string(output), err
}

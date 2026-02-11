package install

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"dialtone/cli/src/core/config"
	"dialtone/cli/src/core/logger"

	"golang.org/x/crypto/ssh"
)

const (
	GoVersion          = "1.25.5"
	NodeVersion        = "22.13.0"
	BunVersion         = "latest"
	ZigVersion         = "0.13.0"
	GHVersion          = "2.86.0"
	PixiVersion        = "latest" // Using latest for pixi
	PodmanVersion      = "latest"
	ArmCompilerVersion = "13.3.rel1"
	Arm64CompilerUrl   = "https://developer.arm.com/-/media/Files/downloads/gnu/13.3.rel1/binrel/arm-gnu-toolchain-13.3.rel1-x86_64-aarch64-none-linux-gnu.tar.xz"
	ArmhfCompilerUrl   = "https://developer.arm.com/-/media/Files/downloads/gnu/13.3.rel1/binrel/arm-gnu-toolchain-13.3.rel1-x86_64-arm-none-linux-gnueabihf.tar.xz"
	CloudflaredVersion = "2025.1.0"
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
	return config.GetDialtoneEnv()
}

func runSimpleShell(command string) {
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Command failed: %s: %v", command, err)
	}
}

func bunArchiveName() (string, error) {
	switch {
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		return "bun-darwin-aarch64.zip", nil
	case runtime.GOOS == "darwin" && runtime.GOARCH == "amd64":
		return "bun-darwin-x64.zip", nil
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		return "bun-linux-x64.zip", nil
	case runtime.GOOS == "linux" && runtime.GOARCH == "arm64":
		return "bun-linux-aarch64.zip", nil
	case runtime.GOOS == "windows" && runtime.GOARCH == "amd64":
		return "bun-windows-x64.zip", nil
	default:
		return "", fmt.Errorf("unsupported platform for bun: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

func installBun(depsDir string, step string) {
	bunDir := filepath.Join(depsDir, "bun")
	bunBin := filepath.Join(bunDir, "bin", "bun")
	if runtime.GOOS == "windows" {
		bunBin = filepath.Join(bunDir, "bin", "bun.exe")
	}
	
	if _, err := os.Stat(bunBin); err == nil {
		logItemStatus("Bun", BunVersion, bunBin, true)
		return
	}

	archive, err := bunArchiveName()
	if err != nil {
		logger.LogFatal("Failed to determine bun archive: %v", err)
	}

	logger.LogInfo("%s: Installing Bun %s...", step, BunVersion)
	downloadURL := fmt.Sprintf("https://github.com/oven-sh/bun/releases/latest/download/%s", archive)
	archivePath := filepath.Join(depsDir, archive)
	extractDir := filepath.Join(depsDir, "bun-extract")

	if runtime.GOOS == "windows" {
		psCmd := fmt.Sprintf("Invoke-WebRequest -Uri %s -OutFile %s; Expand-Archive -Path %s -DestinationPath %s -Force; New-Item -ItemType Directory -Force -Path %s/bin; Copy-Item -Path %s/bun-windows-x64/bun.exe -Destination %s/bin/bun.exe -Force; Remove-Item -Recurse -Force %s, %s", 
			downloadURL, archivePath, archivePath, extractDir, bunDir, extractDir, bunDir, archivePath, extractDir)
		cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psCmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logger.LogFatal("Failed to install Bun on Windows: %v", err)
		}
	} else {
		runSimpleShell(fmt.Sprintf("curl -L -o %s %s", archivePath, downloadURL))
		runSimpleShell(fmt.Sprintf("rm -rf %s && mkdir -p %s/bin", bunDir, bunDir))
		runSimpleShell(fmt.Sprintf("rm -rf %s && mkdir -p %s", extractDir, extractDir))
		runSimpleShell(fmt.Sprintf("unzip -q %s -d %s", archivePath, extractDir))
		runSimpleShell(fmt.Sprintf("cp -f %s/bun-*/bun %s/bin/bun", extractDir, bunDir))
		runSimpleShell(fmt.Sprintf("chmod +x %s/bin/bun", bunDir))
		runSimpleShell(fmt.Sprintf("rm -rf %s %s", archivePath, extractDir))
	}
	logItemStatus("Bun", BunVersion, bunBin, false)
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
		fmt.Println("  - Default location is ~/.dialtone_env")
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

	// Handle clean option is now handled in dialtone.sh wrapper
	if *clean {
		// No-op here, dialtone.sh already cleaned if this flag was present
		logger.LogInfo("Clean flag detected (already handled by dialtone.sh)")
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
	logger.LogInfo("%s", output)

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
	logger.LogInfo("%s", output)
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
	case runtime.GOOS == "windows":
		installLocalDepsWindows()
	default:
		logger.LogFatal("Unsupported platform: %s/%s. Use --linux-wsl or --macos-arm explicitly.", runtime.GOOS, runtime.GOARCH)
	}
}

func installLocalDepsWindows() {
	logger.LogInfo("Installing local dependencies for Windows...")
	depsDir := GetDialtoneEnv()
	os.MkdirAll(depsDir, 0755)

	// 1. Go is already handled by dialtone.ps1

	// 2. Install Bun
	installBun(depsDir, "Step 1")

	printInstallComplete(depsDir)
}


func installLocalDepsWSL() {
	logger.LogInfo("Installing local dependencies for Linux/WSL (User-Local, No Sudo)...")
	depsDir := GetDialtoneEnv()
	os.MkdirAll(depsDir, 0755)

	// 1. Install Go
	runSimpleShell("./dialtone.sh plugin install go")

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

	// 2.1 Install Bun
	installBun(depsDir, "Step 2.1")

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
		// Save original directory to restore later
		origDir, _ := os.Getwd()
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
			// Restore original directory
			os.Chdir(origDir)
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
			logItemStatus("ARMhf Compiler", ArmCompilerVersion, gcc32Bin, true)
		}

		// 7. Install AI
		runSimpleShell("./dialtone.sh plugin install ai")

		// 6. Install Cloudflared
		installCloudflaredLinuxAMD64(depsDir)
	}

	printInstallComplete(depsDir)
}

func installCloudflaredLinuxAMD64(depsDir string) {
	cfDir := filepath.Join(depsDir, "cloudflare")
	cfBin := filepath.Join(cfDir, "cloudflared")
	if _, err := os.Stat(cfBin); err != nil {
		logger.LogInfo("Step 6: Installing Cloudflared %s for Linux AMD64...", CloudflaredVersion)
		downloadUrl := fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/cloudflared-linux-amd64", CloudflaredVersion)
		runSimpleShell(fmt.Sprintf("mkdir -p %s && wget -q -O %s %s && chmod +x %s", cfDir, cfBin, downloadUrl, cfBin))
		logItemStatus("Cloudflared", CloudflaredVersion, cfBin, false)
	} else {
		logItemStatus("Cloudflared", CloudflaredVersion, cfBin, true)
	}
}

func installLocalDepsMacOSAMD64() {
	logger.LogInfo("Installing local dependencies for macOS x86_64 (Intel)...")
	depsDir := GetDialtoneEnv()
	os.MkdirAll(depsDir, 0755)

	// 1. Install Go
	runSimpleShell("./dialtone.sh plugin install go")

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

	// 2.1 Install Bun
	installBun(depsDir, "Step 2.1")

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
	// 4. Install Cloudflared
	installCloudflaredMacOSAMD64(depsDir)

	printInstallComplete(depsDir)
}

func installCloudflaredMacOSAMD64(depsDir string) {
	cfDir := filepath.Join(depsDir, "cloudflare")
	cfBin := filepath.Join(cfDir, "cloudflared")
	if _, err := os.Stat(cfBin); err != nil {
		logger.LogInfo("Step 4: Installing Cloudflared %s for macOS AMD64...", CloudflaredVersion)
		downloadUrl := fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/cloudflared-darwin-amd64.tgz", CloudflaredVersion)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/cloudflared.tgz %s", depsDir, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s -xzf %s/cloudflared.tgz", cfDir, cfDir, depsDir))
		os.Remove(filepath.Join(depsDir, "cloudflared.tgz"))
		logItemStatus("Cloudflared", CloudflaredVersion, cfBin, false)
	} else {
		logItemStatus("Cloudflared", CloudflaredVersion, cfBin, true)
	}
}

func installLocalDepsLinuxARM64() {
	logger.LogInfo("Installing local dependencies for Linux ARM64...")
	depsDir := GetDialtoneEnv()
	os.MkdirAll(depsDir, 0755)

	// 1. Install Go
	runSimpleShell("./dialtone.sh plugin install go")

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

	// 2.1 Install Bun
	installBun(depsDir, "Step 2.1")

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
	// 4. Install Cloudflared
	installCloudflaredLinuxARM64(depsDir)

	printInstallComplete(depsDir)
}

func installCloudflaredLinuxARM64(depsDir string) {
	cfDir := filepath.Join(depsDir, "cloudflare")
	cfBin := filepath.Join(cfDir, "cloudflared")
	if _, err := os.Stat(cfBin); err != nil {
		logger.LogInfo("Step 4: Installing Cloudflared %s for Linux ARM64...", CloudflaredVersion)
		downloadUrl := fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/cloudflared-linux-arm64", CloudflaredVersion)
		runSimpleShell(fmt.Sprintf("mkdir -p %s && wget -q -O %s %s && chmod +x %s", cfDir, cfBin, downloadUrl, cfBin))
		logItemStatus("Cloudflared", CloudflaredVersion, cfBin, false)
	} else {
		logItemStatus("Cloudflared", CloudflaredVersion, cfBin, true)
	}
}

func installLocalDepsMacOSARM() {
	logger.LogInfo("Installing local dependencies for macOS ARM (Apple Silicon)...")
	depsDir := GetDialtoneEnv()
	os.MkdirAll(depsDir, 0755)

	// 1. Install Go
	runSimpleShell("./dialtone.sh plugin install go")

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

	// 2.1 Install Bun
	installBun(depsDir, "Step 2.1")

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
		logItemStatus("Zig", ZigVersion, zigBin, true)
	}

	// 4. Install Cloudflared
	installCloudflaredMacOSARM(depsDir)

	printInstallComplete(depsDir)
}

func installCloudflaredMacOSARM(depsDir string) {
	cfDir := filepath.Join(depsDir, "cloudflare")
	cfBin := filepath.Join(cfDir, "cloudflared")
	if _, err := os.Stat(cfBin); err != nil {
		logger.LogInfo("Step 4: Installing Cloudflared %s for macOS ARM64...", CloudflaredVersion)
		downloadUrl := fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/cloudflared-darwin-arm64.tgz", CloudflaredVersion)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/cloudflared.tgz %s", depsDir, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s -xzf %s/cloudflared.tgz", cfDir, cfDir, depsDir))
		os.Remove(filepath.Join(depsDir, "cloudflared.tgz"))
		logItemStatus("Cloudflared", CloudflaredVersion, cfBin, false)
	} else {
		logItemStatus("Cloudflared", CloudflaredVersion, cfBin, true)
	}
}

func printInstallComplete(depsDir string) {
	logger.LogInfo("")
	logger.LogInfo("========================================")
	logger.LogInfo("Installation complete in %s", depsDir)
	logger.LogInfo("========================================")
	logger.LogInfo("")
	logger.LogInfo("Add to your shell profile (~/.zshrc or ~/.zshrc):")
	logger.LogInfo("  export PATH=\"%s/go/bin:%s/bun/bin:%s/node/bin:%s/zig:%s/gh/bin:%s/pixi:$PATH\"", depsDir, depsDir, depsDir, depsDir, depsDir, depsDir)
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

	// 2.1 Bun
	bunBin := filepath.Join(depsDir, "bun", "bin", "bun")
	if _, err := os.Stat(bunBin); err == nil {
		logItemStatus("Bun", BunVersion, bunBin, true)
	} else {
		logger.LogInfo("Bun (%s) is MISSING", BunVersion)
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

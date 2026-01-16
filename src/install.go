package dialtone

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func RunInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	linuxWSL := fs.Bool("linux-wsl", false, "Install dependencies natively on Linux/WSL (x86_64)")
	macosARM := fs.Bool("macos-arm", false, "Install dependencies natively on macOS ARM (Apple Silicon)")
	showHelp := fs.Bool("help", false, "Show help for install command")

	fs.Usage = func() {
		fmt.Println("Usage: dialtone install [options]")
		fmt.Println()
		fmt.Println("Install development dependencies (Go, Node.js, Zig) for building Dialtone.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --linux-wsl   Install for Linux/WSL x86_64")
		fmt.Println("  --macos-arm   Install for macOS ARM (Apple Silicon)")
		fmt.Println("  --host        SSH host for remote installation (user@host)")
		fmt.Println("  --port        SSH port (default: 22)")
		fmt.Println("  --user        SSH username")
		fmt.Println("  --pass        SSH password")
		fmt.Println("  --help        Show this help message")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  dialtone install                    # Auto-detect OS/arch and install locally")
		fmt.Println("  dialtone install --macos-arm        # Install for macOS Apple Silicon")
		fmt.Println("  dialtone install --linux-wsl        # Install for Linux/WSL x86_64")
		fmt.Println("  dialtone install --host pi@robot    # Install on remote robot via SSH")
		fmt.Println()
		fmt.Println("Notes:")
		fmt.Println("  - Dependencies are installed to ~/.dialtone_env (no sudo required)")
		fmt.Println("  - Auto-detects platform if no flags provided")
		fmt.Println("  - Skips already-installed dependencies")
		fmt.Println("  - Supported platforms: darwin/arm64, darwin/amd64, linux/amd64, linux/arm64")
	}

	fs.Parse(args)

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

	// If no host specified, auto-detect local OS/arch
	if *host == "" && *pass == "" {
		installLocalAuto()
		return
	}

	if *host == "" || *pass == "" {
		LogFatal("Error: -host (user@host) and -pass are required for remote install")
	}

	client, err := dialSSH(*host, *port, *user, *pass)
	if err != nil {
		LogFatal("SSH connection failed: %v", err)
	}
	defer client.Close()

	LogInfo("Installing dependencies on %s...", *host)

	// Install Go
	goVersion := "1.25.5"
	goTarball := fmt.Sprintf("go%s.linux-arm64.tar.gz", goVersion)
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
	`, goVersion, goTarball, *pass, *pass, goTarball, goTarball)

	output, err := runSSHCommand(client, installGoCmd)
	if err != nil {
		LogFatal("Failed to install Go: %v\nOutput: %s", err, output)
	}
	LogInfo(output)

	// Install Node.js
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
		LogFatal("Failed to install Node.js: %v\nOutput: %s", err, output)
	}
	LogInfo(output)
}

func installLocalAuto() {
	LogInfo("Auto-detecting system: %s/%s", runtime.GOOS, runtime.GOARCH)

	switch {
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		installLocalDepsMacOSARM()
	case runtime.GOOS == "darwin" && runtime.GOARCH == "amd64":
		LogInfo("macOS x86_64 detected. Installing with Rosetta-compatible deps...")
		installLocalDepsMacOSAMD64()
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		installLocalDepsWSL()
	case runtime.GOOS == "linux" && runtime.GOARCH == "arm64":
		LogInfo("Linux ARM64 detected (likely Raspberry Pi).")
		installLocalDepsLinuxARM64()
	default:
		LogFatal("Unsupported platform: %s/%s. Use --linux-wsl or --macos-arm explicitly.", runtime.GOOS, runtime.GOARCH)
	}
}

func installLocalDepsWSL() {
	LogInfo("Installing local dependencies for Linux/WSL (User-Local, No Sudo)...")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		LogFatal("Failed to get home directory: %v", err)
	}
	depsDir := filepath.Join(homeDir, ".dialtone_env")
	os.MkdirAll(depsDir, 0755)

	// 1. Install Go 1.25.5
	goVersion := "1.25.5"
	goDir := filepath.Join(depsDir, "go")
	if _, err := os.Stat(filepath.Join(goDir, "bin", "go")); err != nil {
		LogInfo("Step 1: Installing Go %s...", goVersion)
		goTarball := fmt.Sprintf("go%s.linux-amd64.tar.gz", goVersion)
		downloadUrl := fmt.Sprintf("https://go.dev/dl/%s", goTarball)
		runSimpleShell(fmt.Sprintf("wget -O %s/%s %s", depsDir, goTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("tar -C %s -xzf %s/%s", depsDir, depsDir, goTarball))
		os.Remove(filepath.Join(depsDir, goTarball))
	} else {
		LogInfo("Go is already installed in %s", goDir)
	}

	// 2. Install Node.js
	nodeDir := filepath.Join(depsDir, "node")
	if _, err := os.Stat(filepath.Join(nodeDir, "bin", "node")); err != nil {
		LogInfo("Step 2: Installing Node.js...")
		nodeVersion := "22.13.0" // Current LTS
		nodeTarball := fmt.Sprintf("node-v%s-linux-x64.tar.xz", nodeVersion)
		downloadUrl := fmt.Sprintf("https://nodejs.org/dist/v%s/%s", nodeVersion, nodeTarball)
		runSimpleShell(fmt.Sprintf("wget -q -O %s/%s %s", depsDir, nodeTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", nodeDir, nodeDir, depsDir, nodeTarball))
		os.Remove(filepath.Join(depsDir, nodeTarball))
	} else {
		LogInfo("Node.js is already installed in %s", nodeDir)
	}

	// 2.1 Install Vercel CLI
	if _, err := os.Stat(filepath.Join(nodeDir, "bin", "vercel")); err != nil {
		LogInfo("Step 2.1: Installing Vercel CLI...")
		runSimpleShell(fmt.Sprintf("%s/bin/npm install -g vercel", nodeDir))
	} else {
		LogInfo("Vercel CLI is already installed")
	}

	// 2.5 Install Zig (as portable C compiler)
	zigDir := filepath.Join(depsDir, "zig")
	if _, err := os.Stat(filepath.Join(zigDir, "zig")); err != nil {
		LogInfo("Step 2.5: Installing Zig (portable C compiler)...")
		zigVersion := "0.13.0"
		zigTarball := fmt.Sprintf("zig-linux-x86_64-%s.tar.xz", zigVersion)
		downloadUrl := fmt.Sprintf("https://ziglang.org/download/%s/%s", zigVersion, zigTarball)
		runSimpleShell(fmt.Sprintf("wget -q -O %s/%s %s", depsDir, zigTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", zigDir, zigDir, depsDir, zigTarball))
		os.Remove(filepath.Join(depsDir, zigTarball))
	} else {
		LogInfo("Zig is already installed in %s", zigDir)
	}

	// 3. Install V4L2 headers (extract from deb)
	includeDir := filepath.Join(depsDir, "usr", "include")
	if _, err := os.Stat(filepath.Join(includeDir, "linux", "videodev2.h")); err != nil {
		LogInfo("Step 3: Extracting V4L2 headers...")
		// Try apt-get download first, then fall back to direct mirrors
		err := os.Chdir(depsDir)
		if err == nil {
			LogInfo("Attempting apt-get download...")
			cmd := exec.Command("apt-get", "download", "libv4l-dev", "linux-libc-dev")
			if cmd.Run() != nil {
				LogInfo("apt-get download failed, falling back to Ubuntu mirrors...")
				// Noble Noble (24.04) mirrors
				runSimpleShell("wget -q http://archive.ubuntu.com/ubuntu/pool/main/v/v4l-utils/libv4l-dev_1.26.1-4build3_amd64.deb")
				runSimpleShell("wget -q http://archive.ubuntu.com/ubuntu/pool/main/l/linux/linux-libc-dev_6.8.0-31.31_amd64.deb")
			}
			runSimpleShell("dpkg -x libv4l-dev*.deb .")
			runSimpleShell("dpkg -x linux-libc-dev*.deb .")
			runSimpleShell("rm *.deb")
			os.Chdir(homeDir)
		}
	} else {
		LogInfo("V4L2 headers already present in %s", includeDir)
	}

	LogInfo("Local dependencies installation complete in %s", depsDir)
	LogInfo("To use these in your shell, add them to your PATH:")
	LogInfo("export PATH=$PATH:%s/go/bin:%s/node/bin", depsDir, depsDir)
}

func installLocalDepsMacOSAMD64() {
	LogInfo("Installing local dependencies for macOS x86_64 (Intel)...")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		LogFatal("Failed to get home directory: %v", err)
	}
	depsDir := filepath.Join(homeDir, ".dialtone_env")
	os.MkdirAll(depsDir, 0755)

	// 1. Install Go for darwin-amd64
	goVersion := "1.25.5"
	goDir := filepath.Join(depsDir, "go")
	if _, err := os.Stat(filepath.Join(goDir, "bin", "go")); err != nil {
		LogInfo("Step 1: Installing Go %s for macOS x86_64...", goVersion)
		goTarball := fmt.Sprintf("go%s.darwin-amd64.tar.gz", goVersion)
		downloadUrl := fmt.Sprintf("https://go.dev/dl/%s", goTarball)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/%s %s", depsDir, goTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("tar -C %s -xzf %s/%s", depsDir, depsDir, goTarball))
		os.Remove(filepath.Join(depsDir, goTarball))
	} else {
		LogInfo("Go is already installed in %s", goDir)
	}

	// 2. Install Node.js for darwin-x64
	nodeDir := filepath.Join(depsDir, "node")
	if _, err := os.Stat(filepath.Join(nodeDir, "bin", "node")); err != nil {
		LogInfo("Step 2: Installing Node.js for macOS x86_64...")
		nodeVersion := "22.13.0"
		nodeTarball := fmt.Sprintf("node-v%s-darwin-x64.tar.gz", nodeVersion)
		downloadUrl := fmt.Sprintf("https://nodejs.org/dist/v%s/%s", nodeVersion, nodeTarball)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/%s %s", depsDir, nodeTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xzf %s/%s", nodeDir, nodeDir, depsDir, nodeTarball))
		os.Remove(filepath.Join(depsDir, nodeTarball))
	} else {
		LogInfo("Node.js is already installed in %s", nodeDir)
	}

	// 2.1 Install Vercel CLI
	if _, err := os.Stat(filepath.Join(nodeDir, "bin", "vercel")); err != nil {
		LogInfo("Step 2.1: Installing Vercel CLI...")
		runSimpleShell(fmt.Sprintf("%s/bin/npm install -g vercel", nodeDir))
	} else {
		LogInfo("Vercel CLI is already installed")
	}

	// 3. Install Zig for darwin-x86_64
	zigDir := filepath.Join(depsDir, "zig")
	if _, err := os.Stat(filepath.Join(zigDir, "zig")); err != nil {
		LogInfo("Step 3: Installing Zig for macOS x86_64...")
		zigVersion := "0.13.0"
		zigTarball := fmt.Sprintf("zig-macos-x86_64-%s.tar.xz", zigVersion)
		downloadUrl := fmt.Sprintf("https://ziglang.org/download/%s/%s", zigVersion, zigTarball)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/%s %s", depsDir, zigTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", zigDir, zigDir, depsDir, zigTarball))
		os.Remove(filepath.Join(depsDir, zigTarball))
	} else {
		LogInfo("Zig is already installed in %s", zigDir)
	}

	printInstallComplete(depsDir)
}

func installLocalDepsLinuxARM64() {
	LogInfo("Installing local dependencies for Linux ARM64...")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		LogFatal("Failed to get home directory: %v", err)
	}
	depsDir := filepath.Join(homeDir, ".dialtone_env")
	os.MkdirAll(depsDir, 0755)

	// 1. Install Go for linux-arm64
	goVersion := "1.25.5"
	goDir := filepath.Join(depsDir, "go")
	if _, err := os.Stat(filepath.Join(goDir, "bin", "go")); err != nil {
		LogInfo("Step 1: Installing Go %s for Linux ARM64...", goVersion)
		goTarball := fmt.Sprintf("go%s.linux-arm64.tar.gz", goVersion)
		downloadUrl := fmt.Sprintf("https://go.dev/dl/%s", goTarball)
		runSimpleShell(fmt.Sprintf("wget -O %s/%s %s", depsDir, goTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("tar -C %s -xzf %s/%s", depsDir, depsDir, goTarball))
		os.Remove(filepath.Join(depsDir, goTarball))
	} else {
		LogInfo("Go is already installed in %s", goDir)
	}

	// 2. Install Node.js for linux-arm64
	nodeDir := filepath.Join(depsDir, "node")
	if _, err := os.Stat(filepath.Join(nodeDir, "bin", "node")); err != nil {
		LogInfo("Step 2: Installing Node.js for Linux ARM64...")
		nodeVersion := "22.13.0"
		nodeTarball := fmt.Sprintf("node-v%s-linux-arm64.tar.xz", nodeVersion)
		downloadUrl := fmt.Sprintf("https://nodejs.org/dist/v%s/%s", nodeVersion, nodeTarball)
		runSimpleShell(fmt.Sprintf("wget -O %s/%s %s", depsDir, nodeTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", nodeDir, nodeDir, depsDir, nodeTarball))
		os.Remove(filepath.Join(depsDir, nodeTarball))
	} else {
		LogInfo("Node.js is already installed in %s", nodeDir)
	}

	// 2.1 Install Vercel CLI
	if _, err := os.Stat(filepath.Join(nodeDir, "bin", "vercel")); err != nil {
		LogInfo("Step 2.1: Installing Vercel CLI...")
		runSimpleShell(fmt.Sprintf("%s/bin/npm install -g vercel", nodeDir))
	} else {
		LogInfo("Vercel CLI is already installed")
	}

	// 3. Install Zig for linux-aarch64
	zigDir := filepath.Join(depsDir, "zig")
	if _, err := os.Stat(filepath.Join(zigDir, "zig")); err != nil {
		LogInfo("Step 3: Installing Zig for Linux ARM64...")
		zigVersion := "0.13.0"
		zigTarball := fmt.Sprintf("zig-linux-aarch64-%s.tar.xz", zigVersion)
		downloadUrl := fmt.Sprintf("https://ziglang.org/download/%s/%s", zigVersion, zigTarball)
		runSimpleShell(fmt.Sprintf("wget -O %s/%s %s", depsDir, zigTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", zigDir, zigDir, depsDir, zigTarball))
		os.Remove(filepath.Join(depsDir, zigTarball))
	} else {
		LogInfo("Zig is already installed in %s", zigDir)
	}

	printInstallComplete(depsDir)
}

func installLocalDepsMacOSARM() {
	LogInfo("Installing local dependencies for macOS ARM (Apple Silicon)...")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		LogFatal("Failed to get home directory: %v", err)
	}
	depsDir := filepath.Join(homeDir, ".dialtone_env")
	os.MkdirAll(depsDir, 0755)

	// 1. Install Go 1.25.5 for darwin-arm64
	goVersion := "1.25.5"
	goDir := filepath.Join(depsDir, "go")
	if _, err := os.Stat(filepath.Join(goDir, "bin", "go")); err != nil {
		LogInfo("Step 1: Installing Go %s for macOS ARM64...", goVersion)
		goTarball := fmt.Sprintf("go%s.darwin-arm64.tar.gz", goVersion)
		downloadUrl := fmt.Sprintf("https://go.dev/dl/%s", goTarball)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/%s %s", depsDir, goTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("tar -C %s -xzf %s/%s", depsDir, depsDir, goTarball))
		os.Remove(filepath.Join(depsDir, goTarball))
	} else {
		LogInfo("Go is already installed in %s", goDir)
	}

	// 2. Install Node.js for darwin-arm64
	nodeDir := filepath.Join(depsDir, "node")
	if _, err := os.Stat(filepath.Join(nodeDir, "bin", "node")); err != nil {
		LogInfo("Step 2: Installing Node.js for macOS ARM64...")
		nodeVersion := "22.13.0" // Current LTS
		nodeTarball := fmt.Sprintf("node-v%s-darwin-arm64.tar.gz", nodeVersion)
		downloadUrl := fmt.Sprintf("https://nodejs.org/dist/v%s/%s", nodeVersion, nodeTarball)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/%s %s", depsDir, nodeTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xzf %s/%s", nodeDir, nodeDir, depsDir, nodeTarball))
		os.Remove(filepath.Join(depsDir, nodeTarball))
	} else {
		LogInfo("Node.js is already installed in %s", nodeDir)
	}

	// 2.1 Install Vercel CLI
	if _, err := os.Stat(filepath.Join(nodeDir, "bin", "vercel")); err != nil {
		LogInfo("Step 2.1: Installing Vercel CLI...")
		runSimpleShell(fmt.Sprintf("%s/bin/npm install -g vercel", nodeDir))
	} else {
		LogInfo("Vercel CLI is already installed")
	}

	// 3. Install Zig for darwin-arm64 (portable C compiler for CGO cross-compilation)
	zigDir := filepath.Join(depsDir, "zig")
	if _, err := os.Stat(filepath.Join(zigDir, "zig")); err != nil {
		LogInfo("Step 3: Installing Zig (portable C compiler) for macOS ARM64...")
		zigVersion := "0.13.0"
		zigTarball := fmt.Sprintf("zig-macos-aarch64-%s.tar.xz", zigVersion)
		downloadUrl := fmt.Sprintf("https://ziglang.org/download/%s/%s", zigVersion, zigTarball)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/%s %s", depsDir, zigTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", zigDir, zigDir, depsDir, zigTarball))
		os.Remove(filepath.Join(depsDir, zigTarball))
	} else {
		LogInfo("Zig is already installed in %s", zigDir)
	}

	printInstallComplete(depsDir)
}

func printInstallComplete(depsDir string) {
	LogInfo("")
	LogInfo("========================================")
	LogInfo("Installation complete in %s", depsDir)
	LogInfo("========================================")
	LogInfo("")
	LogInfo("Add to your shell profile (~/.zshrc or ~/.bashrc):")
	LogInfo("  export PATH=\"%s/go/bin:%s/node/bin:%s/zig:$PATH\"", depsDir, depsDir, depsDir)
	LogInfo("")
}

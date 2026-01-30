package install

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"dialtone/cli/src/core/config"
	"dialtone/cli/src/core/logger"

	"golang.org/x/crypto/ssh"
)

const (
	GoVersion          = "1.25.5"
	NodeVersion        = "22.13.0"
	ZigVersion         = "0.13.0"
	GHVersion          = "2.86.0"
	PixiVersion        = "latest" // Using latest for pixi
	PodmanVersion      = "latest"
	ArmCompilerVersion = "13.3.rel1"
	Arm64CompilerUrl   = "https://developer.arm.com/-/media/Files/downloads/gnu/13.3.rel1/binrel/arm-gnu-toolchain-13.3.rel1-x86_64-aarch64-none-linux-gnu.tar.xz"
	ArmhfCompilerUrl   = "https://developer.arm.com/-/media/Files/downloads/gnu/13.3.rel1/binrel/arm-gnu-toolchain-13.3.rel1-x86_64-arm-none-linux-gnueabihf.tar.xz"
	CloudflaredVersion = "2025.1.0"
)

type installTarget struct {
	os   string
	arch string
}

type installItem struct {
	name         string
	displayName  string
	downloadURL  string
	downloadPath string
	installedPath string
	sizeBytes    int64
	installed    bool
	installFn    func(string) error
}

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

func getDialtoneCache(depsDir string) string {
	cacheDir := os.Getenv("DIALTONE_CACHE")
	if cacheDir == "" {
		cacheDir = filepath.Join(depsDir, "cache")
	}
	return normalizePath(cacheDir)
}

func normalizePath(path string) string {
	if strings.HasPrefix(path, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

func runSimpleShell(command string) {
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Command failed: %s: %v", command, err)
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
	_ = clean // Handled by dialtone.sh
	cleanCache := fs.Bool("clean-cache", false, "Clear cached downloads before installation")
	_ = cleanCache // Handled by dialtone.sh
	check := fs.Bool("check", false, "Check if dependencies are installed and exit")

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
		logger.LogFatal("Error: --host (user@host) and --pass are required for remote install")
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

// RunInstallList prints dependency download details and status.
func RunInstallList(args []string) {
	target := parseInstallTarget(args)
	depsDir := GetDialtoneEnv()
	if depsDir == "" {
		logger.LogFatal("DIALTONE_ENV not set in .env or environment")
	}

	cacheDir := getDialtoneCache(depsDir)
	ensureDir(cacheDir)
	items := buildInstallItems(depsDir, target)
	if len(items) == 0 {
		logger.LogFatal("No installable dependencies found for %s/%s", target.os, target.arch)
	}

	fmt.Println("Checking dependency sizes...")
	fetchListSizes(items)

	fmt.Printf("Install list for %s/%s (DIALTONE_ENV=%s)\n\n", target.os, target.arch, depsDir)
	for _, item := range items {
		sizeText := "unknown"
		if item.sizeBytes > 0 {
			sizeText = formatSize(item.sizeBytes)
		}
		status := "no"
		if item.installed {
			status = "yes"
		}
		fmt.Printf("- %s (%s)\n", item.displayName, item.name)
		fmt.Printf("  installed: %s (%s)\n", status, item.installedPath)
		fmt.Printf("  url: %s\n", item.downloadURL)
		fmt.Printf("  size: %s\n\n", sizeText)
	}
}

// RunInstallDependency installs a single dependency by name.
func RunInstallDependency(args []string) {
	if len(args) == 0 {
		logger.LogFatal("Usage: dialtone install dependency <name>")
	}
	depName := strings.ToLower(strings.TrimSpace(args[0]))
	target := parseInstallTarget(args[1:])
	depsDir := GetDialtoneEnv()
	if depsDir == "" {
		logger.LogFatal("DIALTONE_ENV not set in .env or environment")
	}

	cacheDir := getDialtoneCache(depsDir)
	ensureDir(cacheDir)
	items := buildInstallItems(depsDir, target)
	for _, item := range items {
		if item.name != depName {
			continue
		}
		if item.installFn == nil {
			logger.LogFatal("Dependency %s is listed but cannot be installed on %s/%s", depName, target.os, target.arch)
		}
		if err := item.installFn(depsDir); err != nil {
			logger.LogFatal("Failed to install %s: %v", depName, err)
		}
		return
	}
	logger.LogFatal("Unknown dependency: %s", depName)
}

func parseInstallTarget(args []string) installTarget {
	fs := flag.NewFlagSet("install-target", flag.ContinueOnError)
	linuxWSL := fs.Bool("linux-wsl", false, "Install dependencies natively on Linux/WSL (x86_64)")
	macosARM := fs.Bool("macos-arm", false, "Install dependencies natively on macOS ARM (Apple Silicon)")
	_ = fs.Bool("clean", false, "Remove all dependencies before installation")
	_ = fs.Bool("clean-cache", false, "Clear cached downloads before installation")
	_ = fs.Bool("check", false, "Check if dependencies are installed and exit")
	_ = fs.String("host", "", "SSH host")
	_ = fs.String("port", "22", "SSH port")
	_ = fs.String("user", "", "SSH user")
	_ = fs.String("pass", "", "SSH password")
	_ = fs.Parse(args)

	if *linuxWSL {
		return installTarget{os: "linux", arch: "amd64"}
	}
	if *macosARM {
		return installTarget{os: "darwin", arch: "arm64"}
	}
	return installTarget{os: runtime.GOOS, arch: runtime.GOARCH}
}

func buildInstallItems(depsDir string, target installTarget) []installItem {
	var items []installItem
	cacheDir := getDialtoneCache(depsDir)

	goURL, goTar := goTarballURL(target)
	if goURL != "" {
		downloadPath := filepath.Join(cacheDir, goTar)
		installedPath := filepath.Join(depsDir, "go", "bin", "go")
		items = append(items, installItem{
			name:          "go",
			displayName:   "Go",
			downloadURL:   goURL,
			downloadPath:  downloadPath,
			installedPath: installedPath,
			sizeBytes:     -1,
			installed:     fileExists(installedPath),
			installFn:     installGoToolchain,
		})
	}

	nodeURL, nodeTar := nodeTarballURL(target)
	if nodeURL != "" {
		downloadPath := filepath.Join(cacheDir, nodeTar)
		installedPath := filepath.Join(depsDir, "node", "bin", "node")
		items = append(items, installItem{
			name:          "node",
			displayName:   "Node.js",
			downloadURL:   nodeURL,
			downloadPath:  downloadPath,
			installedPath: installedPath,
			sizeBytes:     -1,
			installed:     fileExists(installedPath),
			installFn: func(dir string) error {
				return installNodeToolchain(dir, target)
			},
		})
	}

	ghURL, ghArchive := ghArchiveURL(target)
	if ghURL != "" {
		downloadPath := filepath.Join(cacheDir, ghArchive)
		installedPath := filepath.Join(depsDir, "gh", "bin", "gh")
		items = append(items, installItem{
			name:          "gh",
			displayName:   "GitHub CLI",
			downloadURL:   ghURL,
			downloadPath:  downloadPath,
			installedPath: installedPath,
			sizeBytes:     -1,
			installed:     fileExists(installedPath),
			installFn: func(dir string) error {
				return installGitHubCLI(dir, target)
			},
		})
	}

	pixiURL := pixiURL(target)
	if pixiURL != "" {
		downloadPath := filepath.Join(cacheDir, fmt.Sprintf("pixi-%s-%s", target.os, target.arch))
		installedPath := filepath.Join(depsDir, "pixi", "pixi")
		items = append(items, installItem{
			name:          "pixi",
			displayName:   "Pixi",
			downloadURL:   pixiURL,
			downloadPath:  downloadPath,
			installedPath: installedPath,
			sizeBytes:     -1,
			installed:     fileExists(installedPath),
			installFn: func(dir string) error {
				return installPixi(dir, target)
			},
		})
	}

	zigURL, zigArchive := zigArchiveURL(target)
	if zigURL != "" {
		downloadPath := filepath.Join(cacheDir, zigArchive)
		installedPath := filepath.Join(depsDir, "zig", "zig")
		items = append(items, installItem{
			name:          "zig",
			displayName:   "Zig",
			downloadURL:   zigURL,
			downloadPath:  downloadPath,
			installedPath: installedPath,
			sizeBytes:     -1,
			installed:     fileExists(installedPath),
			installFn: func(dir string) error {
				return installZig(dir, target)
			},
		})
	}

	cfURL, cfArchive := cloudflaredURL(target)
	if cfURL != "" {
		downloadPath := filepath.Join(cacheDir, cfArchive)
		installedPath := filepath.Join(depsDir, "cloudflare", "cloudflared")
		items = append(items, installItem{
			name:          "cloudflared",
			displayName:   "Cloudflared",
			downloadURL:   cfURL,
			downloadPath:  downloadPath,
			installedPath: installedPath,
			sizeBytes:     -1,
			installed:     fileExists(installedPath),
			installFn: func(dir string) error {
				return installCloudflared(dir, target)
			},
		})
	}

	if target.os == "linux" {
		gcc64Bin := filepath.Join(depsDir, "gcc-aarch64", "bin", "aarch64-none-linux-gnu-gcc")
		items = append(items, installItem{
			name:          "gcc-aarch64",
			displayName:   "AArch64 Compiler",
			downloadURL:   Arm64CompilerUrl,
			downloadPath:  filepath.Join(cacheDir, "gcc-aarch64.tar.xz"),
			installedPath: gcc64Bin,
			sizeBytes:     -1,
			installed:     fileExists(gcc64Bin),
			installFn: func(dir string) error {
				return installArmCompilerAArch64(dir)
			},
		})

		gcc32Bin := filepath.Join(depsDir, "gcc-armhf", "bin", "arm-none-linux-gnueabihf-gcc")
		items = append(items, installItem{
			name:          "gcc-armhf",
			displayName:   "ARMhf Compiler",
			downloadURL:   ArmhfCompilerUrl,
			downloadPath:  filepath.Join(cacheDir, "gcc-armhf.tar.xz"),
			installedPath: gcc32Bin,
			sizeBytes:     -1,
			installed:     fileExists(gcc32Bin),
			installFn: func(dir string) error {
				return installArmCompilerARMhf(dir)
			},
		})
	}

	return items
}

func fetchContentLength(url string) int64 {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Head(url)
	if err != nil {
		return -1
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return -1
	}
	length := resp.Header.Get("Content-Length")
	if length == "" {
		return -1
	}
	size, err := strconv.ParseInt(length, 10, 64)
	if err != nil || size <= 0 {
		return -1
	}
	return size
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fetchListSizes(items []installItem) {
	type sizeResult struct {
		index int
		size  int64
	}

	results := make(chan sizeResult, len(items))
	for i := range items {
		i := i
		go func() {
			results <- sizeResult{index: i, size: fetchContentLength(items[i].downloadURL)}
		}()
	}

	for range items {
		result := <-results
		items[result.index].sizeBytes = result.size
	}
}

func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%dK", (size+1023)/1024)
	}
	return fmt.Sprintf("%dM", (size+(1024*1024-1))/(1024*1024))
}

func ensureDir(path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		logger.LogFatal("Failed to create directory %s: %v", path, err)
	}
}

func ensureCachedFile(url, cachePath string) error {
	if fileExists(cachePath) {
		return nil
	}
	ensureDir(filepath.Dir(cachePath))
	return downloadFile(url, cachePath)
}

func ensureValidTarXz(url, cachePath string) error {
	if err := ensureCachedFile(url, cachePath); err != nil {
		return err
	}
	if validateTarXz(cachePath) {
		return nil
	}
	_ = os.Remove(cachePath)
	return downloadFile(url, cachePath)
}

func ensureValidTarGz(url, cachePath string) error {
	if err := ensureCachedFile(url, cachePath); err != nil {
		return err
	}
	if validateTarGz(cachePath) {
		return nil
	}
	_ = os.Remove(cachePath)
	return downloadFile(url, cachePath)
}

func ensureValidZip(url, cachePath string) error {
	if err := ensureCachedFile(url, cachePath); err != nil {
		return err
	}
	if validateZip(cachePath) {
		return nil
	}
	_ = os.Remove(cachePath)
	return downloadFile(url, cachePath)
}

func downloadFile(url, dest string) error {
	if _, err := exec.LookPath("curl"); err == nil {
		cmd := exec.Command("curl", "-L", "-o", dest, url)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	if _, err := exec.LookPath("wget"); err == nil {
		cmd := exec.Command("wget", "-O", dest, url)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return fmt.Errorf("neither curl nor wget found in PATH")
}

func copyFile(src, dst string) error {
	if src == dst {
		return nil
	}
	if err := ensureDirForFile(dst); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func ensureDirForFile(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return nil
}

func validateTarXz(path string) bool {
	if !fileExists(path) {
		return false
	}
	cmd := exec.Command("tar", "-tJf", path)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func validateTarGz(path string) bool {
	if !fileExists(path) {
		return false
	}
	cmd := exec.Command("tar", "-tzf", path)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func validateZip(path string) bool {
	if !fileExists(path) {
		return false
	}
	if _, err := exec.LookPath("unzip"); err != nil {
		return false
	}
	cmd := exec.Command("unzip", "-t", path)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func extractTarXz(destDir, tarPath string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	cmd := exec.Command("tar", "-C", destDir, "--strip-components=1", "-xJf", tarPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func goTarballURL(target installTarget) (string, string) {
	if target.os != "darwin" && target.os != "linux" && target.os != "windows" {
		return "", ""
	}
	if target.arch != "amd64" && target.arch != "arm64" {
		return "", ""
	}
	tarball := fmt.Sprintf("go%s.%s-%s.tar.gz", GoVersion, target.os, target.arch)
	return fmt.Sprintf("https://go.dev/dl/%s", tarball), tarball
}

func nodeTarballURL(target installTarget) (string, string) {
	switch target.os {
	case "darwin":
		if target.arch == "arm64" {
			tarball := fmt.Sprintf("node-v%s-darwin-arm64.tar.gz", NodeVersion)
			return fmt.Sprintf("https://nodejs.org/dist/v%s/%s", NodeVersion, tarball), tarball
		}
		if target.arch == "amd64" {
			tarball := fmt.Sprintf("node-v%s-darwin-x64.tar.gz", NodeVersion)
			return fmt.Sprintf("https://nodejs.org/dist/v%s/%s", NodeVersion, tarball), tarball
		}
	case "linux":
		if target.arch == "arm64" {
			tarball := fmt.Sprintf("node-v%s-linux-arm64.tar.xz", NodeVersion)
			return fmt.Sprintf("https://nodejs.org/dist/v%s/%s", NodeVersion, tarball), tarball
		}
		if target.arch == "amd64" {
			tarball := fmt.Sprintf("node-v%s-linux-x64.tar.xz", NodeVersion)
			return fmt.Sprintf("https://nodejs.org/dist/v%s/%s", NodeVersion, tarball), tarball
		}
	}
	return "", ""
}

func ghArchiveURL(target installTarget) (string, string) {
	switch target.os {
	case "darwin":
		if target.arch == "arm64" {
			zip := fmt.Sprintf("gh_%s_macOS_arm64.zip", GHVersion)
			return fmt.Sprintf("https://github.com/cli/cli/releases/download/v%s/%s", GHVersion, zip), zip
		}
		if target.arch == "amd64" {
			zip := fmt.Sprintf("gh_%s_macOS_amd64.zip", GHVersion)
			return fmt.Sprintf("https://github.com/cli/cli/releases/download/v%s/%s", GHVersion, zip), zip
		}
	case "linux":
		if target.arch == "arm64" {
			tarball := fmt.Sprintf("gh_%s_linux_arm64.tar.gz", GHVersion)
			return fmt.Sprintf("https://github.com/cli/cli/releases/download/v%s/%s", GHVersion, tarball), tarball
		}
		if target.arch == "amd64" {
			tarball := fmt.Sprintf("gh_%s_linux_amd64.tar.gz", GHVersion)
			return fmt.Sprintf("https://github.com/cli/cli/releases/download/v%s/%s", GHVersion, tarball), tarball
		}
	}
	return "", ""
}

func pixiURL(target installTarget) string {
	switch target.os {
	case "darwin":
		if target.arch == "arm64" {
			return "https://github.com/prefix-dev/pixi/releases/latest/download/pixi-aarch64-apple-darwin"
		}
		if target.arch == "amd64" {
			return "https://github.com/prefix-dev/pixi/releases/latest/download/pixi-x86_64-apple-darwin"
		}
	case "linux":
		if target.arch == "arm64" {
			return "https://github.com/prefix-dev/pixi/releases/latest/download/pixi-aarch64-unknown-linux-musl"
		}
		if target.arch == "amd64" {
			return "https://github.com/prefix-dev/pixi/releases/latest/download/pixi-x86_64-unknown-linux-musl"
		}
	}
	return ""
}

func zigArchiveURL(target installTarget) (string, string) {
	switch target.os {
	case "darwin":
		if target.arch == "arm64" {
			tarball := fmt.Sprintf("zig-macos-aarch64-%s.tar.xz", ZigVersion)
			return fmt.Sprintf("https://ziglang.org/download/%s/%s", ZigVersion, tarball), tarball
		}
		if target.arch == "amd64" {
			tarball := fmt.Sprintf("zig-macos-x86_64-%s.tar.xz", ZigVersion)
			return fmt.Sprintf("https://ziglang.org/download/%s/%s", ZigVersion, tarball), tarball
		}
	case "linux":
		if target.arch == "arm64" {
			tarball := fmt.Sprintf("zig-linux-aarch64-%s.tar.xz", ZigVersion)
			return fmt.Sprintf("https://ziglang.org/download/%s/%s", ZigVersion, tarball), tarball
		}
		if target.arch == "amd64" {
			tarball := fmt.Sprintf("zig-linux-x86_64-%s.tar.xz", ZigVersion)
			return fmt.Sprintf("https://ziglang.org/download/%s/%s", ZigVersion, tarball), tarball
		}
	}
	return "", ""
}

func cloudflaredURL(target installTarget) (string, string) {
	switch target.os {
	case "darwin":
		if target.arch == "arm64" {
			tgz := "cloudflared-darwin-arm64.tgz"
			return fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/%s", CloudflaredVersion, tgz), tgz
		}
		if target.arch == "amd64" {
			tgz := "cloudflared-darwin-amd64.tgz"
			return fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/%s", CloudflaredVersion, tgz), tgz
		}
	case "linux":
		if target.arch == "arm64" {
			bin := "cloudflared-linux-arm64"
			return fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/%s", CloudflaredVersion, bin), bin
		}
		if target.arch == "amd64" {
			bin := "cloudflared-linux-amd64"
			return fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/%s", CloudflaredVersion, bin), bin
		}
	}
	return "", ""
}

func installGoToolchain(depsDir string) error {
	runSimpleShell("./dialtone.sh plugin install go")
	return nil
}

func installNodeToolchain(depsDir string, target installTarget) error {
	nodeDir := filepath.Join(depsDir, "node")
	nodeBin := filepath.Join(nodeDir, "bin", "node")
	if _, err := os.Stat(nodeBin); err == nil {
		logItemStatus("Node.js", NodeVersion, nodeBin, true)
		return nil
	}

	downloadURL, tarball := nodeTarballURL(target)
	if downloadURL == "" {
		return fmt.Errorf("unsupported Node.js target: %s/%s", target.os, target.arch)
	}

	cacheDir := getDialtoneCache(depsDir)
	cachePath := filepath.Join(cacheDir, tarball)
	if strings.HasSuffix(tarball, ".tar.xz") {
		if err := ensureValidTarXz(downloadURL, cachePath); err != nil {
			return err
		}
	} else if strings.HasSuffix(tarball, ".tar.gz") {
		if err := ensureValidTarGz(downloadURL, cachePath); err != nil {
			return err
		}
	} else if err := ensureCachedFile(downloadURL, cachePath); err != nil {
		return err
	}
	if err := copyFile(cachePath, filepath.Join(depsDir, tarball)); err != nil {
		return err
	}

	if strings.HasSuffix(tarball, ".tar.xz") {
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", nodeDir, nodeDir, depsDir, tarball))
	} else {
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xzf %s/%s", nodeDir, nodeDir, depsDir, tarball))
	}
	os.Remove(filepath.Join(depsDir, tarball))
	logItemStatus("Node.js", NodeVersion, nodeBin, false)
	return nil
}

func installGitHubCLI(depsDir string, target installTarget) error {
	ghDir := filepath.Join(depsDir, "gh")
	ghBin := filepath.Join(ghDir, "bin", "gh")
	if _, err := os.Stat(ghBin); err == nil {
		logItemStatus("GitHub CLI", GHVersion, ghBin, true)
		return nil
	}

	downloadURL, archive := ghArchiveURL(target)
	if downloadURL == "" {
		return fmt.Errorf("unsupported GitHub CLI target: %s/%s", target.os, target.arch)
	}

	cacheDir := getDialtoneCache(depsDir)
	cachePath := filepath.Join(cacheDir, archive)
	if strings.HasSuffix(archive, ".zip") {
		if err := ensureValidZip(downloadURL, cachePath); err != nil {
			return err
		}
	} else if strings.HasSuffix(archive, ".tar.gz") {
		if err := ensureValidTarGz(downloadURL, cachePath); err != nil {
			return err
		}
	} else if err := ensureCachedFile(downloadURL, cachePath); err != nil {
		return err
	}
	if err := copyFile(cachePath, filepath.Join(depsDir, archive)); err != nil {
		return err
	}

	if strings.HasSuffix(archive, ".zip") {
		runSimpleShell(fmt.Sprintf("unzip -q %s/%s -d %s && mv %s/gh_%s_macOS_%s/* %s/ && rm -rf %s/gh_%s_macOS_%s", depsDir, archive, ghDir, ghDir, GHVersion, target.arch, ghDir, ghDir, GHVersion, target.arch))
	} else {
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xzf %s/%s", ghDir, ghDir, depsDir, archive))
	}
	os.Remove(filepath.Join(depsDir, archive))
	logItemStatus("GitHub CLI", GHVersion, ghBin, false)
	return nil
}

func installPixi(depsDir string, target installTarget) error {
	pixiDir := filepath.Join(depsDir, "pixi")
	pixiBin := filepath.Join(pixiDir, "pixi")
	if _, err := os.Stat(pixiBin); err == nil {
		logItemStatus("Pixi", PixiVersion, pixiBin, true)
		return nil
	}

	downloadURL := pixiURL(target)
	if downloadURL == "" {
		return fmt.Errorf("unsupported Pixi target: %s/%s", target.os, target.arch)
	}

	cacheDir := getDialtoneCache(depsDir)
	cachePath := filepath.Join(cacheDir, fmt.Sprintf("pixi-%s-%s", target.os, target.arch))
	if err := ensureCachedFile(downloadURL, cachePath); err != nil {
		return err
	}
	ensureDir(pixiDir)
	if err := copyFile(cachePath, pixiBin); err != nil {
		return err
	}
	if err := os.Chmod(pixiBin, 0755); err != nil {
		return err
	}
	logItemStatus("Pixi", PixiVersion, pixiBin, false)
	return nil
}

func installZig(depsDir string, target installTarget) error {
	zigDir := filepath.Join(depsDir, "zig")
	zigBin := filepath.Join(zigDir, "zig")
	if _, err := os.Stat(zigBin); err == nil {
		logItemStatus("Zig", ZigVersion, zigBin, true)
		return nil
	}

	downloadURL, tarball := zigArchiveURL(target)
	if downloadURL == "" {
		return fmt.Errorf("unsupported Zig target: %s/%s", target.os, target.arch)
	}

	cacheDir := getDialtoneCache(depsDir)
	cachePath := filepath.Join(cacheDir, tarball)
	if err := ensureValidTarXz(downloadURL, cachePath); err != nil {
		return err
	}

	tarPath := filepath.Join(depsDir, tarball)
	if err := copyFile(cachePath, tarPath); err != nil {
		return err
	}
	if err := extractTarXz(zigDir, tarPath); err != nil {
		_ = os.Remove(cachePath)
		if err := downloadFile(downloadURL, cachePath); err != nil {
			return err
		}
		if err := copyFile(cachePath, tarPath); err != nil {
			return err
		}
		if err := extractTarXz(zigDir, tarPath); err != nil {
			return err
		}
	}
	_ = os.Remove(tarPath)
	logItemStatus("Zig", ZigVersion, zigBin, false)
	return nil
}

func installCloudflared(depsDir string, target installTarget) error {
	cfDir := filepath.Join(depsDir, "cloudflare")
	cfBin := filepath.Join(cfDir, "cloudflared")
	if _, err := os.Stat(cfBin); err == nil {
		logItemStatus("Cloudflared", CloudflaredVersion, cfBin, true)
		return nil
	}

	downloadURL, archive := cloudflaredURL(target)
	if downloadURL == "" {
		return fmt.Errorf("unsupported Cloudflared target: %s/%s", target.os, target.arch)
	}

	cacheDir := getDialtoneCache(depsDir)
	cachePath := filepath.Join(cacheDir, archive)
	if strings.HasSuffix(archive, ".tgz") {
		if err := ensureValidTarGz(downloadURL, cachePath); err != nil {
			return err
		}
	} else if err := ensureCachedFile(downloadURL, cachePath); err != nil {
		return err
	}

	if strings.HasSuffix(archive, ".tgz") {
		if err := copyFile(cachePath, filepath.Join(depsDir, archive)); err != nil {
			return err
		}
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s -xzf %s/%s", cfDir, cfDir, depsDir, archive))
		os.Remove(filepath.Join(depsDir, archive))
	} else {
		ensureDir(cfDir)
		if err := copyFile(cachePath, cfBin); err != nil {
			return err
		}
		if err := os.Chmod(cfBin, 0755); err != nil {
			return err
		}
	}
	logItemStatus("Cloudflared", CloudflaredVersion, cfBin, false)
	return nil
}

func installArmCompilerAArch64(depsDir string) error {
	gcc64Dir := filepath.Join(depsDir, "gcc-aarch64")
	gcc64Bin := filepath.Join(gcc64Dir, "bin", "aarch64-none-linux-gnu-gcc")
	if _, err := os.Stat(gcc64Bin); err == nil {
		logItemStatus("AArch64 Compiler", ArmCompilerVersion, gcc64Bin, true)
		return nil
	}

	tarball := "gcc-aarch64.tar.xz"
	cacheDir := getDialtoneCache(depsDir)
	cachePath := filepath.Join(cacheDir, tarball)
	if err := ensureValidTarXz(Arm64CompilerUrl, cachePath); err != nil {
		return err
	}
	if err := copyFile(cachePath, filepath.Join(depsDir, tarball)); err != nil {
		return err
	}
	runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", gcc64Dir, gcc64Dir, depsDir, tarball))
	os.Remove(filepath.Join(depsDir, tarball))
	logItemStatus("AArch64 Compiler", ArmCompilerVersion, gcc64Bin, false)
	return nil
}

func installArmCompilerARMhf(depsDir string) error {
	gcc32Dir := filepath.Join(depsDir, "gcc-armhf")
	gcc32Bin := filepath.Join(gcc32Dir, "bin", "arm-none-linux-gnueabihf-gcc")
	if _, err := os.Stat(gcc32Bin); err == nil {
		logItemStatus("ARMhf Compiler", ArmCompilerVersion, gcc32Bin, true)
		return nil
	}

	tarball := "gcc-armhf.tar.xz"
	cacheDir := getDialtoneCache(depsDir)
	cachePath := filepath.Join(cacheDir, tarball)
	if err := ensureValidTarXz(ArmhfCompilerUrl, cachePath); err != nil {
		return err
	}
	if err := copyFile(cachePath, filepath.Join(depsDir, tarball)); err != nil {
		return err
	}
	runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", gcc32Dir, gcc32Dir, depsDir, tarball))
	os.Remove(filepath.Join(depsDir, tarball))
	logItemStatus("ARMhf Compiler", ArmCompilerVersion, gcc32Bin, false)
	return nil
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
	if err := installGoToolchain(depsDir); err != nil {
		logger.LogFatal("Failed to install Go: %v", err)
	}

	target := installTarget{os: "linux", arch: "amd64"}

	// 2. Install Node.js
	if err := installNodeToolchain(depsDir, target); err != nil {
		logger.LogFatal("Failed to install Node.js: %v", err)
	}

	// 2.1 Install Vercel CLI
	nodeDir := filepath.Join(depsDir, "node")
	vercelBin := filepath.Join(nodeDir, "bin", "vercel")
	if _, err := os.Stat(vercelBin); err != nil {
		logger.LogInfo("Step 2.1: Installing Vercel CLI...")
		runSimpleShell(fmt.Sprintf("%s/bin/npm install -g --prefix %s vercel", nodeDir, nodeDir))
		logItemStatus("Vercel CLI", "latest", vercelBin, false)
	} else {
		logItemStatus("Vercel CLI", "latest", vercelBin, true)
	}

	// 2.2 Install GitHub CLI
	if err := installGitHubCLI(depsDir, target); err != nil {
		logger.LogFatal("Failed to install GitHub CLI: %v", err)
	}

	// 2.3 Install Pixi
	if err := installPixi(depsDir, target); err != nil {
		logger.LogFatal("Failed to install Pixi: %v", err)
	}

	// 2.5 Install Zig
	if err := installZig(depsDir, target); err != nil {
		logger.LogFatal("Failed to install Zig: %v", err)
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
		if err := installArmCompilerAArch64(depsDir); err != nil {
			logger.LogFatal("Failed to install AArch64 compiler: %v", err)
		}

		// ARMhf Compiler
		if err := installArmCompilerARMhf(depsDir); err != nil {
			logger.LogFatal("Failed to install ARMhf compiler: %v", err)
		}

		// 7. Install AI
		runSimpleShell("./dialtone.sh plugin install ai")

		// 6. Install Cloudflared
		if err := installCloudflared(depsDir, target); err != nil {
			logger.LogFatal("Failed to install Cloudflared: %v", err)
		}
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
	if err := installGoToolchain(depsDir); err != nil {
		logger.LogFatal("Failed to install Go: %v", err)
	}

	target := installTarget{os: "darwin", arch: "amd64"}

	// 2. Install Node.js
	if err := installNodeToolchain(depsDir, target); err != nil {
		logger.LogFatal("Failed to install Node.js: %v", err)
	}

	// 2.2 Install GitHub CLI
	if err := installGitHubCLI(depsDir, target); err != nil {
		logger.LogFatal("Failed to install GitHub CLI: %v", err)
	}

	// 2.3 Install Pixi
	if err := installPixi(depsDir, target); err != nil {
		logger.LogFatal("Failed to install Pixi: %v", err)
	}
	// 4. Install Cloudflared
	if err := installCloudflared(depsDir, target); err != nil {
		logger.LogFatal("Failed to install Cloudflared: %v", err)
	}

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
	if err := installGoToolchain(depsDir); err != nil {
		logger.LogFatal("Failed to install Go: %v", err)
	}

	target := installTarget{os: "linux", arch: "arm64"}

	// 2. Install Node.js
	if err := installNodeToolchain(depsDir, target); err != nil {
		logger.LogFatal("Failed to install Node.js: %v", err)
	}

	// 2.2 Install GitHub CLI
	if err := installGitHubCLI(depsDir, target); err != nil {
		logger.LogFatal("Failed to install GitHub CLI: %v", err)
	}

	// 2.3 Install Pixi
	if err := installPixi(depsDir, target); err != nil {
		logger.LogFatal("Failed to install Pixi: %v", err)
	}

	// 4. Install Cloudflared
	if err := installCloudflared(depsDir, target); err != nil {
		logger.LogFatal("Failed to install Cloudflared: %v", err)
	}

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
	if err := installGoToolchain(depsDir); err != nil {
		logger.LogFatal("Failed to install Go: %v", err)
	}

	target := installTarget{os: "darwin", arch: "arm64"}

	// 2. Install Node.js
	if err := installNodeToolchain(depsDir, target); err != nil {
		logger.LogFatal("Failed to install Node.js: %v", err)
	}

	// 2.2 Install GitHub CLI
	if err := installGitHubCLI(depsDir, target); err != nil {
		logger.LogFatal("Failed to install GitHub CLI: %v", err)
	}

	// 2.3 Install Pixi
	if err := installPixi(depsDir, target); err != nil {
		logger.LogFatal("Failed to install Pixi: %v", err)
	}

	// 3. Install Zig
	if err := installZig(depsDir, target); err != nil {
		logger.LogFatal("Failed to install Zig: %v", err)
	}

	// 4. Install Cloudflared
	if err := installCloudflared(depsDir, target); err != nil {
		logger.LogFatal("Failed to install Cloudflared: %v", err)
	}

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

	// 2.7 ARM Cross-Compilers (Local, Linux only)
	if runtime.GOOS == "linux" {
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

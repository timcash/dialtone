package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"dialtone/dev/core/config"
	"dialtone/dev/core/logger"
)

const (
	CloudflaredVersion = "2025.1.0"
)

func logItemStatus(name, version, path string, alreadyInstalled bool) {
	status := "installed successfully"
	if alreadyInstalled {
		status = "is already installed"
	}
	logger.LogInfo("%s (%s) %s at %s", name, version, status, path)
}

func runSimpleShell(command string) {
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Command failed: %s: %v", command, err)
	}
}

// RunCloudflaredInstall handles the installation of cloudflared for the current OS/architecture.
func RunCloudflaredInstall(depsDir string) {
	cfDir := filepath.Join(depsDir, "cloudflare")
	cfBin := filepath.Join(cfDir, "cloudflared")

	if _, err := os.Stat(cfBin); err == nil {
		logItemStatus("Cloudflared", CloudflaredVersion, cfBin, true)
		return
	}

	logger.LogInfo("Installing Cloudflared %s for %s/%s...", CloudflaredVersion, runtime.GOOS, runtime.GOARCH)

	downloadUrl := ""
	switch {
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		downloadUrl = fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/cloudflared-linux-amd64", CloudflaredVersion)
	case runtime.GOOS == "linux" && runtime.GOARCH == "arm64":
		downloadUrl = fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/cloudflared-linux-arm64", CloudflaredVersion)
	case runtime.GOOS == "darwin" && runtime.GOARCH == "amd64":
		downloadUrl = fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/cloudflared-darwin-amd64.tgz", CloudflaredVersion)
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		downloadUrl = fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/download/%s/cloudflared-darwin-arm64.tgz", CloudflaredVersion)
	default:
		logger.LogFatal("Unsupported platform for Cloudflared: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	runSimpleShell(fmt.Sprintf("mkdir -p %s", cfDir))
	runSimpleShell(fmt.Sprintf("wget -q -O %s %s", cfBin, downloadUrl))
	runSimpleShell(fmt.Sprintf("chmod +x %s", cfBin))

	// Handle .tgz archives for macOS
	if strings.HasSuffix(downloadUrl, ".tgz") {
		runSimpleShell(fmt.Sprintf("tar -xzf %s -C %s", cfBin, cfDir))
		runSimpleShell(fmt.Sprintf("mv %s/cloudflared %s", cfDir, cfBin)) // cloudflared binary is inside the tar.gz
	}


	if _, err := os.Stat(cfBin); err != nil {
		logger.LogFatal("Failed to install Cloudflared at %s: %v", cfBin, err)
	} else {
		logItemStatus("Cloudflared", CloudflaredVersion, cfBin, false)
	}
}
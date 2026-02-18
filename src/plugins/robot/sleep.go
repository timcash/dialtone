package robot

import (
	"dialtone/cli/src/core/logger"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func RunSleep(versionDir string, args []string) {
	// Parse args/env
	hostname := os.Getenv("DIALTONE_HOSTNAME")
	if hostname == "" {
		hostname = "drone-1"
	}

	cwd, _ := os.Getwd()
	sleepSrc := filepath.Join(cwd, "src", "plugins", "robot", versionDir, "cmd", "sleep", "main.go")
	binDir := filepath.Join(cwd, "src", "plugins", "robot", "bin")
	localBin := filepath.Join(binDir, "dialtone-sleep")

	// 1. Build Sleep Binary (Local Arch)
	logger.LogInfo("[SLEEP] Building sleep binary for local machine...")
	// We use 'go build' directly for local build, assuming go is installed or using docker for local arch if needed.
	// For simplicity in this env, we'll use the local go if available, or the builder image matching local arch.
	// Assuming local go is available since we are running dialtone.sh.
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", localBin, sleepSrc)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Failed to build sleep binary: %v", err)
	}

	// 2. Setup Local Systemd Service for Sleep Server
	logger.LogInfo("[SLEEP] Setting up local sleep service...")
	home, _ := os.UserHomeDir()
	stateDir := filepath.Join(home, ".config", "dialtone-sleep")
	os.MkdirAll(stateDir, 0755)

	serviceContent := fmt.Sprintf(`[Unit]
Description=Dialtone Robot Sleep Server (Local)
After=network.target

[Service]
ExecStart=%s --hostname %s --state-dir %s
WorkingDirectory=%s
Environment="TS_AUTHKEY=%s"
Restart=always

[Install]
WantedBy=default.target`, localBin, hostname, stateDir, cwd, os.Getenv("TS_AUTHKEY"))

	userSystemdDir := filepath.Join(home, ".config", "systemd", "user")
	os.MkdirAll(userSystemdDir, 0755)
	serviceFile := filepath.Join(userSystemdDir, "dialtone-sleep.service")
	
	if err := os.WriteFile(serviceFile, []byte(serviceContent), 0644); err != nil {
		logger.LogFatal("Failed to write service file: %v", err)
	}

	// Reload and Start Sleep Service
	exec.Command("systemctl", "--user", "daemon-reload").Run()
	exec.Command("systemctl", "--user", "enable", "dialtone-sleep.service").Run()
	if err := exec.Command("systemctl", "--user", "restart", "dialtone-sleep.service").Run(); err != nil {
		logger.LogFatal("Failed to start dialtone-sleep.service: %v", err)
	}

	// 3. Reconfigure Proxy to point to Localhost:8080
	logger.LogInfo("[SLEEP] Reconfiguring Cloudflare proxy to http://localhost:8080...")
	// We call the cloudflare plugin CLI to update the service with the new --url
	proxyCmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "cloudflare", "setup-service", "--name", hostname, "--user", "--url", "http://localhost:8080")
	proxyCmd.Stdout = os.Stdout
	proxyCmd.Stderr = os.Stderr
	if err := proxyCmd.Run(); err != nil {
		logger.LogWarn("[WARNING] Failed to update proxy service: %v", err)
	}

	logger.LogInfo("[SLEEP] Sleep mode active locally.")
	logger.LogInfo(" - Sleep Server: http://localhost:8080")
	logger.LogInfo(" - Public Proxy: https://%s.dialtone.earth -> localhost:8080", hostname)
	logger.LogInfo(" - Robot: Assumed OFF (Remote)")
}

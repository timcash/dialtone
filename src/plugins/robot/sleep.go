package robot

import (
	"dialtone/cli/src/core/logger"
	core_ssh "dialtone/cli/src/core/ssh"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"
)

func RunSleep(versionDir string, args []string) {
	// Parse args/env
	host := os.Getenv("ROBOT_HOST")
	user := os.Getenv("ROBOT_USER")
	pass := os.Getenv("ROBOT_PASSWORD")
	hostname := os.Getenv("DIALTONE_HOSTNAME")

	if host == "" || pass == "" {
		logger.LogFatal("ROBOT_HOST and ROBOT_PASSWORD are required")
	}
	if hostname == "" {
		hostname = "drone-1"
	}

	cwd, _ := os.Getwd()
	// Use relative path for Podman container build
	sleepSrcRel := filepath.Join("src", "plugins", "robot", versionDir, "cmd", "sleep", "main.go")
	binDir := filepath.Join(cwd, "src", "plugins", "robot", "bin")
	localBin := filepath.Join(binDir, "dialtone-sleep")

	// 1. Build Sleep Binary (ARM64)
	logger.LogInfo("[SLEEP] Building sleep binary for ARM64...")
	cmd := exec.Command("podman", "run", "--rm",
		"-v", fmt.Sprintf("%s:/src:Z", cwd),
		"-v", "dialtone-go-build-cache:/root/.cache/go-build:Z",
		"-w", "/src",
		"-e", "GOOS=linux",
		"-e", "GOARCH=arm64",
		"-e", "CGO_ENABLED=0",
		"dialtone-builder",
		"go", "build", "-trimpath", "-ldflags=-s -w", "-o", "src/plugins/robot/bin/dialtone-sleep", sleepSrcRel)
	
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Failed to build sleep binary: %v", err)
	}

	// 2. Connect to Robot
	logger.LogInfo("[SLEEP] Connecting to %s...", host)
	client, err := core_ssh.DialSSH(host, "22", user, pass)
	if err != nil {
		logger.LogFatal("SSH connection failed: %v", err)
	}
	defer client.Close()

	remoteHome, _ := core_ssh.GetRemoteHome(client)
	remoteDeployDir := path.Join(remoteHome, "dialtone_deploy")
	remoteBin := path.Join(remoteDeployDir, "dialtone-sleep")

	// 2.5 Stop existing services before upload (file busy)
	logger.LogInfo("[SLEEP] Stopping remote services...")
	stopCmds := []string{
		fmt.Sprintf("echo '%s' | sudo -S systemctl stop dialtone-sleep.service", pass),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 dialtone-sleep || true", pass),
	}
	for _, c := range stopCmds {
		core_ssh.RunSSHCommand(client, c)
	}

	// 3. Upload Binary
	logger.LogInfo("[SLEEP] Uploading binary to %s...", remoteBin)
	if err := core_ssh.UploadFile(client, localBin, remoteBin); err != nil {
		logger.LogFatal("Upload failed: %v", err)
	}
	core_ssh.RunSSHCommand(client, "chmod +x "+remoteBin)

	// 4. Setup Systemd Service
	serviceContent := fmt.Sprintf(`[Unit]
Description=Dialtone Robot SLEEP Mode
After=network.target

[Service]
ExecStart=%s --hostname %s --state-dir %s/.config/dialtone
WorkingDirectory=%s
Environment="TS_AUTHKEY=%s"
Restart=always
User=%s

[Install]
WantedBy=multi-user.target`, remoteBin, hostname, remoteHome, remoteDeployDir, os.Getenv("TS_AUTHKEY"), user)

	tempServicePath := path.Join(remoteDeployDir, fmt.Sprintf("dialtone-sleep.service.tmp.%d", time.Now().Unix()))
	core_ssh.WriteRemoteFile(client, tempServicePath, serviceContent)

	finalServicePath := "/etc/systemd/system/dialtone-sleep.service"
	
	// 5. Swap Services
	logger.LogInfo("[SLEEP] Stopping dialtone.service and starting dialtone-sleep.service...")
	cmds := []string{
		fmt.Sprintf("echo '%s' | sudo -S mv %s %s", pass, tempServicePath, finalServicePath),
		fmt.Sprintf("echo '%s' | sudo -S chmod 0644 %s", pass, finalServicePath),
		fmt.Sprintf("echo '%s' | sudo -S systemctl daemon-reload", pass),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 dialtone || true", pass), // Kill background/nohup instances
		fmt.Sprintf("echo '%s' | sudo -S systemctl stop dialtone.service", pass),
		fmt.Sprintf("echo '%s' | sudo -S systemctl disable dialtone.service", pass),
		fmt.Sprintf("echo '%s' | sudo -S systemctl enable dialtone-sleep.service", pass),
		fmt.Sprintf("echo '%s' | sudo -S systemctl restart dialtone-sleep.service", pass),
	}

	for _, c := range cmds {
		if _, err := core_ssh.RunSSHCommand(client, c); err != nil {
			logger.LogFatal("Remote command failed: %v", err)
		}
	}

	logger.LogInfo("[SLEEP] Robot is now sleeping. Access at https://%s.dialtone.earth", hostname)

	// 6. Ensure local proxy is running
	proxyService := fmt.Sprintf("dialtone-proxy-%s.service", hostname)
	logger.LogInfo("[SLEEP] ensuring local proxy service '%s' is running...", proxyService)
	if err := exec.Command("systemctl", "--user", "enable", "--now", proxyService).Run(); err != nil {
		logger.LogWarn("[WARNING] Failed to start local proxy service: %v. You may need to run 'robot deploy --proxy' first.", err)
	} else {
		logger.LogInfo("[SLEEP] Local proxy service active.")
	}
}

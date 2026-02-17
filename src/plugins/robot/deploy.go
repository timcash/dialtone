package robot

import (
	"dialtone/cli/src/core/logger"
	core_ssh "dialtone/cli/src/core/ssh"
	rcli "dialtone/cli/src/plugins/robot/robot_cli"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"golang.org/x/crypto/ssh"
)

func RunDeploy(versionDir string, args []string) {
	fs := flag.NewFlagSet("deploy", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	hostname := fs.String("hostname", os.Getenv("DIALTONE_HOSTNAME"), "Tailscale hostname for the robot")
	proxy := fs.Bool("proxy", false, "Expose Web UI via Cloudflare proxy (drone-1.dialtone.earth)")
	service := fs.Bool("service", false, "Set up Dialtone as a systemd service on the robot")
	diagnostic := fs.Bool("diagnostic", false, "Run UI diagnostic after deployment")
	verify := fs.Bool("verify", false, "Run step-by-step remote verification")
	fs.Parse(args)

	if *host == "" {
		logger.LogFatal("Error: --host or ROBOT_HOST is required")
	}
	if *pass == "" {
		logger.LogFatal("Error: --pass or ROBOT_PASSWORD is required")
	}

	if *verify {
		if err := rcli.RunDeployTest(versionDir, nil); err != nil {
			logger.LogFatal("Pre-deployment verification failed: %v", err)
		}
	}

	// Use DIALTONE_HOSTNAME if set in env, otherwise fallback to SSH host if flag not provided
	if *hostname == "" {
		*hostname = os.Getenv("DIALTONE_HOSTNAME")
	}
	if *hostname == "" {
		*hostname = *host
	}

	validateRequiredVars([]string{"DIALTONE_HOSTNAME", "TS_AUTHKEY"})

	cwd, _ := os.Getwd()
	
	// Auto-bump UI version before build
	bumpUIVersion(versionDir)
	
	// Read UI version from package.json
	uiVersion := "unknown"
	pkgJSONPath := filepath.Join(cwd, "src", "plugins", "robot", versionDir, "ui", "package.json")
	if data, err := os.ReadFile(pkgJSONPath); err == nil {
		var pkg struct { Version string `json:"version"` }
		if err := json.Unmarshal(data, &pkg); err == nil {
			uiVersion = pkg.Version
		}
	}

	logger.LogInfo("[DEPLOY] Starting deployment of Robot UI %s (%s)...", uiVersion, versionDir)
	logger.LogInfo("[DEPLOY] Target: %s (%s)", *hostname, *host)

	// Step 1: Install dependencies using version-specific logic
	if err := RunInstall(versionDir); err != nil {
		logger.LogFatal("Failed to install UI dependencies: %v", err)
	}

	logger.LogInfo("Connecting to %s to detect architecture...", *host)
	client, err := core_ssh.DialSSH(*host, *port, *user, *pass)
	if err != nil {
		logger.LogFatal("Failed to connect: %v", err)
	}
	defer client.Close()

	validateSudo(client, *pass)
	setupSSHKey(client, *user, *pass)

	remoteArch, _ := core_ssh.RunSSHCommand(client, "uname -m")
	remoteArch = strings.TrimSpace(remoteArch)

	var buildFlag string
	switch remoteArch {
	case "aarch64", "arm64":
		buildFlag = "--linux-arm64"
	case "x86_64", "amd64":
		buildFlag = "--linux-amd64"
	default:
		logger.LogFatal("Unsupported arch: %s", remoteArch)
	}

	// Step 2: Build UI and Binary using version-specific logic
	if err := RunBuild(versionDir, buildFlag); err != nil {
		logger.LogFatal("Failed to build: %v", err)
	}

	logger.LogInfo("Starting deployment to %s...", *host)
	
	// Remote binary name is 'dialtone' to match legacy behavior
	remoteBinaryName := "dialtone"
	binaryName := "dialtone-arm64"
	if remoteArch == "x86_64" || remoteArch == "amd64" {
		binaryName = "dialtone-amd64"
	}
	
	localBinaryPath := filepath.Join("src", "plugins", "robot", "bin", binaryName)
	remoteHome, _ := core_ssh.GetRemoteHome(client)
	// Use REMOTE_DIR_DEPLOY if set, otherwise default to 'dialtone_deploy'
	remoteDeployDir := os.Getenv("REMOTE_DIR_DEPLOY")
	if remoteDeployDir == "" {
		remoteDeployDir = path.Join(remoteHome, "dialtone_deploy")
	}
	remotePath := path.Join(remoteDeployDir, remoteBinaryName)

	// Stop ALL legacy services and kill any stray processes before uploading
	logger.LogInfo("[DEPLOY] Stopping all remote services and processes...")
	// Stop/Disable legacy services if they exist
	for _, svc := range []string{"dialtone", "dialtone-robot", "dialtone_robot"} {
		core_ssh.RunSSHCommand(client, fmt.Sprintf("printf \"%%s\\n\" \"%s\" | sudo -S -p '' systemctl stop %s.service", *pass, svc))
		core_ssh.RunSSHCommand(client, fmt.Sprintf("printf \"%%s\\n\" \"%s\" | sudo -S -p '' systemctl disable %s.service", *pass, svc))
	}

	// Kill any process using old or new name
	killCmd := fmt.Sprintf("printf \"%%s\\n\" \"%s\" | sudo -S -p '' pkill -9 dialtone; printf \"%%s\\n\" \"%s\" | sudo -S -p '' pkill -9 %s", *pass, *pass, remoteBinaryName)
	core_ssh.RunSSHCommand(client, killCmd)
	
	// Ensure remote deploy directory exists
	core_ssh.RunSSHCommand(client, fmt.Sprintf("rm -rf %s && mkdir -p %s", remoteDeployDir, remoteDeployDir))
	
	// Give it a moment to release files
	time.Sleep(1 * time.Second)

	logger.LogInfo("[DEPLOY] Uploading binary to %s...", remotePath)
	if err := core_ssh.UploadFile(client, localBinaryPath, remotePath); err != nil {
		logger.LogFatal("Failed to upload binary: %v", err)
	}
	core_ssh.RunSSHCommand(client, "chmod +x "+remotePath)

	if *service {
		logger.LogInfo("[DEPLOY] Setting up systemd service...")
		
		mavlinkEndpoint := os.Getenv("MAVLINK_ENDPOINT")
		mavlinkFlag := ""
		if mavlinkEndpoint != "" {
			mavlinkFlag = fmt.Sprintf("--mavlink %s", mavlinkEndpoint)
		}

		// Use port 80 for Tailscale (tsnet doesn't require root for port 80)
		systemdServiceContent := fmt.Sprintf(`[Unit]
Description=Dialtone Robot Service
After=network.target tailscaled.service

[Service]
ExecStart=%s robot start --hostname %s --ephemeral --web-port 80 --port 4222 --ws-port 4223 %s
WorkingDirectory=%s
Environment="TS_AUTHKEY=%s"
Restart=always
User=%s

[Install]
WantedBy=multi-user.target`, remotePath, *hostname, mavlinkFlag, remoteDeployDir, os.Getenv("TS_AUTHKEY"), *user)
		
		// Create a temporary file on the remote robot with the service content
		tempServicePath := path.Join(remoteDeployDir, fmt.Sprintf("dialtone.service.tmp.%d", time.Now().Unix()))
		err = core_ssh.WriteRemoteFile(client, tempServicePath, systemdServiceContent)
		if err != nil {
			logger.LogFatal("Failed to write temporary systemd service file: %v", err)
		}
		
		// Move the temporary file to /etc/systemd/system/ using sudo mv
		finalServicePath := "/etc/systemd/system/dialtone.service"
		sudoMoveCmd := fmt.Sprintf("printf \"%%s\\n\" \"%s\" | sudo -S -p '' mv %s %s", *pass, tempServicePath, finalServicePath)
		_, err = core_ssh.RunSSHCommand(client, sudoMoveCmd)
		if err != nil {
			logger.LogFatal("Failed to move systemd service file: %v", err)
		}
		
		// Set correct permissions on the service file
		sudoChmodCmd := fmt.Sprintf("printf \"%%s\\n\" \"%s\" | sudo -S -p '' chmod 0644 %s", *pass, finalServicePath)
		_, err = core_ssh.RunSSHCommand(client, sudoChmodCmd)
		if err != nil {
			logger.LogFatal("Failed to set permissions on systemd service file: %v", err)
		}

		// Enable and start the service, piping password to sudo
		core_ssh.RunSSHCommand(client, fmt.Sprintf("printf \"%%s\\n\" \"%s\" | sudo -S -p '' systemctl daemon-reload", *pass))
		core_ssh.RunSSHCommand(client, fmt.Sprintf("printf \"%%s\\n\" \"%s\" | sudo -S -p '' systemctl enable dialtone.service", *pass))
		core_ssh.RunSSHCommand(client, fmt.Sprintf("printf \"%%s\\n\" \"%s\" | sudo -S -p '' systemctl restart dialtone.service", *pass))
		
		logger.LogInfo("[DEPLOY] Systemd service set up and restarted.")
	} else {
		// NOT A SERVICE: Just start the binary in the background manually
		logger.LogInfo("[DEPLOY] Starting binary in background (no-service mode)...")
		
		mavlinkEndpoint := os.Getenv("MAVLINK_ENDPOINT")
		mavlinkFlag := ""
		if mavlinkEndpoint != "" {
			mavlinkFlag = fmt.Sprintf("--mavlink %s", mavlinkEndpoint)
		}

		// We use nohup and redirect output to ensure it survives SSH logout
		startCmd := fmt.Sprintf("nohup %s robot start --hostname %s --local-only --web-port 8080 --port 4222 --ws-port 4223 %s > %s/robot.log 2>&1 &", 
			remotePath, *hostname, mavlinkFlag, remoteDeployDir)
		
		core_ssh.RunSSHCommand(client, startCmd)
		logger.LogInfo("[DEPLOY] Binary started in background. Logs: %s/robot.log", remoteDeployDir)
	}

	// --- POST-DEPLOYMENT STEP-BY-STEP HEALTH CHECKS ---
	logger.LogInfo("[DEPLOY] Starting post-deployment health checks...")
	
	checkPort := 8080
	if *service {
		checkPort = 80
		
		// 1. Wait for service to be active
		timeout := 15 * time.Second
		logger.LogInfo("[DEPLOY] Step 1: Waiting for dialtone.service to be ACTIVE (Timeout: %v)...", timeout)
		active := false
		start := time.Now()
		for time.Since(start) < timeout {
			out, _ := core_ssh.RunSSHCommand(client, "systemctl is-active dialtone.service")
			if strings.TrimSpace(out) == "active" {
				active = true
				break
			}
			time.Sleep(1 * time.Second)
		}
		if !active {
			logger.LogWarn("[WARNING] dialtone.service is not active after restart")
		} else {
			logger.LogInfo("[DEPLOY] Check 1: dialtone.service is ACTIVE")
		}
	}

	// 2. Internal Web Health Check (from robot perspective)
	webTimeout := 20 * time.Second
	logger.LogInfo("[DEPLOY] Step 2: Internal Web Health Check (Timeout: %v)...", webTimeout)
	webOK := false
	webStart := time.Now()
	for time.Since(webStart) < webTimeout {
		healthCheckCmd := fmt.Sprintf("curl -s -o /dev/null -w \"%%%%{http_code}\" http://127.0.0.1:%d/health", checkPort)
		out, err := core_ssh.RunSSHCommand(client, healthCheckCmd)
		if err == nil && strings.TrimSpace(out) == "200" {
			logger.LogInfo("[DEPLOY] Check 2: Internal Web Health OK (200)")
			webOK = true
			break
		}
		time.Sleep(2 * time.Second)
	}
	if !webOK {
		logger.LogWarn("[WARNING] Internal Web Health Check failed after retries")
	}

	// 3. Internal NATS Check (using bash native dev-tcp)
	natsTimeout := 10 * time.Second
	logger.LogInfo("[DEPLOY] Step 3: Internal NATS Port 4222 Check (Timeout: %v)...", natsTimeout)
	natsOK := false
	natsStart := time.Now()
	for time.Since(natsStart) < natsTimeout {
		natsCheckCmd := "timeout 1 bash -c 'cat < /dev/null > /dev/tcp/127.0.0.1/4222' && echo OK"
		out, err := core_ssh.RunSSHCommand(client, natsCheckCmd)
		if err == nil && strings.Contains(out, "OK") {
			logger.LogInfo("[DEPLOY] Check 3: Internal NATS Port 4222 OK")
			natsOK = true
			break
		}
		time.Sleep(2 * time.Second)
	}
	if !natsOK {
		logger.LogWarn("[WARNING] Internal NATS Port Check failed")
	}

	// 4. External Reachability Check
	if *service {
		externalTimeout := 60 * time.Second
		logger.LogInfo("[DEPLOY] Step 4: Verifying Tailscale reachability (http://%s/health) (Timeout: %v)...", *hostname, externalTimeout)
		externalOK := false
		extStart := time.Now()
		for time.Since(extStart) < externalTimeout {
			externalClient := &http.Client{Timeout: 10 * time.Second}
			resp, err := externalClient.Get(fmt.Sprintf("http://%s/health", *hostname))
			if err == nil && resp.StatusCode == http.StatusOK {
				logger.LogInfo("[DEPLOY] Check 4: External Tailscale Web OK (200)")
				externalOK = true
				break
			}
			time.Sleep(5 * time.Second)
		}
		if !externalOK {
			logger.LogWarn("[WARNING] External Tailscale Web Check failed. Dumping remote logs...")
			logs, _ := core_ssh.RunSSHCommand(client, "sudo journalctl -u dialtone.service -n 50 --no-pager")
			fmt.Printf("\n--- REMOTE SERVICE LOGS ---\n%s\n--- END LOGS ---\n", logs)
		}
	} else {
		lanTimeout := 5 * time.Second
		logger.LogInfo("[DEPLOY] Step 4: Verifying LAN reachability (http://%s:8080/health) (Timeout: %v)...", *host, lanTimeout)
		externalClient := &http.Client{Timeout: lanTimeout}
		resp, err := externalClient.Get(fmt.Sprintf("http://%s:8080/health", *host))
		if err == nil && resp.StatusCode == http.StatusOK {
			logger.LogInfo("[DEPLOY] Check 4: LAN Web OK (200)")
		} else {
			logger.LogWarn("[WARNING] LAN Web Check failed: %v", err)
		}
	}

	if *proxy {
		logger.LogInfo("[DEPLOY] Setting up Cloudflare tunnel proxy via cloudflare plugin...")
		cwd, _ := os.Getwd()
		cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "cloudflare", "setup-service", "--name", *hostname)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logger.LogWarn("[WARNING] Cloudflare proxy setup failed: %v", err)
		} else {
			logger.LogInfo("[DEPLOY] Cloudflare tunnel proxy for %s.dialtone.earth setup successfully.", *hostname)
		}
	}
	
	logger.LogInfo("[DEPLOY] Deployment complete.")

	if *diagnostic {
		logger.LogInfo("[DEPLOY] Starting post-deployment diagnostic...")
		// Give the service a moment to start up on the remote
		time.Sleep(5 * time.Second)
		if err := rcli.RunDiagnostic(versionDir); err != nil {
			logger.LogError("Post-deployment diagnostic failed: %v", err)
			os.Exit(1)
		}
	}
}

func validateRequiredVars(vars []string) {
	missing := []string{}
	for _, v := range vars {
		if os.Getenv(v) == "" {
			missing = append(missing, v)
		}
	}
	if len(missing) > 0 {
		logger.LogFatal("Missing required environment variables: %s. Please check your env/.env file.", strings.Join(missing, ", "))
	}
}

func validateSudo(client *ssh.Client, pass string) {
	sudoCmd := fmt.Sprintf("echo '%s' | sudo -S true < /dev/null", pass)
	_, err := core_ssh.RunSSHCommand(client, sudoCmd)
	if err != nil {
		logger.LogFatal("Sudo validation failed")
	}
}

func setupSSHKey(client *ssh.Client, remoteUser, pass string) {
	home, _ := os.UserHomeDir()
	pubKeyPath := filepath.Join(home, ".ssh", "id_ed25519.pub")
	pubKey, err := os.ReadFile(pubKeyPath)
	if err != nil {
		// Fallback to older id_rsa.pub
		pubKeyPath = filepath.Join(home, ".ssh", "id_rsa.pub")
		pubKey, err = os.ReadFile(pubKeyPath)
	}

	if err != nil {
		logger.LogWarn("--------------------------------------------------------------------------------")
		logger.LogWarn("[WARNING] No SSH public key found on your machine.")
		logger.LogWarn("To enable seamless, passwordless deployments, please generate a key:")
		logger.LogWarn("  ssh-keygen -t ed25519 -N \"\" -f ~/.ssh/id_ed25519")
		logger.LogWarn("--------------------------------------------------------------------------------")
		return
	}
	pubKeyStr := strings.TrimSpace(string(pubKey))
	
	// Ensure ~/.ssh directory exists with correct permissions
	setupKeyCmd := fmt.Sprintf("mkdir -p ~/.ssh && chmod 700 ~/.ssh && echo '%s' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys", pubKeyStr)
	_, err = core_ssh.RunSSHCommand(client, setupKeyCmd)
	if err != nil {
		logger.LogWarn("Failed to transfer SSH public key: %v", err)
	} else {
		logger.LogInfo("Transferred SSH public key to remote authorized_keys.")
	}

	// Configure passwordless sudo for the remoteUser
	sudoersContent := fmt.Sprintf("%s ALL=(ALL) NOPASSWD: ALL", remoteUser)
	sudoersFile := fmt.Sprintf("/etc/sudoers.d/dialtone_ssh_key_sudo_%s", remoteUser)

	// Create temporary file on remote
	tempSudoersPath := path.Join("/tmp", fmt.Sprintf("dialtone_ssh_key_sudo_%s.tmp", remoteUser))
	err = core_ssh.WriteRemoteFile(client, tempSudoersPath, sudoersContent)
	if err != nil {
		logger.LogWarn("Failed to write temporary sudoers file: %v", err)
		return
	}

	// Move temporary file to /etc/sudoers.d/ using sudo
	sudoMoveCmd := fmt.Sprintf("echo '%s' | sudo -S -p '' mv %s %s && sudo chown root:root %s", pass, tempSudoersPath, sudoersFile, sudoersFile)
	_, err = core_ssh.RunSSHCommand(client, sudoMoveCmd)
	if err != nil {
		logger.LogWarn("Failed to move sudoers file into place. Passwordless sudo not configured: %v", err)
		return
	}
	
	// Set correct permissions on the sudoers file
	sudoChmodCmd := fmt.Sprintf("echo '%s' | sudo -S -p '' chmod 0440 %s", pass, sudoersFile)
	_, err = core_ssh.RunSSHCommand(client, sudoChmodCmd)
	if err != nil {
		logger.LogWarn("Failed to set permissions on sudoers file: %v", err)
		return
	}
	
	logger.LogInfo("Configured passwordless sudo for user '%s' via SSH key.", remoteUser)
}

func bumpUIVersion(versionDir string) {
	cwd, _ := os.Getwd()
	pkgJSONPath := filepath.Join(cwd, "src", "plugins", "robot", versionDir, "ui", "package.json")
	
	data, err := os.ReadFile(pkgJSONPath)
	if err != nil {
		return
	}

	var pkg map[string]any
	if err := json.Unmarshal(data, &pkg); err != nil {
		return
	}

	version, ok := pkg["version"].(string)
	if !ok {
		return
	}

	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return
	}

	newVersion := fmt.Sprintf("%s.%s.%d", parts[0], parts[1], patch+1)
	pkg["version"] = newVersion

	newData, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(pkgJSONPath, newData, 0644)
	logger.LogInfo("[Robot] Auto-bumped UI version to %s", newVersion)
}

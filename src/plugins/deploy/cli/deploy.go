package cli

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"dialtone/cli/src/core/build"
	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/ssh"

	sshlib "golang.org/x/crypto/ssh"
)

// RunDeploy handles deployment to remote robot
func RunDeploy(args []string) {
	fs := flag.NewFlagSet("deploy", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	ephemeral := fs.Bool("ephemeral", false, "Register as ephemeral node on Tailscale")
	proxy := fs.Bool("proxy", false, "Expose Web UI via Cloudflare proxy (drone-1.dialtone.earth)")
	service := fs.Bool("service", false, "Set up Dialtone as a systemd service on the robot")
	showHelp := fs.Bool("help", false, "Show help for deploy command")

	fs.Usage = func() {
		fmt.Println("Usage: dialtone deploy [options]")
		fmt.Println()
		fmt.Println("Deploy the Dialtone binary to a remote robot via SSH.")
		fmt.Println("Auto-detects remote architecture and cross-compiles locally using Podman.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --host        SSH host (user@host) [env: ROBOT_HOST]")
		fmt.Println("  --port        SSH port (default: 22)")
		fmt.Println("  --user        SSH username [env: ROBOT_USER]")
		fmt.Println("  --pass        SSH password [env: ROBOT_PASSWORD]")
		fmt.Println("  --ephemeral   Register as ephemeral node on Tailscale (default: false)")
		fmt.Println("  --proxy       Expose Web UI via Cloudflare proxy [drone-1.dialtone.earth]")
		fmt.Println("  --service     Set up as a systemd service on the robot")
		fmt.Println("  --help        Show this help message")
		fmt.Println()
	}

	fs.Parse(args)

	if *showHelp {
		fs.Usage()
		return
	}

	if *host == "" || *pass == "" {
		logger.LogFatal("Error: --host (user@host) and --pass are required for deployment")
	}

	// Validate required environment variables
	validateRequiredVars([]string{"DIALTONE_HOSTNAME", "TS_AUTHKEY"})

	deployDialtone(*host, *port, *user, *pass, *ephemeral, *proxy, *service)
}

func deployDialtone(host, port, user, pass string, ephemeral bool, proxy bool, service bool) {
	logger.LogInfo("Starting deployment to %s...", host)

	// 1. Connect to Remote
	client, err := ssh.DialSSH(host, port, user, pass)
	if err != nil {
		logger.LogFatal("Failed to connect: %v", err)
	}
	defer client.Close()

	// 2. Detect Architecture
	logger.LogInfo("Detecting remote architecture...")
	remoteArch, err := ssh.RunSSHCommand(client, "uname -m")
	if err != nil {
		logger.LogFatal("Failed to run uname -m: %v", err)
	}
	remoteArch = strings.TrimSpace(remoteArch)
	logger.LogInfo("Remote architecture: %s", remoteArch)

	var buildFlag string
	var binaryName string

	switch remoteArch {
	case "aarch64", "arm64":
		buildFlag = "--linux-arm64"
		binaryName = "dialtone-arm64"
	case "armv7l", "arm":
		buildFlag = "--linux-arm"
		binaryName = "dialtone-arm"
	case "x86_64", "amd64":
		buildFlag = "--linux-amd64"
		binaryName = "dialtone-amd64"
	default:
		logger.LogFatal("Unsupported remote architecture: %s", remoteArch)
	}

	// 3. Run Build (Cross-Compile)
	// We remove --local to allow Podman-based builds (e.g. on WSL)
	logger.LogInfo("Cross-compiling for %s...", remoteArch)
	// Skip public WWW build during deploy (not required for robot binary)
	_ = os.Setenv("DIALTONE_SKIP_WWW", "1")
	build.RunBuild([]string{buildFlag})

	localBinaryPath := filepath.Join("bin", binaryName)
	if _, err := os.Stat(localBinaryPath); os.IsNotExist(err) {
		logger.LogFatal("Build failed: binary %s not found", localBinaryPath)
	}

	// 4. Prepare Remote Directory
	remoteDir := os.Getenv("REMOTE_DIR_DEPLOY")
	if remoteDir == "" {
		home, err := ssh.GetRemoteHome(client)
		if err != nil {
			logger.LogFatal("Failed to get remote home: %v", err)
		}
		remoteDir = path.Join(home, "dialtone_deploy")
	}

	logger.LogInfo("Preparing remote directory %s...", remoteDir)
	_, _ = ssh.RunSSHCommand(client, "pkill dialtone || true")
	// Use rm -rf to ensure we can create a directory even if a file exists with the same name
	if _, err := ssh.RunSSHCommand(client, fmt.Sprintf("rm -rf %s && mkdir -p %s", remoteDir, remoteDir)); err != nil {
		logger.LogFatal("Failed to create remote directory: %v", err)
	}

	// 5. Upload Binary
	logger.LogInfo("Uploading binary %s...", localBinaryPath)
	remoteBinaryPath := path.Join(remoteDir, "dialtone")
	// Use ToSlash for cross-platform local path handling in SFTP
	if err := ssh.UploadFile(client, filepath.ToSlash(localBinaryPath), remoteBinaryPath); err != nil {
		logger.LogFatal("Failed to upload binary: %v", err)
	}
	_, _ = ssh.RunSSHCommand(client, fmt.Sprintf("chmod +x %s", remoteBinaryPath))

	// 6. Restart Service
	logger.LogInfo("Starting service...")

	hostnameParam := os.Getenv("DIALTONE_DOMAIN")
	if hostnameParam == "" {
		hostnameParam = os.Getenv("DIALTONE_HOSTNAME")
	}
	if hostnameParam == "" {
		hostnameParam = "dialtone-1"
	}

	tsAuthKey := os.Getenv("TS_AUTHKEY")
	ephemeralFlag := ""
	if ephemeral {
		ephemeralFlag = "-ephemeral"
	}

	mavlinkEndpoint := os.Getenv("MAVLINK_ENDPOINT")
	mavlinkFlag := ""
	if mavlinkEndpoint != "" {
		mavlinkFlag = fmt.Sprintf("-mavlink %s", mavlinkEndpoint)
	}

	if service {
		setupRemoteService(client, hostnameParam, tsAuthKey, ephemeralFlag, mavlinkFlag, remoteBinaryPath, user)
	} else {
		startCmd := fmt.Sprintf("rm -rf ~/dialtone && cp %s ~/dialtone && chmod +x ~/dialtone && nohup sh -c 'TS_AUTHKEY=%s ~/dialtone start -hostname %s %s %s' > ~/nats.log 2>&1 < /dev/null &", remoteBinaryPath, tsAuthKey, hostnameParam, ephemeralFlag, mavlinkFlag)

		if err := ssh.RunSSHCommandNoWait(client, startCmd); err != nil {
			logger.LogFatal("Failed to start: %v", err)
		}
	}

	verifyTailscaleAuth(client)

	logger.LogInfo("Deployment complete!")
	logger.LogInfo("Run './dialtone.sh logs --remote' to verify startup.")

	if proxy {
		if service {
			logger.LogInfo("Setting up local Cloudflare proxy service for %s.dialtone.earth...", hostnameParam)
			cwd, _ := os.Getwd()
			setupCmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "cloudflare", "setup-service", "--name", hostnameParam)
			setupCmd.Stdout = os.Stdout
			setupCmd.Stderr = os.Stderr
			if err := setupCmd.Run(); err != nil {
				logger.LogWarn("Failed to set up local proxy service: %v", err)
			}
		} else {
			logger.LogInfo("Starting background Cloudflare proxy for %s.dialtone.earth...", hostnameParam)
			cwd, _ := os.Getwd()
			cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "cloudflare", "robot", hostnameParam)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			if err := cmd.Start(); err != nil {
				logger.LogWarn("Failed to start Cloudflare proxy: %v", err)
			} else {
				logger.LogInfo("Cloudflare proxy started (PID: %d)", cmd.Process.Pid)
			}
		}
	} else {
		logger.LogInfo("")
		logger.LogInfo("To expose this robot's Web UI via Cloudflare subdomain, run:")
		logger.LogInfo("  ./dialtone.sh cloudflare robot %s", hostnameParam)
	}

	// 7. Verification
	verifyDeployment(client, hostnameParam, proxy, service)
}

func verifyDeployment(client *sshlib.Client, hostname string, proxy, service bool) {
	logger.LogInfo("--- VERIFICATION ---")

	// 1. Remote Service Check
	if service {
		out, err := ssh.RunSSHCommand(client, "systemctl is-active dialtone.service")
		if err == nil && strings.TrimSpace(out) == "active" {
			logger.LogInfo("[VERIFY] Remote dialtone.service is ACTIVE")
		} else {
			logger.LogWarn("[VERIFY] Remote dialtone.service is NOT ACTIVE (status: %s)", strings.TrimSpace(out))
		}
	}

	// 2. Local Service Check
	if service && proxy {
		cmd := exec.Command("systemctl", "is-active", fmt.Sprintf("dialtone-proxy-%s.service", hostname))
		out, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(out)) == "active" {
			logger.LogInfo("[VERIFY] Local dialtone-proxy-%s.service is ACTIVE", hostname)
		} else {
			logger.LogWarn("[VERIFY] Local proxy service is NOT ACTIVE (status: %s)", strings.TrimSpace(string(out)))
		}
	}

	// 3. Web UI Check
	if proxy {
		url := fmt.Sprintf("https://%s.dialtone.earth", hostname)
		logger.LogInfo("Checking Web UI at %s...", url)

		// Give it a moment to stabilize
		time.Sleep(2 * time.Second)

		httpClient := &http.Client{Timeout: 5 * time.Second}
		resp, err := httpClient.Get(url)
		if err != nil {
			logger.LogWarn("[VERIFY] Failed to reach Cloudflare URL: %v", err)
		} else {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)
			if strings.Contains(bodyStr, "v1.1.1") {
				logger.LogInfo("[VERIFY] Web UI is LIVE and running v1.1.1")
			} else {
				logger.LogWarn("[VERIFY] Web UI is accessible but version string 'v1.1.1' not found")
			}
		}
	}
}

func setupRemoteService(client *sshlib.Client, hostname, tsAuthKey, ephemeralFlag, mavlinkFlag, binaryPath, user string) {
	logger.LogInfo("Setting up systemd service on robot...")

	remoteDialtonePath := "/home/" + user + "/dialtone"
	_, _ = ssh.RunSSHCommand(client, fmt.Sprintf("cp %s %s && chmod +x %s", binaryPath, remoteDialtonePath, remoteDialtonePath))

	serviceTemplate := `[Unit]
Description=Dialtone Robot Service
After=network.target

[Service]
ExecStart=%s start -hostname %s %s %s
WorkingDirectory=/home/%s
User=%s
Environment=TS_AUTHKEY=%s
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`
	serviceContent := fmt.Sprintf(serviceTemplate, remoteDialtonePath, hostname, ephemeralFlag, mavlinkFlag, user, user, tsAuthKey)
	tmpPath := "/tmp/dialtone.service"
	servicePath := "/etc/systemd/system/dialtone.service"

	err := ssh.WriteRemoteFile(client, tmpPath, serviceContent)
	if err != nil {
		logger.LogFatal("Failed to write remote service file: %v", err)
	}

	commands := []string{
		fmt.Sprintf("sudo cp %s %s", tmpPath, servicePath),
		"sudo systemctl daemon-reload",
		"sudo systemctl enable dialtone.service",
		"sudo systemctl restart dialtone.service",
	}

	for _, cmd := range commands {
		_, err := ssh.RunSSHCommand(client, cmd)
		if err != nil {
			logger.LogFatal("Failed to execute command on robot: %s, error: %v", cmd, err)
		}
	}

	logger.LogInfo("SUCCESS: Systemd service installed and started on robot.")
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

func verifyTailscaleAuth(client *sshlib.Client) {
	logger.LogInfo("Verifying Tailscale auth key...")

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		logOutput, err := ssh.RunSSHCommand(client, "tail -n 200 ~/nats.log")
		if err != nil {
			logger.LogFatal("Failed to read remote logs for Tailscale verification: %v", err)
		}

		if reason := tsnetFailureReason(logOutput); reason != "" {
			logger.LogFatal("Tailscale auth failed: %s\nRecent log output:\n%s", reason, tailLines(logOutput, 30))
		}
		if tsnetSuccess(logOutput) {
			logger.LogInfo("Tailscale auth key verified.")
			return
		}

		time.Sleep(2 * time.Second)
	}

	logger.LogInfo("Tailscale auth key verification pending (no failures detected yet).")
	logger.LogInfo("If startup looks stalled, check logs with './dialtone.sh logs --remote'.")
}

func tsnetFailureReason(logOutput string) string {
	failures := map[string]string{
		"TS_AUTHKEY environment variable is not set": "TS_AUTHKEY is missing on the remote process",
		"Failed to connect to Tailscale":             "failed to connect to Tailscale (invalid or expired auth key)",
		"Timed out waiting for Tailscale IP":         "timed out waiting for a Tailscale IP",
	}

	for needle, reason := range failures {
		if strings.Contains(logOutput, needle) {
			return reason
		}
	}

	if strings.Contains(logOutput, "[FATAL]") && strings.Contains(strings.ToLower(logOutput), "tailscale") {
		return "fatal Tailscale error (see logs)"
	}

	return ""
}

func tsnetSuccess(logOutput string) bool {
	return strings.Contains(logOutput, "TSNet: Connected") ||
		strings.Contains(logOutput, "Connected to Tailscale") ||
		strings.Contains(logOutput, "Tailscale IP")
}

func tailLines(output string, maxLines int) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-maxLines:], "\n")
}

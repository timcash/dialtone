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

	"dialtone/dev/build"
	"dialtone/dev/plugins/logs/src_v1/go"
	"dialtone/dev/ssh"

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
		logs.Fatal("Error: --host (user@host) and --pass are required for deployment")
	}

	// Validate required environment variables
	validateRequiredVars([]string{"DIALTONE_HOSTNAME", "TS_AUTHKEY"})

	deployDialtone(*host, *port, *user, *pass, *ephemeral, *proxy, *service)
}

func deployDialtone(host, port, user, pass string, ephemeral bool, proxy bool, service bool) {
	logs.Info("Starting deployment to %s...", host)

	// 1. Connect to Remote
	client, err := ssh.DialSSH(host, port, user, pass)
	if err != nil {
		logs.Fatal("Failed to connect: %v", err)
	}
	defer client.Close()

	// 2. Validate Sudo & Setup SSH Key
	validateSudo(client, pass)
	setupSSHKey(client)

	// 3. Detect Architecture
	logs.Info("Detecting remote architecture...")
	remoteArch, err := ssh.RunSSHCommand(client, "uname -m")
	if err != nil {
		logs.Fatal("Failed to run uname -m: %v", err)
	}
	remoteArch = strings.TrimSpace(remoteArch)
	logs.Info("Remote architecture: %s", remoteArch)

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
		logs.Fatal("Unsupported remote architecture: %s", remoteArch)
	}

	// 3. Run Build (Cross-Compile)
	// We remove --local to allow Podman-based builds (e.g. on WSL)
	logs.Info("Cross-compiling for %s...", remoteArch)
	// Skip public WWW build during deploy (not required for robot binary)
	_ = os.Setenv("DIALTONE_SKIP_WWW", "1")
	build.RunBuild([]string{buildFlag})

	localBinaryPath := filepath.Join("bin", binaryName)
	if _, err := os.Stat(localBinaryPath); os.IsNotExist(err) {
		logs.Fatal("Build failed: binary %s not found", localBinaryPath)
	}

	// 4. Prepare Remote Directory
	remoteDir := os.Getenv("REMOTE_DIR_DEPLOY")
	if remoteDir == "" {
		home, err := ssh.GetRemoteHome(client)
		if err != nil {
			logs.Fatal("Failed to get remote home: %v", err)
		}
		remoteDir = path.Join(home, "dialtone_deploy")
	}

	logs.Info("Preparing remote directory %s...", remoteDir)
	_, _ = ssh.RunSSHCommand(client, "pkill dialtone || true")
	// Use rm -rf to ensure we can create a directory even if a file exists with the same name
	if _, err := ssh.RunSSHCommand(client, fmt.Sprintf("rm -rf %s && mkdir -p %s", remoteDir, remoteDir)); err != nil {
		logs.Fatal("Failed to create remote directory: %v", err)
	}

	// 5. Upload Binary
	logs.Info("Uploading binary %s...", localBinaryPath)
	remoteBinaryPath := path.Join(remoteDir, "dialtone")
	// Use ToSlash for cross-platform local path handling in SFTP
	if err := ssh.UploadFile(client, filepath.ToSlash(localBinaryPath), remoteBinaryPath); err != nil {
		logs.Fatal("Failed to upload binary: %v", err)
	}
	_, _ = ssh.RunSSHCommand(client, fmt.Sprintf("chmod +x %s", remoteBinaryPath))

	// 6. Restart Service
	logs.Info("Starting service...")

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
			logs.Fatal("Failed to start: %v", err)
		}
	}

	verifyTailscaleAuth(client)

	logs.Info("Deployment complete!")
	logs.Info("Run './dialtone.sh logs --remote' to verify startup.")

	if proxy {
		if service {
			logs.Info("Setting up local Cloudflare proxy service for %s.dialtone.earth...", hostnameParam)
			cwd, _ := os.Getwd()
			setupCmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "cloudflare", "setup-service", "--name", hostnameParam)
			setupCmd.Stdout = os.Stdout
			setupCmd.Stderr = os.Stderr
			if err := setupCmd.Run(); err != nil {
				logs.Warn("Failed to set up local proxy service: %v", err)
			}
		} else {
			logs.Info("Starting background Cloudflare proxy for %s.dialtone.earth...", hostnameParam)
			cwd, _ := os.Getwd()
			cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "cloudflare", "robot", hostnameParam)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			if err := cmd.Start(); err != nil {
				logs.Warn("Failed to start Cloudflare proxy: %v", err)
			} else {
				logs.Info("Cloudflare proxy started (PID: %d)", cmd.Process.Pid)
			}
		}
	} else {
		logs.Info("")
		logs.Info("To expose this robot's Web UI via Cloudflare subdomain, run:")
		logs.Info("  ./dialtone.sh cloudflare robot %s", hostnameParam)
	}

	// 7. Verification
	if err := verifyDeployment(client, hostnameParam, proxy, service); err != nil {
		logs.Fatal("Deployment verification FAILED: %v", err)
	}
	logs.Info("Deployment verification SUCCESS.")
}

func verifyDeployment(client *sshlib.Client, hostname string, proxy, service bool) error {
	logs.Info("--- VERIFICATION ---")

	// 1. Remote Service Check
	if service {
		logs.Info("Verifying remote dialtone.service...")
		success := false
		var lastOut string
		for i := 0; i < 5; i++ {
			out, err := ssh.RunSSHCommand(client, "systemctl is-active dialtone.service")
			lastOut = strings.TrimSpace(out)
			if err == nil && lastOut == "active" {
				logs.Info("[VERIFY] Remote dialtone.service is ACTIVE")
				success = true
				break
			}
			time.Sleep(1 * time.Second)
		}
		if !success {
			return fmt.Errorf("remote dialtone.service is NOT ACTIVE (status: %s)", lastOut)
		}
	}

	// 2. Local Service Check
	if service && proxy {
		serviceName := fmt.Sprintf("dialtone-proxy-%s.service", hostname)
		logs.Info("Verifying local %s...", serviceName)
		success := false
		var lastOut string
		for i := 0; i < 5; i++ {
			cmd := exec.Command("systemctl", "is-active", serviceName)
			out, err := cmd.Output()
			lastOut = strings.TrimSpace(string(out))
			if err == nil && lastOut == "active" {
				logs.Info("[VERIFY] Local %s is ACTIVE", serviceName)
				success = true
				break
			}
			time.Sleep(1 * time.Second)
		}
		if !success {
			return fmt.Errorf("local proxy service %s is NOT ACTIVE (status: %s)", serviceName, lastOut)
		}
	}

	// 3. Web UI Check
	if proxy {
		url := fmt.Sprintf("https://%s.dialtone.earth", hostname)
		logs.Info("Checking Web UI at %s...", url)

		success := false
		var lastErr error
		for i := 0; i < 10; i++ {
			httpClient := &http.Client{Timeout: 5 * time.Second}
			resp, err := httpClient.Get(url)
			if err != nil {
				lastErr = err
			} else {
				defer resp.Body.Close()
				body, _ := io.ReadAll(resp.Body)
				bodyStr := string(body)
				if strings.Contains(bodyStr, "v1.1.1") {
					logs.Info("[VERIFY] Web UI is LIVE and running v1.1.1")
					success = true
					break
				} else {
					lastErr = fmt.Errorf("version string 'v1.1.1' not found in response")
				}
			}
			time.Sleep(2 * time.Second)
		}
		if !success {
			return fmt.Errorf("failed to verify Web UI at %s: %v", url, lastErr)
		}
	}

	return nil
}

func setupRemoteService(client *sshlib.Client, hostname, tsAuthKey, ephemeralFlag, mavlinkFlag, binaryPath, user string) {
	logs.Info("Setting up systemd service on robot...")

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
		logs.Fatal("Failed to write remote service file: %v", err)
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
			logs.Fatal("Failed to execute command on robot: %s, error: %v", cmd, err)
		}
	}

	logs.Info("SUCCESS: Systemd service installed and started on robot.")
}

func validateRequiredVars(vars []string) {
	missing := []string{}
	for _, v := range vars {
		if os.Getenv(v) == "" {
			missing = append(missing, v)
		}
	}
	if len(missing) > 0 {
		logs.Fatal("Missing required environment variables: %s. Please check your env/.env file.", strings.Join(missing, ", "))
	}
}

func validateSudo(client *sshlib.Client, pass string) {
	logs.Info("Validating sudo access on robot...")

	// 1. Try to run a simple sudo command using the password
	// We use sudo -S to read from stdin
	sudoCmd := fmt.Sprintf("echo '%s' | sudo -S true", pass)
	_, err := ssh.RunSSHCommand(client, sudoCmd)
	if err != nil {
		logs.Fatal("Sudo validation FAILED: User does not have sudo rights or password is incorrect. Error: %v", err)
	}

	// 2. Automatically configure passwordless sudo for this user to make automation smoother
	// This only works if we already have sudo rights (which we just verified)
	user, _ := ssh.RunSSHCommand(client, "whoami")
	user = strings.TrimSpace(user)

	logs.Info("Configuring passwordless sudo for user '%s'...", user)
	sudoersLine := fmt.Sprintf("%s ALL=(ALL) NOPASSWD:ALL", user)
	// Append to a new file in /etc/sudoers.d/
	setupSudoers := fmt.Sprintf("echo '%s' | sudo -S sh -c \"echo '%s' > /etc/sudoers.d/dialtone-%s && chmod 0440 /etc/sudoers.d/dialtone-%s\"", pass, sudoersLine, user, user)
	_, err = ssh.RunSSHCommand(client, setupSudoers)
	if err != nil {
		logs.Warn("Warning: Failed to configure passwordless sudo: %v", err)
	} else {
		logs.Info("Passwordless sudo configured.")
	}
}

func setupSSHKey(client *sshlib.Client) {
	logs.Info("Ensuring SSH key access on robot...")

	home, err := os.UserHomeDir()
	if err != nil {
		logs.Warn("Could not find local home directory: %v", err)
		return
	}

	// Check for common public key paths
	keyPaths := []string{
		filepath.Join(home, ".ssh", "id_ed25519.pub"),
		filepath.Join(home, ".ssh", "id_rsa.pub"),
	}

	var pubKey []byte
	for _, path := range keyPaths {
		data, err := os.ReadFile(path)
		if err == nil {
			pubKey = data
			break
		}
	}

	if len(pubKey) == 0 {
		logs.Warn("No local SSH public key found (checked id_ed25519 and id_rsa). Skipping key setup.")
		return
	}

	pubKeyStr := strings.TrimSpace(string(pubKey))
	logs.Info("Uploading public key to robot...")

	setupKeyCmd := fmt.Sprintf("mkdir -p ~/.ssh && chmod 700 ~/.ssh && echo '%s' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys", pubKeyStr)
	_, err = ssh.RunSSHCommand(client, setupKeyCmd)
	if err != nil {
		logs.Warn("Failed to upload SSH key: %v", err)
	} else {
		logs.Info("SSH key successfully added to robot.")
	}
}

func verifyTailscaleAuth(client *sshlib.Client) {
	logs.Info("Verifying Tailscale auth key...")

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		logOutput, err := ssh.RunSSHCommand(client, "tail -n 200 ~/nats.log")
		if err != nil {
			logs.Fatal("Failed to read remote logs for Tailscale verification: %v", err)
		}

		if reason := tsnetFailureReason(logOutput); reason != "" {
			logs.Fatal("Tailscale auth failed: %s\nRecent log output:\n%s", reason, tailLines(logOutput, 30))
		}
		if tsnetSuccess(logOutput) {
			logs.Info("Tailscale auth key verified.")
			return
		}

		time.Sleep(2 * time.Second)
	}

	logs.Info("Tailscale auth key verification pending (no failures detected yet).")
	logs.Info("If startup looks stalled, check logs with './dialtone.sh logs --remote'.")
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

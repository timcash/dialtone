package cli

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/ssh"
	build_cli "dialtone/cli/src/plugins/build/cli"
)

// RunDeploy handles deployment to remote robot
func RunDeploy(args []string) {
	fs := flag.NewFlagSet("deploy", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	ephemeral := fs.Bool("ephemeral", false, "Register as ephemeral node on Tailscale")
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
		fmt.Println("  --ephemeral   Register as ephemeral node on Tailscale (default: true)")
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

	deployDialtone(*host, *port, *user, *pass, *ephemeral)
}

func deployDialtone(host, port, user, pass string, ephemeral bool) {
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
	default:
		logger.LogFatal("Unsupported remote architecture: %s", remoteArch)
	}

	// 3. Run Build (Cross-Compile)
	logger.LogInfo("Cross-compiling for %s...", remoteArch)
	build_cli.RunBuild([]string{"--local", buildFlag})

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
	_, _ = ssh.RunSSHCommand(client, fmt.Sprintf("mkdir -p %s", remoteDir))

	// 5. Upload Binary
	logger.LogInfo("Uploading binary %s...", localBinaryPath)
	remoteBinaryPath := path.Join(remoteDir, "dialtone")
	if err := ssh.UploadFile(client, localBinaryPath, remoteBinaryPath); err != nil {
		logger.LogFatal("Failed to upload binary: %v", err)
	}
	_, _ = ssh.RunSSHCommand(client, fmt.Sprintf("chmod +x %s", remoteBinaryPath))

	// 6. Restart Service
	logger.LogInfo("Starting service...")

	hostnameParam := os.Getenv("DIALTONE_HOSTNAME")
	if hostnameParam == "" {
		hostnameParam = "dialtone-1"
	}

	tsAuthKey := os.Getenv("TS_AUTHKEY")
	opencodeKey := os.Getenv("OPENCODE_API_KEY")
	ephemeralFlag := ""
	if ephemeral {
		ephemeralFlag = "-ephemeral"
	}

	mavlinkEndpoint := os.Getenv("MAVLINK_ENDPOINT")
	mavlinkFlag := ""
	if mavlinkEndpoint != "" {
		mavlinkFlag = fmt.Sprintf("-mavlink %s", mavlinkEndpoint)
	}

	// Always start opencode for now as requested
	opencodeFlag := "-opencode"

	startCmd := fmt.Sprintf("rm -rf ~/dialtone && cp %s ~/dialtone && chmod +x ~/dialtone && nohup sh -c 'TS_AUTHKEY=%s OPENCODE_API_KEY=%s ~/dialtone start -hostname %s %s %s %s' > ~/nats.log 2>&1 < /dev/null &", remoteBinaryPath, tsAuthKey, opencodeKey, hostnameParam, ephemeralFlag, mavlinkFlag, opencodeFlag)

	if err := ssh.RunSSHCommandNoWait(client, startCmd); err != nil {
		logger.LogFatal("Failed to start: %v", err)
	}

	logger.LogInfo("Deployment complete!")
	logger.LogInfo("Robot Dashboard: http://%s", hostnameParam)
	logger.LogInfo("Opencode Web UI: http://%s:3000", hostnameParam)
	logger.LogInfo("Run './dialtone.sh logs --remote' to verify startup.")
}

func validateRequiredVars(vars []string) {
	missing := []string{}
	for _, v := range vars {
		if os.Getenv(v) == "" {
			missing = append(missing, v)
		}
	}
	if len(missing) > 0 {
		logger.LogFatal("Missing required environment variables: %s. Please check your .env file.", strings.Join(missing, ", "))
	}
}

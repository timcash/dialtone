package dialtone

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
)

// RunDeploy handles deployment to remote robot
func RunDeploy(args []string) {
	fs := flag.NewFlagSet("deploy", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	ephemeral := fs.Bool("ephemeral", true, "Register as ephemeral node on Tailscale")
	showHelp := fs.Bool("help", false, "Show help for deploy command")

	fs.Usage = func() {
		fmt.Println("Usage: dialtone deploy [options]")
		fmt.Println()
		fmt.Println("Deploy the Dialtone binary to a remote robot via SSH.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --host        SSH host (user@host) [env: ROBOT_HOST]")
		fmt.Println("  --port        SSH port (default: 22)")
		fmt.Println("  --user        SSH username [env: ROBOT_USER]")
		fmt.Println("  --pass        SSH password [env: ROBOT_PASSWORD]")
		fmt.Println("  --ephemeral   Register as ephemeral node on Tailscale (default: true)")
		fmt.Println("  --help        Show this help message")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  dialtone deploy --host pi@192.168.1.100 --pass mypassword")
		fmt.Println("  dialtone deploy   # Uses ROBOT_HOST, ROBOT_PASSWORD from .env")
		fmt.Println()
		fmt.Println("Notes:")
		fmt.Println("  - Builds ARM64 binary if not already present (bin/dialtone-arm64)")
		fmt.Println("  - Auto-provisions TS_AUTHKEY if TS_API_KEY is set")
		fmt.Println("  - Requires REMOTE_DIR_SRC, REMOTE_DIR_DEPLOY, DIALTONE_HOSTNAME, TS_AUTHKEY")
	}

	fs.Parse(args)

	if *showHelp {
		fs.Usage()
		return
	}

	// If TS_API_KEY is available, provision a fresh key before deployment
	if os.Getenv("TS_API_KEY") != "" {
		LogInfo("TS_API_KEY found, auto-provisioning fresh TS_AUTHKEY for deployment...")
		provisionKey(os.Getenv("TS_API_KEY"))
	}

	if *host == "" || *pass == "" {
		LogFatal("Error: --host (user@host) and --pass are required for deployment")
	}

	validateRequiredVars([]string{"REMOTE_DIR_SRC", "REMOTE_DIR_DEPLOY", "DIALTONE_HOSTNAME", "TS_AUTHKEY"})

	deployDialtone(*host, *port, *user, *pass, *ephemeral)
}

func deployDialtone(host, port, user, pass string, ephemeral bool) {
	LogInfo("Starting deployment of Dialtone (Remote Build)...")

	localBinary := "bin/dialtone-arm64"
	usePrebuilt := false
	if _, err := os.Stat(localBinary); err == nil {
		usePrebuilt = true
		LogInfo("Found pre-built binary, using it for deployment.")
	}

	client, err := dialSSH(host, port, user, pass)
	if err != nil {
		LogFatal("Failed to connect: %v", err)
	}
	defer client.Close()

	if usePrebuilt {
		remoteDir := os.Getenv("REMOTE_DIR_DEPLOY")
		if remoteDir == "" {
			home, err := getRemoteHome(client)
			if err != nil {
				LogFatal("Failed to get remote home: %v", err)
			}
			remoteDir = path.Join(home, "dialtone_deploy")
		}
		LogInfo("Cleaning and creating remote directory %s...", remoteDir)
		_, _ = runSSHCommand(client, fmt.Sprintf("rm -rf %s && mkdir -p %s", remoteDir, remoteDir))

		LogInfo("Uploading pre-built binary %s...", localBinary)
		remotePath := path.Join(remoteDir, "dialtone")
		if err := uploadFile(client, localBinary, remotePath); err != nil {
			LogFatal("Failed to upload binary: %v", err)
		}
	} else {
		remoteDir := os.Getenv("REMOTE_DIR_SRC")
		if remoteDir == "" {
			home, err := getRemoteHome(client)
			if err != nil {
				LogFatal("Failed to get remote home: %v", err)
			}
			remoteDir = path.Join(home, "dialtone_src")
		}
		LogInfo("Cleaning and creating remote directory %s...", remoteDir)
		_, _ = runSSHCommand(client, fmt.Sprintf("rm -rf %s && mkdir -p %s/src", remoteDir, remoteDir))

		filesToUpload := []string{"go.mod", "go.sum", "dialtone.go"}
		srcFiles, _ := filepath.Glob("src/*.go")
		for _, f := range srcFiles {
			filesToUpload = append(filesToUpload, f)
		}

		for _, file := range filesToUpload {
			LogInfo("Uploading %s...", file)
			remotePath := path.Join(remoteDir, file)
			// Ensure parent directory exists on remote
			parentDir := filepath.ToSlash(path.Dir(remotePath))
			_, _ = runSSHCommand(client, fmt.Sprintf("mkdir -p %s", parentDir))
			if err := uploadFile(client, file, remotePath); err != nil {
				LogFatal("Failed to upload %s: %v", file, err)
			}
		}

		// Also upload web_build if it exists
		webBuildDir := filepath.Join("src", "web_build")
		if _, err := os.Stat(webBuildDir); err == nil {
			LogInfo("Uploading web assets...")
			uploadDir(client, webBuildDir, path.Join(remoteDir, "src", "web_build"))
		}

		LogInfo("Building on Raspberry Pi...")
		buildCmd := fmt.Sprintf(`
			export PATH=$PATH:/usr/local/go/bin
			cd %s
			/usr/local/go/bin/go build -v -o dialtone .
		`, remoteDir)
		output, err := runSSHCommand(client, buildCmd)
		if err != nil {
			LogFatal("Remote build failed: %v\nOutput: %s", err, output)
		}
	}

	LogInfo("Stopping remote dialtone...")
	_, _ = runSSHCommand(client, "pkill dialtone || true")

	LogInfo("Starting service...")
	remoteBaseDir := os.Getenv("REMOTE_DIR_SRC")
	if usePrebuilt {
		remoteBaseDir = os.Getenv("REMOTE_DIR_DEPLOY")
	}
	remoteBinaryPath := path.Join(remoteBaseDir, "dialtone")

	hostnameParam := os.Getenv("DIALTONE_HOSTNAME")
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

	startCmd := fmt.Sprintf("rm -rf ~/dialtone && cp %s ~/dialtone && chmod +x ~/dialtone && nohup sh -c 'TS_AUTHKEY=%s ~/dialtone start -hostname %s %s %s' > ~/nats.log 2>&1 < /dev/null &", remoteBinaryPath, tsAuthKey, hostnameParam, ephemeralFlag, mavlinkFlag)

	if err := runSSHCommandNoWait(client, startCmd); err != nil {
		LogFatal("Failed to start: %v", err)
	}

	LogInfo("Deployment complete!")
}

package robot

import (
	"context"
	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/util"
	"dialtone/cli/src/core/web"
	"dialtone/cli/src/core/mock"
	"dialtone/cli/src/core/build"
	"dialtone/cli/src/core/ssh"
	mavlink_app "dialtone/cli/src/plugins/mavlink/app"
	rcli "dialtone/cli/src/plugins/robot/robot_cli"
	robot_ops "dialtone/cli/src/plugins/robot/src_v1/cmd/ops"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bluenviron/gomavlib/v3/pkg/dialects/common"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"tailscale.com/tsnet"
	sshlib "golang.org/x/crypto/ssh"
	"embed"
	"io/fs"
)

//go:embed all:src_v1/ui/dist
var webFS embed.FS

// RunRobot handles 'robot <subcommand>'
func RunRobot(args []string) {
	if len(args) == 0 {
		printRobotUsage()
		return
	}

	subcommand := args[0]
	restArgs := args[1:]

	// Helper to get directory with latest default
	getDir := func() string {
		if len(args) > 1 && strings.HasPrefix(args[1], "src_v") {
			return args[1]
		}
		return getLatestVersionDir()
	}

	switch subcommand {
	case "start":
		RunStart(restArgs)
	case "deploy":
		RunDeploy(restArgs)
	case "deploy-test":
		vDir := getDir()
		cmdArgs := restArgs
		if len(restArgs) > 0 && restArgs[0] == vDir {
			cmdArgs = restArgs[1:]
		}
		if err := rcli.RunDeployTest(vDir, cmdArgs); err != nil {
			fmt.Printf("Robot deploy-test error: %v\n", err)
			os.Exit(1)
		}
	case "vpn-test":
		if err := rcli.RunVPNTest(restArgs); err != nil {
			fmt.Printf("Robot vpn-test error: %v\n", err)
			os.Exit(1)
		}
	case "install":
		if err := robot_ops.Install(); err != nil {
			fmt.Printf("Robot install error: %v\n", err)
			os.Exit(1)
		}
	case "fmt":
		RunFmt(getDir())
	case "format":
		RunFormat(getDir())
	case "vet":
		RunVet(getDir())
	case "go-build":
		RunGoBuild(getDir())
	case "lint":
		RunLint(getDir())
	case "serve":
		RunServe(getDir())
	case "ui-run":
		RunUIRun(getDir(), args[2:])
	case "dev":
		RunDev(getDir())
	case "build":
		if err := robot_ops.Build(); err != nil {
			fmt.Printf("Robot build error: %v\n", err)
			os.Exit(1)
		}
	case "test":
		if len(restArgs) > 0 && strings.HasPrefix(restArgs[0], "src_v") {
			RunVersionedTest(restArgs[0])
		} else {
			fmt.Println("Usage: dialtone robot test src_vN")
		}
	case "diagnostic":
		if len(restArgs) == 0 {
			fmt.Println("Usage: dialtone robot diagnostic <src_vN>")
			os.Exit(1)
		}
		if err := rcli.RunDiagnostic(restArgs[0]); err != nil {
			fmt.Printf("Robot diagnostic error: %v\n", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		printRobotUsage()
	default:
		fmt.Printf("Unknown robot subcommand: %s\n", subcommand)
		printRobotUsage()
	}
}

func printRobotUsage() {
	fmt.Println("Usage: dialtone robot <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  start       Start the NATS and Web server (core robot logic)")
	fmt.Println("  deploy      Deploy binary to remote robot via SSH")
	fmt.Println("  deploy-test Step-by-step remote verification using debug binaries")
	fmt.Println("  vpn-test    Test Tailscale (tsnet) connectivity")
	fmt.Println("\nVersioned Source Commands (src_vN):")
	fmt.Println("  install     Install UI dependencies")
	fmt.Println("  fmt         Run formatting checks/fixes")
	fmt.Println("  format      Run UI format checks")
	fmt.Println("  vet         Run go vet checks")
	fmt.Println("  go-build    Run go build checks")
	fmt.Println("  lint        Run lint checks")
	fmt.Println("  dev         Run UI in development mode")
	fmt.Println("  ui-run      Run UI dev server")
	fmt.Println("  build       Build everything needed (UI assets)")
	fmt.Println("  serve       Run the plugin Go server")
	fmt.Println("  test        Run automated test suite")
	fmt.Println("  diagnostic  Run UI diagnostic against a deployed robot")
}

func getLatestVersionDir() string {
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "robot")
	entries, _ := os.ReadDir(pluginDir)
	maxVer := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "src_v") {
			ver, _ := strconv.Atoi(e.Name()[5:])
			if ver > maxVer {
				maxVer = ver
			}
		}
	}
	if maxVer == 0 {
		return "src_v1"
	}
	return fmt.Sprintf("src_v%d", maxVer)
}

// --- Legacy / Core Robot Logic ---

func RunStart(args []string) {
	// Check if running under systemd
	if os.Getenv("INVOCATION_ID") == "" {
		logger.LogInfo("[WARNING] Process is not running as a systemd service. Consider running via systemctl.")
	} else {
		logger.LogInfo("[INFO] Running as systemd service.")
	}

	fs := flag.NewFlagSet("start", flag.ExitOnError)
	defaultHostname := os.Getenv("DIALTONE_HOSTNAME")
	if defaultHostname == "" {
		defaultHostname = "dialtone-1"
	}
	hostname := fs.String("hostname", defaultHostname, "Tailscale hostname for this NATS server")
	natsPort := fs.Int("port", 4222, "NATS port to listen on (both local and Tailscale)")
	webPort := fs.Int("web-port", 80, "Web dashboard port")
	stateDir := fs.String("state-dir", "", "Directory to store Tailscale state")
	ephemeral := fs.Bool("ephemeral", false, "Register as ephemeral node")
	localOnly := fs.Bool("local-only", false, "Run without Tailscale")
	wsPort := fs.Int("ws-port", 4223, "NATS WebSocket port")
	verbose := fs.Bool("verbose", false, "Enable verbose logging")
	mavlinkAddr := fs.String("mavlink", "", "Mavlink connection string")
	useMock := fs.Bool("mock", false, "Use mock telemetry and camera data")
	fs.Parse(args)

	if *stateDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			logger.LogFatal("Failed to get home directory: %v", err)
		}
		*stateDir = filepath.Join(homeDir, ".config", "dialtone")
	}

	if *localOnly || os.Getenv("TS_AUTHKEY") == "" {
		if os.Getenv("TS_AUTHKEY") == "" && !*localOnly {
			logger.LogInfo("TS_AUTHKEY missing, falling back to local-only mode.")
		}
		runLocalOnly(*natsPort, *wsPort, *webPort, *verbose, *mavlinkAddr, *useMock, *hostname)
		return
	}

	runWithTailscale(*hostname, *natsPort, *wsPort, *webPort, *stateDir, *ephemeral, *verbose, *mavlinkAddr, *useMock)
}

func runLocalOnly(port, wsPort, webPort int, verbose bool, mavlinkAddr string, useMock bool, hostname string) {
	// Use 0.0.0.0 to ensure local access works without Tailscale
	ns := startNATSServer("0.0.0.0", port, wsPort, verbose)
	defer ns.Shutdown()

	logger.LogInfo("NATS server started on port %d (local only)", port)

	if useMock {
		go mock.StartMockMavlink(port)
	} else if mavlinkAddr != "" {
		startMavlink(mavlinkAddr, port)
	}

	startNatsPublisher(port)

	webHandler := web.CreateWebHandler(hostname, port, wsPort, webPort, port, wsPort, ns, nil, nil, useMock, webFS)
	if sub, err := fs.Sub(webFS, "src_v1/ui/dist"); err == nil {
		webHandler = web.CreateWebHandler(hostname, port, wsPort, webPort, port, wsPort, ns, nil, nil, useMock, sub)
	}
	
	// Local web listener
	localWebAddr := fmt.Sprintf("0.0.0.0:%d", webPort)
	if webPort == 80 {
		// Try port 80 but fallback to 8080 if it fails (likely permission denied)
		logger.LogInfo("Web UI (Local Only): Attempting port 80...")
		go http.ListenAndServe(":80", webHandler)
		localWebAddr = "0.0.0.0:8080"
	}

	logger.LogInfo("Web UI (Local Only): Serving at http://%s", localWebAddr)
	http.ListenAndServe(localWebAddr, webHandler)
}

func runWithTailscale(hostname string, port, wsPort, webPort int, stateDir string, ephemeral, verbose bool, mavlinkAddr string, useMock bool) {
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		logger.LogFatal("Failed to create state directory: %v", err)
	}

	ts := &tsnet.Server{
		Hostname:  hostname,
		Dir:       stateDir,
		Ephemeral: ephemeral,
		AuthKey:   os.Getenv("TS_AUTHKEY"),
		UserLogf:  logger.LogInfo,
	}
	if verbose {
		ts.Logf = logger.LogInfo
	}

	util.CheckStaleHostname(hostname)

	if os.Getenv("TS_AUTHKEY") == "" {
		logger.LogFatal("ERROR: TS_AUTHKEY environment variable is not set.")
	}

	logger.LogInfo("Connecting to Tailscale...")
	status, err := ts.Up(context.Background())
	if err != nil {
		logger.LogFatal("Failed to connect to Tailscale: %v", err)
	}
	defer ts.Close()

	var ips []netip.Addr
	ips = status.TailscaleIPs
	ipStr := "none"
	if len(ips) > 0 {
		ipStr = ips[0].String()
	}
	logger.LogInfo("TSNet: Connected (IP: %s)", ipStr)

	localNATSPort := port + 10000
	// Start NATS locally without WebSocket (handled by unified web handler)
	ns := startNATSServer("0.0.0.0", localNATSPort, 0, verbose)
	defer ns.Shutdown()

	if useMock {
		go mock.StartMockMavlink(localNATSPort)
	} else if mavlinkAddr != "" {
		startMavlink(mavlinkAddr, localNATSPort)
	}

	startNatsPublisher(localNATSPort)

	natsLn, _ := ts.Listen("tcp", fmt.Sprintf(":%d", port))
	webLn, _ := ts.Listen("tcp", fmt.Sprintf(":%d", webPort))
	defer natsLn.Close()
	defer webLn.Close()

	go util.ProxyListener(natsLn, fmt.Sprintf("0.0.0.0:%d", localNATSPort))

	lc, _ := ts.LocalClient()
	
	// Use sub-filesystem for static assets to ensure correct path resolution
	var staticFS fs.FS = webFS
	if sub, err := fs.Sub(webFS, "src_v1/ui/dist"); err == nil {
		staticFS = sub
	}
	
	webHandler := web.CreateWebHandler(hostname, port, wsPort, webPort, localNATSPort, localNATSPort, ns, lc, ips, useMock, staticFS)

	go func() {
		logger.LogInfo("Web UI (Tailscale): Serving at http://%s:%d", hostname, webPort)
		http.Serve(webLn, webHandler)
	}()

	// Start local listener on 8080 for Cloudflare/direct access
	localWebAddr := "0.0.0.0:8080"
	logger.LogInfo("Web UI (Local): Serving at http://%s", localWebAddr)
	go http.ListenAndServe(":8080", webHandler)

	util.WaitForShutdown()
}

func startNATSServer(host string, port, wsPort int, verbose bool) *server.Server {
	opts := &server.Options{
		Host:  host,
		Port:  port,
		Debug: verbose,
		Trace: verbose,
	}
	if wsPort > 0 {
		opts.Websocket = server.WebsocketOpts{
			Host:  host,
			Port:  wsPort,
			NoTLS: true,
		}
	}
	ns, err := server.NewServer(opts)
	if err != nil {
		logger.LogFatal("Failed to create NATS server: %v", err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(10 * time.Second) {
		logger.LogFatal("NATS server failed to start")
	}
	return ns
}

func startMavlink(endpoint string, natsPort int) {
	logger.LogInfo("Starting Mavlink Service on %s...", endpoint)
	config := mavlink_app.MavlinkConfig{
		Endpoint: endpoint,
		Callback: func(evt *mavlink_app.MavlinkEvent) {
			var subject string
			var data []byte
			var err error
			switch evt.Type {
			case "HEARTBEAT":
				if msg, ok := evt.Data.(*common.MessageHeartbeat); ok {
					subject = "mavlink.heartbeat"
					data, err = json.Marshal(map[string]any{
						"type": "HEARTBEAT",
						"mav_type": msg.Type,
						"timestamp": time.Now().Unix(),
					})
				}
			}
			if err == nil && subject != "" {
				select {
				case mock.MavlinkPubChan <- mock.MavlinkNatsMsg{Subject: subject, Data: data}:
				default:
				}
			}
		},
	}
	svc, err := mavlink_app.NewMavlinkService(config)
	if err != nil {
		logger.LogFatal("Failed to create Mavlink service: %v", err)
	}
	go svc.Start()
}

func startNatsPublisher(port int) {
	go func() {
		time.Sleep(2 * time.Second)
		nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", port))
		if err != nil {
			return
		}
		defer nc.Close()
		for msg := range mock.MavlinkPubChan {
			nc.Publish(msg.Subject, msg.Data)
		}
	}()
}

// --- Deploy Logic ---

func RunDeploy(args []string) {
	fs := flag.NewFlagSet("deploy", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	hostname := fs.String("hostname", os.Getenv("DIALTONE_HOSTNAME"), "Tailscale hostname for the robot")
	proxy := fs.Bool("proxy", false, "Expose Web UI via Cloudflare proxy (drone-1.dialtone.earth)")
	service := fs.Bool("service", false, "Set up Dialtone as a systemd service on the robot")
	diagnostic := fs.Bool("diagnostic", false, "Run UI diagnostic after deployment")
	fs.Parse(args)

	if *host == "" || *pass == "" {
		logger.LogFatal("Error: --host and --pass are required")
	}

	// Use DIALTONE_HOSTNAME if set in env, otherwise fallback to SSH host if flag not provided
	if *hostname == "" {
		*hostname = os.Getenv("DIALTONE_HOSTNAME")
	}
	if *hostname == "" {
		*hostname = *host
	}

	validateRequiredVars([]string{"DIALTONE_HOSTNAME", "TS_AUTHKEY"})

	latestVer := getLatestVersionDir()
	cwd, _ := os.Getwd()
	
	// Read UI version from package.json
	uiVersion := "unknown"
	pkgJSONPath := filepath.Join(cwd, "src", "plugins", "robot", latestVer, "ui", "package.json")
	if data, err := os.ReadFile(pkgJSONPath); err == nil {
		var pkg struct { Version string `json:"version"` }
		if err := json.Unmarshal(data, &pkg); err == nil {
			uiVersion = pkg.Version
		}
	}

	logger.LogInfo("[DEPLOY] Starting deployment of Robot UI %s (%s)...", uiVersion, latestVer)
	logger.LogInfo("[DEPLOY] Target: %s (%s)", *hostname, *host)

	if err := robot_ops.Install(); err != nil {
		logger.LogFatal("Failed to install UI dependencies: %v", err)
	}

	logger.LogInfo("Connecting to %s to detect architecture...", *host)
	client, err := ssh.DialSSH(*host, *port, *user, *pass)
	if err != nil {
		logger.LogFatal("Failed to connect: %v", err)
	}
	defer client.Close()

	validateSudo(client, *pass)
	setupSSHKey(client, *user, *pass)

	remoteArch, _ := ssh.RunSSHCommand(client, "uname -m")
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

	uiDir := filepath.Join(cwd, "src", "plugins", "robot", latestVer, "ui")
	fmt.Printf(">> [Robot] Building UI: %s\n", latestVer)
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "build")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Failed to build UI: %v", err)
	}

	robotBinDir := filepath.Join("src", "plugins", "robot", "bin")
	fmt.Printf(">> [Robot] Building Dialtone Binary (%s) into %s\n", buildFlag, robotBinDir)
	build.RunBuild([]string{"--output-dir", robotBinDir, "--skip-web", "--skip-www", buildFlag})

	logger.LogInfo("Starting deployment to %s...", *host)
	
	// Remote binary name is always dialtone_robot
	remoteBinaryName := "dialtone_robot"
	binaryName := "dialtone-arm64"
	if remoteArch == "x86_64" || remoteArch == "amd64" {
		binaryName = "dialtone-amd64"
	}
	
	localBinaryPath := filepath.Join("src", "plugins", "robot", "bin", binaryName)
	remoteDir, _ := ssh.GetRemoteHome(client)
	remotePath := path.Join(remoteDir, remoteBinaryName)

	// Stop ALL legacy services and kill any stray processes before uploading
	logger.LogInfo("[DEPLOY] Stopping all remote services and processes...")
	// Stop/Disable legacy services if they exist
	for _, svc := range []string{"dialtone", "dialtone-robot"} {
		ssh.RunSSHCommand(client, fmt.Sprintf("printf \"%%s\\n\" \"%s\" | sudo -S -p '' systemctl stop %s.service", *pass, svc))
		ssh.RunSSHCommand(client, fmt.Sprintf("printf \"%%s\\n\" \"%s\" | sudo -S -p '' systemctl disable %s.service", *pass, svc))
	}
	// Stop new service if it's already there
	ssh.RunSSHCommand(client, fmt.Sprintf("printf \"%%s\\n\" \"%s\" | sudo -S -p '' systemctl stop dialtone_robot.service", *pass))

	// Kill any process using old or new name
	killCmd := fmt.Sprintf("printf \"%%s\\n\" \"%s\" | sudo -S -p '' pkill -9 dialtone; printf \"%%s\\n\" \"%s\" | sudo -S -p '' pkill -9 %s", *pass, *pass, remoteBinaryName)
	ssh.RunSSHCommand(client, killCmd)
	
	// Give it a moment to release the file
	time.Sleep(1 * time.Second)

	logger.LogInfo("[DEPLOY] Uploading binary to %s...", remotePath)
	if err := ssh.UploadFile(client, localBinaryPath, remotePath); err != nil {
		logger.LogFatal("Failed to upload binary: %v", err)
	}
	ssh.RunSSHCommand(client, "chmod +x "+remotePath)

	if *service {
		logger.LogInfo("[DEPLOY] Setting up systemd service...")
		// Use port 8080 for local web UI to avoid permission issues
		systemdServiceContent := fmt.Sprintf(`[Unit]
Description=Dialtone Robot Service
After=network.target tailscaled.service

[Service]
ExecStart=%s robot start --hostname %s --ephemeral --web-port 8080 --port 4222 --ws-port 4223
WorkingDirectory=%s
Environment="TS_AUTHKEY=%s"
Restart=always
User=%s

[Install]
WantedBy=multi-user.target`, remotePath, *hostname, remoteDir, os.Getenv("TS_AUTHKEY"), *user)
		
		// Create a temporary file on the remote robot with the service content
		tempServicePath := path.Join(remoteDir, fmt.Sprintf("dialtone_robot.service.tmp.%d", time.Now().Unix()))
		err = ssh.WriteRemoteFile(client, tempServicePath, systemdServiceContent)
		if err != nil {
			logger.LogFatal("Failed to write temporary systemd service file: %v", err)
		}
		
		// Move the temporary file to /etc/systemd/system/ using sudo mv
		finalServicePath := "/etc/systemd/system/dialtone_robot.service"
		sudoMoveCmd := fmt.Sprintf("printf \"%%s\\n\" \"%s\" | sudo -S -p '' mv %s %s", *pass, tempServicePath, finalServicePath)
		_, err = ssh.RunSSHCommand(client, sudoMoveCmd)
		if err != nil {
			logger.LogFatal("Failed to move systemd service file: %v", err)
		}
		
		// Set correct permissions on the service file
		sudoChmodCmd := fmt.Sprintf("printf \"%%s\\n\" \"%s\" | sudo -S -p '' chmod 0644 %s", *pass, finalServicePath)
		_, err = ssh.RunSSHCommand(client, sudoChmodCmd)
		if err != nil {
			logger.LogFatal("Failed to set permissions on systemd service file: %v", err)
		}

		// Enable and start the service, piping password to sudo
		ssh.RunSSHCommand(client, fmt.Sprintf("printf \"%%s\\n\" \"%s\" | sudo -S -p '' systemctl daemon-reload", *pass))
		ssh.RunSSHCommand(client, fmt.Sprintf("printf \"%%s\\n\" \"%s\" | sudo -S -p '' systemctl enable dialtone_robot.service", *pass))
		ssh.RunSSHCommand(client, fmt.Sprintf("printf \"%%s\\n\" \"%s\" | sudo -S -p '' systemctl start dialtone_robot.service", *pass))
		
		logger.LogInfo("[DEPLOY] Systemd service set up and started.")
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
		if err := rcli.RunDiagnostic(latestVer); err != nil {
			logger.LogError("Post-deployment diagnostic failed: %v", err)
			os.Exit(1)
		}
	}
}

func validateRequiredVars(vars []string) {
	for _, v := range vars {
		if os.Getenv(v) == "" {
			logger.LogFatal("Missing env var: %s", v)
		}
	}
}

func validateSudo(client *sshlib.Client, pass string) {
	sudoCmd := fmt.Sprintf("echo '%s' | sudo -S true < /dev/null", pass)
	_, err := ssh.RunSSHCommand(client, sudoCmd)
	if err != nil {
		logger.LogFatal("Sudo validation failed")
	}
}

func setupSSHKey(client *sshlib.Client, remoteUser, pass string) {
	home, _ := os.UserHomeDir()
	pubKeyPath := filepath.Join(home, ".ssh", "id_ed25519.pub")
	pubKey, err := os.ReadFile(pubKeyPath)
	if err != nil {
		logger.LogInfo("SSH public key not found at %s. Key-based authentication may not be available for passwordless sudo setup.", pubKeyPath)
		return
	}
	pubKeyStr := strings.TrimSpace(string(pubKey))
	
	// Ensure ~/.ssh directory exists with correct permissions
	setupKeyCmd := fmt.Sprintf("mkdir -p ~/.ssh && chmod 700 ~/.ssh && echo '%s' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys", pubKeyStr)
	_, err = ssh.RunSSHCommand(client, setupKeyCmd)
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
	err = ssh.WriteRemoteFile(client, tempSudoersPath, sudoersContent)
	if err != nil {
		logger.LogWarn("Failed to write temporary sudoers file: %v", err)
		return
	}

	// Move temporary file to /etc/sudoers.d/ using sudo
	sudoMoveCmd := fmt.Sprintf("echo '%s' | sudo -S -p '' mv %s %s", pass, tempSudoersPath, sudoersFile)
	_, err = ssh.RunSSHCommand(client, sudoMoveCmd)
	if err != nil {
		logger.LogWarn("Failed to move sudoers file into place. Passwordless sudo not configured: %v", err)
		return
	}
	
	// Set correct permissions on the sudoers file
	sudoChmodCmd := fmt.Sprintf("echo '%s' | sudo -S -p '' chmod 0440 %s", pass, sudoersFile)
	_, err = ssh.RunSSHCommand(client, sudoChmodCmd)
	if err != nil {
		logger.LogWarn("Failed to set permissions on sudoers file: %v", err)
		return
	}
	
	logger.LogInfo("Configured passwordless sudo for user '%s' via SSH key.", remoteUser)
}

// --- Versioned Source Commands ---

func RunFmt(versionDir string) error {
	fmt.Printf(">> [Robot] Fmt: %s\n", versionDir)
	cwd, _ := os.Getwd()
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "fmt", "./src/plugins/robot/"+versionDir+"/...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunFormat(versionDir string) error {
	fmt.Printf(">> [Robot] Format: %s\n", versionDir)
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "robot", versionDir, "ui")
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "format")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunVet(versionDir string) error {
	fmt.Printf(">> [Robot] Vet: %s\n", versionDir)
	cwd, _ := os.Getwd()
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "vet", "./src/plugins/robot/"+versionDir+"/...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunGoBuild(versionDir string) error {
	fmt.Printf(">> [Robot] Go Build: %s\n", versionDir)
	cwd, _ := os.Getwd()
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "build", "./src/plugins/robot/"+versionDir+"/...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunLint(versionDir string) error {
	fmt.Printf(">> [Robot] Lint: %s\n", versionDir)
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "robot", versionDir, "ui")
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "lint")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunServe(versionDir string) error {
	fmt.Printf(">> [Robot] Serve: %s\n", versionDir)
	cwd, _ := os.Getwd()
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "run", filepath.Join("src", "plugins", "robot", versionDir, "cmd", "main.go"))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunUIRun(versionDir string, extraArgs []string) error {
	fmt.Printf(">> [Robot] UI Run: %s\n", versionDir)
	port := 3000
	if len(extraArgs) >= 2 && extraArgs[0] == "--port" {
		if p, err := strconv.Atoi(extraArgs[1]); err == nil {
			port = p
		}
	}
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "robot", versionDir, "ui")
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(port))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunDev(versionDir string) error {
	return RunUIRun(versionDir, nil)
}

func RunVersionedTest(versionDir string) error {
	cwd, _ := os.Getwd()
	testPkg := "./" + filepath.Join("src", "plugins", "robot", versionDir, "test")
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "run", testPkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

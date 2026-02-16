package robot

import (
	"context"
	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/util"
	"dialtone/cli/src/core/web"
	// "dialtone/cli/src/core/config" // Not directly used in robot.go anymore
	"dialtone/cli/src/core/mock"
	"dialtone/cli/src/core/build"
	"dialtone/cli/src/core/ssh"
	mavlink_app "dialtone/cli/src/plugins/mavlink/app"
	ai_app "dialtone/cli/src/plugins/ai/app"
	robot_ops "dialtone/cli/src/plugins/robot/src_v1/cmd/ops"
	"encoding/json"
	"flag"
	"fmt"
	//"net" // Removed: Not directly used in robot.go
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	// "runtime" // Not directly used in robot.go anymore
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
	opencode := fs.Bool("opencode", false, "Start opencode AI assistant server")
	useMock := fs.Bool("mock", false, "Use mock telemetry and camera data")
	fs.Parse(args)

	if *stateDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			logger.LogFatal("Failed to get home directory: %v", err)
		}
		*stateDir = filepath.Join(homeDir, ".config", "dialtone")
	}

	if *useMock && *opencode {
		logger.LogInfo("Mock mode enabled: Disabling opencode")
		*opencode = false
	}

	if *localOnly {
		runLocalOnly(*natsPort, *wsPort, *webPort, *verbose, *mavlinkAddr, *opencode, *useMock, *hostname)
		return
	}

	runWithTailscale(*hostname, *natsPort, *wsPort, *webPort, *stateDir, *ephemeral, *verbose, *mavlinkAddr, *opencode, *useMock)
}

func runLocalOnly(port, wsPort, webPort int, verbose bool, mavlinkAddr string, opencode bool, useMock bool, hostname string) {
	ns := startNATSServer("0.0.0.0", port, wsPort, verbose)
	defer ns.Shutdown()

	logger.LogInfo("NATS server started on port %d (local only)", port)

	if useMock {
		go mock.StartMockMavlink(port)
	} else if mavlinkAddr != "" {
		startMavlink(mavlinkAddr, port)
	}

	if opencode {
		go ai_app.RunOpencodeServer(3000)
	}

	startNatsPublisher(port)

	webHandler := web.CreateWebHandler(hostname, port, wsPort, webPort, port, wsPort, ns, nil, nil, useMock, webFS)
	if sub, err := fs.Sub(webFS, "src_v1/ui/dist"); err == nil {
		webHandler = web.CreateWebHandler(hostname, port, wsPort, webPort, port, wsPort, ns, nil, nil, useMock, sub)
	}
	localWebAddr := fmt.Sprintf("0.0.0.0:%d", webPort)
	if webPort == 80 {
		localWebAddr = "0.0.0.0:8080"
	}

	logger.LogInfo("Web UI (Local Only): Serving at http://%s", localWebAddr)
	http.ListenAndServe(localWebAddr, webHandler)
}

func runWithTailscale(hostname string, port, wsPort, webPort int, stateDir string, ephemeral, verbose bool, mavlinkAddr string, opencode bool, useMock bool) {
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
	localWSPort := wsPort + 10000
	ns := startNATSServer("127.0.0.1", localNATSPort, localWSPort, verbose)
	defer ns.Shutdown()

	if useMock {
		go mock.StartMockMavlink(localNATSPort)
	} else if mavlinkAddr != "" {
		startMavlink(mavlinkAddr, localNATSPort)
	}

	if opencode {
		go ai_app.RunOpencodeServer(3000)
	}

	startNatsPublisher(localNATSPort)

	natsLn, _ := ts.Listen("tcp", fmt.Sprintf(":%d", port))
	wsLn, _ := ts.Listen("tcp", fmt.Sprintf(":%d", wsPort))
	webLn, _ := ts.Listen("tcp", fmt.Sprintf(":%d", webPort))
	defer natsLn.Close()
	defer wsLn.Close()
	defer webLn.Close()

	go util.ProxyListener(natsLn, fmt.Sprintf("127.0.0.1:%d", localNATSPort))
	go util.ProxyListener(wsLn, fmt.Sprintf("127.0.0.1:%d", localWSPort))

	lc, _ := ts.LocalClient()
	webHandler := web.CreateWebHandler(hostname, port, wsPort, webPort, localNATSPort, localWSPort, ns, lc, ips, useMock, webFS)

	go func() {
		logger.LogInfo("Web UI (Tailscale): Serving at http://%s:%d", hostname, webPort)
		http.Serve(webLn, webHandler)
	}()

	util.WaitForShutdown()
}

func startNATSServer(host string, port, wsPort int, verbose bool) *server.Server {
	opts := &server.Options{
		Host:  host,
		Port:  port,
		Debug: verbose,
		Trace: verbose,
		Websocket: server.WebsocketOpts{
			Host:  host,
			Port:  wsPort,
			NoTLS: true,
		},
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

	cwd, _ := os.Getwd()
	logger.LogInfo("Building Robot UI and Binary...")
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
	setupSSHKey(client)

	remoteArch, _ := ssh.RunSSHCommand(client, "uname -m")
	remoteArch = strings.TrimSpace(remoteArch)

	var buildFlag string
	var binaryName string
	switch remoteArch {
	case "aarch64", "arm64":
		buildFlag = "--linux-arm64"
		binaryName = "dialtone-arm64"
	case "x86_64", "amd64":
		buildFlag = "--linux-amd64"
		binaryName = "dialtone-amd64"
	default:
		logger.LogFatal("Unsupported arch: %s", remoteArch)
	}

	uiDir := filepath.Join(cwd, "src", "plugins", "robot", "src_v1", "ui")
	fmt.Printf(">> [Robot] Building UI: src_v1\n")
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
	localBinaryPath := filepath.Join("src", "plugins", "robot", "bin", binaryName)
	
	remoteDir, _ := ssh.GetRemoteHome(client)
	remotePath := path.Join(remoteDir, "dialtone")
	
	logger.LogInfo("Uploading binary...")
	ssh.UploadFile(client, localBinaryPath, remotePath)
	ssh.RunSSHCommand(client, "chmod +x "+remotePath)

	if *service {
		logger.LogInfo("Setting up systemd service...")
		systemdServiceContent := fmt.Sprintf(`[Unit]
Description=Dialtone Robot Service
After=network.target tailscaled.service

[Service]
ExecStart=%s robot start --hostname %s --ephemeral --web-port 80 --nats-port 4222 --ws-port 4223
WorkingDirectory=%s
Environment="TS_AUTHKEY=%s"
Restart=always
User=%s

[Install]
WantedBy=multi-user.target`, remotePath, *hostname, remoteDir, os.Getenv("TS_AUTHKEY"), *user)
		
		ssh.RunSSHCommand(client, fmt.Sprintf("echo \"%s\" | sudo -S tee /etc/systemd/system/dialtone-robot.service", systemdServiceContent))
		ssh.RunSSHCommand(client, "sudo systemctl enable dialtone-robot")
		ssh.RunSSHCommand(client, "sudo systemctl start dialtone-robot")
		logger.LogInfo("Systemd service set up and started.")
	}

	if *proxy {
		logger.LogInfo("Setting up Cloudflare tunnel proxy via cloudflare plugin...")
		cwd, _ := os.Getwd()
		cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "cloudflare", "setup-service", "--name", *hostname)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logger.LogInfo("[WARNING] Cloudflare proxy setup failed: %v", err)
		} else {
			logger.LogInfo("Cloudflare tunnel proxy for %s.dialtone.earth setup successfully.", *hostname)
		}
	}
	
	logger.LogInfo("Deployment complete.")
}

func RunSyncCode(args []string) {
	// Simple version of SyncCode
	fs := flag.NewFlagSet("sync-code", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	fs.Parse(args)

	if *host == "" || *pass == "" {
		logger.LogFatal("Error: -host and -pass required")
	}

	client, err := ssh.DialSSH(*host, "22", "", *pass)
	if err != nil {
		logger.LogFatal("SSH failed: %v", err)
	}
	defer client.Close()

	home, _ := ssh.GetRemoteHome(client)
	remoteDir := path.Join(home, "dialtone_src")
	ssh.RunSSHCommand(client, "mkdir -p "+remoteDir)

	logger.LogInfo("Syncing src/ to %s...", remoteDir)
	ssh.UploadDirFiltered(client, "src", path.Join(remoteDir, "src"), []string{".git", "node_modules", "dist"})
	logger.LogInfo("Sync complete.")
}

func validateRequiredVars(vars []string) {
	for _, v := range vars {
		if os.Getenv(v) == "" {
			logger.LogFatal("Missing env var: %s", v)
		}
	}
}

func validateSudo(client *sshlib.Client, pass string) {
	sudoCmd := fmt.Sprintf("echo '%s' | sudo -S true", pass)
	_, err := ssh.RunSSHCommand(client, sudoCmd)
	if err != nil {
		logger.LogFatal("Sudo validation failed")
	}
}

func setupSSHKey(client *sshlib.Client) {
	home, _ := os.UserHomeDir()
	pubKeyPath := filepath.Join(home, ".ssh", "id_ed25519.pub")
	pubKey, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return
	}
	pubKeyStr := strings.TrimSpace(string(pubKey))
	setupKeyCmd := fmt.Sprintf("mkdir -p ~/.ssh && echo '%s' >> ~/.ssh/authorized_keys", pubKeyStr)
	ssh.RunSSHCommand(client, setupKeyCmd)
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

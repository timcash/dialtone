package cli

import (
	"bytes"
	"crypto/rand"
	"dialtone/dev/plugins/test/src_v1/go"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"dialtone/dev/config"
	"dialtone/dev/plugins/logs/src_v1/go"
	"dialtone/dev/util"
)

func findCloudflared() string {
	depsDir := config.GetDialtoneEnv()

	cfPath := filepath.Join(depsDir, "cloudflare", "cloudflared")
	if _, err := os.Stat(cfPath); err == nil {
		return cfPath
	}

	// Fallback to system PATH
	if p, err := exec.LookPath("cloudflared"); err == nil {
		return p
	}

	return "cloudflared"
}

func runBun(repoRoot, uiDir string, args ...string) *exec.Cmd {
	bunArgs := append([]string{"bun", "exec", "--cwd", uiDir}, args...)
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), bunArgs...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// RunCloudflare handles 'cloudflare <subcommand>'
func RunCloudflare(args []string) {
	if len(args) == 0 {
		printCloudflareUsage()
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
	case "login":
		runLogin(restArgs)
	case "tunnel":
		runTunnel(restArgs)
	case "serve":
		if len(restArgs) > 0 && strings.HasPrefix(restArgs[0], "src_v") {
			RunServe(restArgs[0])
		} else {
			runServe(restArgs)
		}
	case "robot":
		runRobot(restArgs)
	case "proxy":
		runProxy(restArgs)
	case "provision":
		runProvision(restArgs)
	case "install":
		RunInstall(getDir())
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
	case "dev":
		RunDev(getDir())
	case "ui-run":
		RunUIRun(getDir(), args[2:])
	case "test":
		RunTest(getDir())
	case "build":
		RunBuild(getDir())
	case "setup-service":
		RunSetupService(restArgs)
	case "help", "-h", "--help":
		printCloudflareUsage()
	default:
		fmt.Printf("Unknown cloudflare command: %s\n", subcommand)
		printCloudflareUsage()
		os.Exit(1)
	}
}

func printCloudflareUsage() {
	fmt.Println("Usage: dialtone cloudflare <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  login       Authenticate with Cloudflare")
	fmt.Println("  tunnel      Manage Cloudflare tunnels (create, list, etc.)")
	fmt.Println("  serve       Forward a local HTTP server to the web (or run plugin Go server with src_vN)")
	fmt.Println("  robot       Expose a remote robot via this computer (proxy -> tunnel)")
	fmt.Println("  proxy       Start a local TCP proxy to a remote target")
	fmt.Println("  provision   Create a new tunnel and save token to .env (requires API Token)")
	fmt.Println("  setup-service Set up the Cloudflare proxy as a local systemd service")
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
	fmt.Println("  test        Run automated tests and write TEST.md artifacts")
	fmt.Println("  help        Show this help message")
}

func RunSetupService(args []string) {
	fs := flag.NewFlagSet("setup-service", flag.ExitOnError)
	name := fs.String("name", os.Getenv("DIALTONE_DOMAIN"), "Cloudflare subdomain/robot name")
	userMode := fs.Bool("user", false, "Install as user-level service (no sudo)")
	urlFlag := fs.String("url", "", "Target URL for the proxy (optional)")
	fs.Parse(args)

	robotName := *name
	if robotName == "" && len(fs.Args()) > 0 {
		robotName = fs.Args()[0]
	}
	if robotName == "" {
		robotName = os.Getenv("DIALTONE_HOSTNAME")
	}

	if robotName == "" {
		logs.Fatal("Error: robot name is required for setup-service (pass as arg or set DIALTONE_DOMAIN/DIALTONE_HOSTNAME in .env)")
	}

	cwd, _ := os.Getwd()
	dialtoneSh := filepath.Join(cwd, "dialtone.sh")
	user := os.Getenv("USER")
	if user == "" {
		user = "user"
	}

	// Determine dependency dir
	depsDir := config.GetDialtoneEnv()

	// Configure based on mode
	var servicePath, wantedBy, userLine, installCmd, reloadCmd, enableCmd, restartCmd string
	var useSudo bool

	if *userMode {
		// Check for lingering
		out, err := exec.Command("loginctl", "show-user", user, "--property=Linger").Output()
		if err == nil {
			if strings.TrimSpace(string(out)) == "Linger=no" {
				logs.Fatal("Error: User lingering is not enabled. Cloudflare proxy service requires lingering to run in the background.\nPlease run: loginctl enable-linger %s", user)
			}
		} else {
			// If loginctl fails, warn but proceed (might be non-systemd environment or permission issue, though unlikely for user query)
			logs.Info("Warning: Could not verify user lingering status. Ensure it is enabled: loginctl enable-linger %s", user)
		}

		homeDir, _ := os.UserHomeDir()
		userSystemdDir := filepath.Join(homeDir, ".config", "systemd", "user")
		if err := os.MkdirAll(userSystemdDir, 0755); err != nil {
			logs.Fatal("Failed to create user systemd dir: %v", err)
		}
		servicePath = filepath.Join(userSystemdDir, fmt.Sprintf("dialtone-proxy-%s.service", robotName))
		wantedBy = "default.target"
		userLine = "" // User implied in user mode
		useSudo = false
		installCmd = fmt.Sprintf("cp %%s %s", servicePath)
		reloadCmd = "systemctl --user daemon-reload"
		enableCmd = fmt.Sprintf("systemctl --user enable dialtone-proxy-%s.service", robotName)
		restartCmd = fmt.Sprintf("systemctl --user restart dialtone-proxy-%s.service", robotName)
	} else {
		servicePath = fmt.Sprintf("/etc/systemd/system/dialtone-proxy-%s.service", robotName)
		wantedBy = "multi-user.target"
		userLine = fmt.Sprintf("User=%s", user)
		useSudo = true
		installCmd = fmt.Sprintf("cp %%s %s", servicePath)
		reloadCmd = "systemctl daemon-reload"
		enableCmd = fmt.Sprintf("systemctl enable dialtone-proxy-%s.service", robotName)
		restartCmd = fmt.Sprintf("systemctl restart dialtone-proxy-%s.service", robotName)
	}

	extraArgs := ""
	if *urlFlag != "" {
		extraArgs = fmt.Sprintf(" --url %s", *urlFlag)
	}

	serviceTemplate := `[Unit]
Description=Dialtone Cloudflare Proxy for %s
After=network.target

[Service]
ExecStart=%s cloudflare robot --name %s%s
WorkingDirectory=%s
%s
Environment=DIALTONE_ENV=%s
Environment=DIALTONE_ENV_FILE=%s
Restart=always
RestartSec=10

[Install]
WantedBy=%s
`
	serviceContent := fmt.Sprintf(serviceTemplate, robotName, dialtoneSh, robotName, extraArgs, cwd, userLine, depsDir, filepath.Join(cwd, "env/.env"), wantedBy)
	
	// Clean up empty lines if User line is empty
	if userLine == "" {
		serviceContent = strings.Replace(serviceContent, "\n\nEnvironment=", "\nEnvironment=", 1)
	}

	fmt.Printf("Creating systemd service at %s...\n", servicePath)

	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("dialtone-proxy-%s.service", robotName))
	err := os.WriteFile(tmpFile, []byte(serviceContent), 0644)
	if err != nil {
		logs.Fatal("Failed to write temporary service file: %v", err)
	}

	runShell := config.RunSimpleShell
	if useSudo {
		runShell = config.RunSudoShell
	}

	runShell(fmt.Sprintf(installCmd, tmpFile))
	runShell(reloadCmd)
	runShell(enableCmd)
	runShell(restartCmd)

	logs.Info("SUCCESS: Cloudflare proxy service for '%s' installed and started.", robotName)
	if *userMode {
		logs.Info("Check status with: systemctl --user status dialtone-proxy-%s.service", robotName)
		logs.Info("Note: Ensure linger is enabled to keep service running after logout: loginctl enable-linger %s", user)
	} else {
		logs.Info("Check status with: systemctl status dialtone-proxy-%s.service", robotName)
	}
}

func getLatestVersionDir() string {
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "cloudflare")
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

func RunFmt(versionDir string) error {
	fmt.Printf(">> [CLOUDFLARE] Fmt: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "fmt", "./src/plugins/cloudflare/"+versionDir+"/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunFormat(versionDir string) error {
	fmt.Printf(">> [CLOUDFLARE] Format: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	uiDir := filepath.Join(cwd, "src", "plugins", "cloudflare", versionDir, "ui")
	cmd := runBun(cwd, uiDir, "run", "format")
	return cmd.Run()
}

func RunVet(versionDir string) error {
	fmt.Printf(">> [CLOUDFLARE] Vet: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "vet", "./src/plugins/cloudflare/"+versionDir+"/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunGoBuild(versionDir string) error {
	fmt.Printf(">> [CLOUDFLARE] Go Build: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "build", "./src/plugins/cloudflare/"+versionDir+"/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunLint(versionDir string) error {
	fmt.Printf(">> [CLOUDFLARE] Lint: %s\n", versionDir)

	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "cloudflare", versionDir, "ui")

	fmt.Println("   [LINT] Running tsc...")
	cmd := runBun(cwd, uiDir, "run", "lint")
	return cmd.Run()
}

func RunServe(versionDir string) error {
	fmt.Printf(">> [CLOUDFLARE] Serve: %s\n", versionDir)

	cwd, _ := os.Getwd()
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "run", filepath.ToSlash(filepath.Join("src", "plugins", "cloudflare", versionDir, "cmd", "main.go")))
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunUIRun(versionDir string, extraArgs []string) error {
	fmt.Printf(">> [CLOUDFLARE] UI Run: %s\n", versionDir)
	port := 3000
	if len(extraArgs) >= 2 && extraArgs[0] == "--port" {
		if p, err := strconv.Atoi(extraArgs[1]); err == nil {
			port = p
		}
	}

	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "cloudflare", versionDir, "ui")
	cmd := runBun(cwd, uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(port))
	return cmd.Run()
}

func RunDev(versionDir string) error {
	fmt.Printf(">> [CLOUDFLARE] Dev: %s\n", versionDir)
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "cloudflare", versionDir, "ui")
	versionDirPath := filepath.Join(cwd, "src", "plugins", "cloudflare", versionDir)
	devPort := 3000
	devURL := fmt.Sprintf("http://127.0.0.1:%d", devPort)

	devSession, err := test_v2.NewDevSession(test_v2.DevSessionOptions{
		VersionDirPath: versionDirPath,
		Port:           devPort,
		URL:            devURL,
		ConsoleWriter:  os.Stdout,
	})
	if err != nil {
		return err
	}
	defer devSession.Close()

	fmt.Println("   [DEV] Running vite dev...")
	cmd := runBun(cwd, uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(devPort))
	cmd.Stdout = devSession.Writer()
	cmd.Stderr = devSession.Writer()
	if err := cmd.Start(); err != nil {
		return err
	}
	devSession.StartBrowserAttach()

	waitErr := cmd.Wait()
	return waitErr
}

func RunBuild(versionDir string) error {
	fmt.Printf(">> [CLOUDFLARE] Build: %s\n", versionDir)
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "cloudflare", versionDir, "ui")

	if err := RunInstall(versionDir); err != nil {
		return err
	}

	fmt.Println("   [BUILD] Running UI build...")
	cmd := runBun(cwd, uiDir, "run", "build")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %v", err)
	}

	fmt.Println(">> [CLOUDFLARE] Build successful")
	return nil
}

func RunTest(versionDir string) error {
	dir := versionDir
	cwd, _ := os.Getwd()
	testPkg := "./" + filepath.ToSlash(filepath.Join("src", "plugins", "cloudflare", dir, "test"))
	if _, err := os.Stat(filepath.Join(cwd, "src", "plugins", "cloudflare", dir, "test", "main.go")); os.IsNotExist(err) {
		return fmt.Errorf("test runner not found: %s/main.go", testPkg)
	}
	fmt.Printf(">> [CLOUDFLARE] Running Test Suite for %s...\n", dir)
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "run", testPkg)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runLogin(args []string) {
	cf := findCloudflared()
	logs.Info("Logging into Cloudflare...")

	cmd := exec.Command(cf, "tunnel", "login")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		logs.Fatal("Cloudflare login failed: %v", err)
	}
	logs.Info("Cloudflare login complete.")
}

func runTunnel(args []string) {
	cf := findCloudflared()
	if len(args) == 0 {
		fmt.Println("Usage: dialtone cloudflare tunnel <subcommand>")
		fmt.Println("\nSubcommands:")
		fmt.Println("  create <name>   Create a new tunnel")
		fmt.Println("  list            List existing tunnels")
		fmt.Println("  run <name>      Run a tunnel")
		fmt.Println("  route <name>    Route a hostname to a tunnel")
		fmt.Println("  cleanup         Terminate all local tunnel processes")
		return
	}

	sub := args[0]
	subArgs := args[1:]

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "tunnel")

	switch sub {
	case "create":
		cmdArgs = append(cmdArgs, "create")
		cmdArgs = append(cmdArgs, subArgs...)
	case "list":
		cmdArgs = append(cmdArgs, "list")
		cmdArgs = append(cmdArgs, subArgs...)
	case "run":
		if len(subArgs) < 1 {
			fmt.Println("Usage: dialtone cloudflare tunnel run <name> [options]")
			return
		}
		tunnelName := subArgs[0]
		runFs := flag.NewFlagSet("run", flag.ExitOnError)
		urlFlag := runFs.String("url", "", "Service URL to forward to")
		tokenFlag := runFs.String("token", os.Getenv("CF_TUNNEL_TOKEN"), "Cloudflare Tunnel Service Token")
		runFs.Parse(subArgs[1:])

		if *urlFlag == "" {
			logs.Fatal("Error: --url is required for 'cloudflare tunnel run'")
		}
		if *tokenFlag != "" {
			cmdArgs = append(cmdArgs, "run", "--token", *tokenFlag)
		} else {
			cmdArgs = append(cmdArgs, "run", tunnelName)
		}
		cmdArgs = append(cmdArgs, "--url", *urlFlag)
	case "route":
		if len(subArgs) == 0 {
			fmt.Println("Usage: dialtone cloudflare tunnel route <tunnel-name> [hostname]")
			return
		}
		tunnelName := subArgs[0]
		hostname := ""
		if len(subArgs) > 1 {
			hostname = subArgs[1]
		} else {
			config.LoadConfig()
			dh := os.Getenv("DIALTONE_HOSTNAME")
			if dh != "" {
				hostname = fmt.Sprintf("%s.dialtone.earth", dh)
				logs.Info("Using DIALTONE_HOSTNAME for subdomain: %s", hostname)
			}
		}
		if hostname == "" {
			logs.Fatal("No hostname provided and DIALTONE_HOSTNAME not set in env/.env")
		}
		cmdArgs = append(cmdArgs, "route", "dns", tunnelName, hostname)
	case "cleanup":
		runCleanup()
		return
	default:
		fmt.Printf("Unknown tunnel subcommand: %s\n", sub)
		return
	}

	cmd := exec.Command(cf, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logs.Fatal("Cloudflare tunnel %s failed: %v", sub, err)
	}
}

func runServe(args []string) {
	cf := findCloudflared()
	if len(args) < 1 {
		fmt.Println("Usage: dialtone cloudflare serve <port-or-url>")
		return
	}

	target := args[0]
	logs.Info("Starting Cloudflare tunnel to serve %s...", target)

	// cloudflared tunnel --url http://localhost:PORT
	// Or just cloudflared tunnel --url target

	cmdArgs := []string{"tunnel", "--url", target}
	cmd := exec.Command(cf, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logs.Fatal("Cloudflare serve failed: %v", err)
	}
}

func runCleanup() {
	logs.Info("Cleaning up local Cloudflare tunnels...")
	// We'll use pkill if available, otherwise fallback to a manual process check
	// For simplicity and speed in a CLI context:
	cmd := exec.Command("pkill", "-f", "cloudflared")
	// Ignore error as it fails if no processes are found
	_ = cmd.Run()
	logs.Info("Tunnel cleanup complete.")
}

func runRobot(args []string) {
	fs := flag.NewFlagSet("robot", flag.ExitOnError)
	hostname := fs.String("name", "", "Robot hostname (default: DIALTONE_HOSTNAME)")
	token := fs.String("token", "", "Cloudflare Tunnel Token (overrides .env)")
	urlFlag := fs.String("url", "", "Target URL (overrides default http://name:80)")
	fs.Parse(args)

	name := *hostname
	if name == "" && len(fs.Args()) > 0 {
		name = fs.Args()[0]
	}
	if name == "" {
		name = os.Getenv("DIALTONE_DOMAIN")
	}
	if name == "" {
		name = os.Getenv("DIALTONE_HOSTNAME")
	}

	if name == "" {
		logs.Fatal("Error: robot name is required (pass as arg or set DIALTONE_HOSTNAME in .env)")
	}

	// 1. Determine the Tunnel Token
	runToken := *token
	if runToken == "" {
		// Look for namespaced token first: CF_TUNNEL_TOKEN_%s
		envKey := fmt.Sprintf("CF_TUNNEL_TOKEN_%s", strings.ToUpper(strings.ReplaceAll(name, "-", "_")))
		runToken = os.Getenv(envKey)
		if runToken == "" {
			// Fallback to global token
			runToken = os.Getenv("CF_TUNNEL_TOKEN")
		}
	}

	// 2. Determine the target URL for the robot
	targetURL := *urlFlag
	if targetURL == "" {
		targetURL = fmt.Sprintf("http://%s:80", name)
	}

	// 3. Run Cloudflare Tunnel
	cf := findCloudflared()
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "tunnel")

	if runToken != "" {
		logs.Info("Using Tunnel Token for %s...", name)
		cmdArgs = append(cmdArgs, "run", "--token", runToken)
	} else {
		logs.Info("No token found, attempting to run by name (requires 'cloudflare login')...")
		cmdArgs = append(cmdArgs, "run", name)
	}

	cmdArgs = append(cmdArgs, "--url", targetURL)

	logs.Info("Starting Cloudflare tunnel for %s.dialtone.earth, directly targeting %s...", name, targetURL)
	cmd := exec.Command(cf, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // Run in new process group

	if err := cmd.Start(); err != nil {
		logs.Fatal("Cloudflare tunnel failed to start: %v", err)
	}
	logs.Info("Cloudflare tunnel process started with PID: %d", cmd.Process.Pid)

	// Keep the dialtone process alive to track the cloudflared process
	util.WaitForShutdown()
}

func runProxy(args []string) {
	fs := flag.NewFlagSet("proxy", flag.ExitOnError)
	localPort := fs.Int("port", 8081, "Local port to listen on")
	target := fs.String("target", "", "Target address (e.g. drone-1:8080)")
	fs.Parse(args)

	targetAddr := *target
	if targetAddr == "" && len(fs.Args()) > 0 {
		targetAddr = fs.Args()[0]
	}

	if targetAddr == "" {
		logs.Fatal("Usage: dialtone cloudflare proxy <target> [--port <local-port>]")
	}

	addr := fmt.Sprintf("127.0.0.1:%d", *localPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		logs.Fatal("Failed to listen on %s: %v", addr, err)
	}

	logs.Info("TCP Proxy started: %s -> %s", addr, targetAddr)
	util.ProxyListener(ln, targetAddr)
}

func runProvision(args []string) {
	fs := flag.NewFlagSet("provision", flag.ExitOnError)
	name := fs.String("name", "", "Tunnel name (subdomain prefix)")
	domain := fs.String("domain", "dialtone.earth", "Cloudflare managed domain")
	apiToken := fs.String("api-token", os.Getenv("CLOUDFLARE_API_TOKEN"), "Cloudflare API Token")
	accountID := fs.String("account-id", os.Getenv("CLOUDFLARE_ACCOUNT_ID"), "Cloudflare Account ID")
	fs.Parse(args)

	tunnelName := *name
	if tunnelName == "" && len(fs.Args()) > 0 {
		tunnelName = fs.Args()[0]
	}

	if tunnelName == "" {
		tunnelName = os.Getenv("DIALTONE_HOSTNAME")
	}

	if tunnelName == "" {
		logs.Fatal("Usage: dialtone cloudflare provision <name> [--domain <domain>]")
	}

	if *apiToken == "" || *accountID == "" {
		logs.Fatal("Error: CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID must be set in .env")
	}

	fullHostname := fmt.Sprintf("%s.%s", tunnelName, *domain)
	logs.Info("Provisioning Cloudflare Tunnel and DNS for: %s", fullHostname)

	client := &http.Client{}

	// 1. Find Zone ID for the domain
	type ZoneResponse struct {
		Result []struct {
			ID string `json:"id"`
		} `json:"result"`
		Success bool `json:"success"`
	}

	zoneReq, _ := http.NewRequest("GET", fmt.Sprintf("https://api.cloudflare.com/client/v4/zones?name=%s", *domain), nil)
	zoneReq.Header.Set("Authorization", "Bearer "+*apiToken)
	zoneResp, err := client.Do(zoneReq)
	if err != nil {
		logs.Fatal("Failed to fetch zone: %v", err)
	}
	defer zoneResp.Body.Close()

	var zr ZoneResponse
	json.NewDecoder(zoneResp.Body).Decode(&zr)
	if !zr.Success || len(zr.Result) == 0 {
		logs.Fatal("Could not find Cloudflare zone for %s. Check your API Token permissions.", *domain)
	}
	zoneID := zr.Result[0].ID

	// 2. Generate Secret and Create Tunnel
	secretBytes := make([]byte, 32)
	rand.Read(secretBytes)
	tunnelSecret := base64.StdEncoding.EncodeToString(secretBytes)

	type CreateResponse struct {
		Result struct {
			ID string `json:"id"`
		} `json:"result"`
		Success bool `json:"success"`
	}

	payload := map[string]string{"name": tunnelName, "tunnel_secret": tunnelSecret}
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/tunnels", *accountID)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+*apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		logs.Fatal("Tunnel creation failed. (Already exists? Use a different name)")
	}
	var cr CreateResponse
	json.NewDecoder(resp.Body).Decode(&cr)
	tunnelID := cr.Result.ID
	logs.Info("Tunnel created: %s", tunnelID)

	// 3. Create DNS CNAME Record
	dnsPayload := map[string]any{
		"type":    "CNAME",
		"name":    tunnelName,
		"content": fmt.Sprintf("%s.cfargotunnel.com", tunnelID),
		"proxied": true,
		"ttl":     1, // Auto
	}
	dnsBody, _ := json.Marshal(dnsPayload)
	dnsURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", zoneID)
	dnsReq, _ := http.NewRequest("POST", dnsURL, bytes.NewBuffer(dnsBody))
	dnsReq.Header.Set("Authorization", "Bearer "+*apiToken)
	dnsReq.Header.Set("Content-Type", "application/json")

	dnsResp, err := client.Do(dnsReq)
	if err != nil || dnsResp.StatusCode != http.StatusOK {
		logs.Info("Warning: Failed to create DNS record (it might already exist).")
	} else {
		logs.Info("DNS record created: %s -> %s.cfargotunnel.com", fullHostname, tunnelID)
	}

	// 4. Generate the Run Token and Save to .env
	tokenData := map[string]string{"a": *accountID, "t": tunnelID, "s": tunnelSecret}
	tokenJSON, _ := json.Marshal(tokenData)
	runToken := base64.StdEncoding.EncodeToString(tokenJSON)

	envPath := os.Getenv("DIALTONE_ENV_FILE")
	if envPath == "" {
		envPath = "env/.env"
	}
	f, _ := os.OpenFile(envPath, os.O_APPEND|os.O_WRONLY, 0644)
	defer f.Close()
	f.WriteString(fmt.Sprintf("\n# CF Token for %s\nCF_TUNNEL_TOKEN_%s=%s\n", tunnelName, strings.ToUpper(strings.ReplaceAll(tunnelName, "-", "_")), runToken))

	logs.Info("SUCCESS: Token saved as CF_TUNNEL_TOKEN_%s", strings.ToUpper(strings.ReplaceAll(tunnelName, "-", "_")))
	logs.Info("Access your robot at: https://%s", fullHostname)
}

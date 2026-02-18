package vpn

import (
	"bytes"
	"context"
	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/util"
	"dialtone/cli/src/core/web"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"tailscale.com/tsnet"
	"embed"
)

//go:embed all:src_v1/ui
var webFS embed.FS

// RunVPN handles 'vpn <subcommand>'
func RunVPN(args []string) {
	if len(args) == 0 {
		printVPNUsage()
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
	case "provision":
		RunProvision(restArgs)
	case "test":
		if len(restArgs) > 0 && strings.HasPrefix(restArgs[0], "src_v") {
			RunVersionedTest(restArgs[0])
		} else {
			RunTest(restArgs)
		}
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
	case "serve":
		RunServe(getDir())
	case "ui-run":
		RunUIRun(getDir(), args[2:])
	case "dev":
		RunDev(getDir())
	case "build":
		RunBuild(getDir())
	case "help", "-h", "--help":
		printVPNUsage()
	default:
		// Default behavior: run as a VPN relay (legacy)
		RunLocalVPN(args)
	}
}

func printVPNUsage() {
	fmt.Println("Usage: dialtone vpn <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  provision   Generate Tailscale Auth Key")
	fmt.Println("  test        Run tsnet connectivity test (or automated suite with src_vN)")
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
	fmt.Println("  help        Show this help message")
}

func getLatestVersionDir() string {
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "vpn")
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
	fmt.Printf(">> [VPN] Fmt: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "fmt", "./src/plugins/vpn/"+versionDir+"/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunFormat(versionDir string) error {
	fmt.Printf(">> [VPN] Format: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	uiDir := filepath.Join(cwd, "src", "plugins", "vpn", versionDir, "ui")
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "format")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunVet(versionDir string) error {
	fmt.Printf(">> [VPN] Vet: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "vet", "./src/plugins/vpn/"+versionDir+"/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunGoBuild(versionDir string) error {
	fmt.Printf(">> [VPN] Go Build: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "build", "./src/plugins/vpn/"+versionDir+"/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunLint(versionDir string) error {
	fmt.Printf(">> [VPN] Lint: %s\n", versionDir)
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "vpn", versionDir, "ui")
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "lint")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunServe(versionDir string) error {
	fmt.Printf(">> [VPN] Serve: %s\n", versionDir)
	cwd, _ := os.Getwd()
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "run", filepath.ToSlash(filepath.Join("src", "plugins", "vpn", versionDir, "cmd", "main.go")))
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunUIRun(versionDir string, extraArgs []string) error {
	fmt.Printf(">> [VPN] UI Run: %s\n", versionDir)
	port := 3000
	if len(extraArgs) >= 2 && extraArgs[0] == "--port" {
		if p, err := strconv.Atoi(extraArgs[1]); err == nil {
			port = p
		}
	}
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "vpn", versionDir, "ui")
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(port))
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunDev(versionDir string) error {
	fmt.Printf(">> [VPN] Dev: %s\n", versionDir)
	// For now, just run ui-run as a simple dev mode
	return RunUIRun(versionDir, nil)
}

func RunBuild(versionDir string) error {
	fmt.Printf(">> [VPN] Build: %s\n", versionDir)
	if err := RunInstall(versionDir); err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "vpn", versionDir, "ui")
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "build")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %v", err)
	}
	fmt.Println(">> [VPN] Build successful")
	return nil
}

func RunInstall(versionDir string) error {
	fmt.Printf(">> [VPN] Install: %s\n", versionDir)
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "vpn", versionDir, "ui")
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "install")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunVersionedTest(versionDir string) error {
	fmt.Printf(">> [VPN] Running Test Suite for %s...\n", versionDir)
	cwd, _ := os.Getwd()
	testPkg := "./" + filepath.ToSlash(filepath.Join("src", "plugins", "vpn", versionDir, "test"))
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "run", testPkg)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunProvision(args []string) {
	fs := flag.NewFlagSet("provision", flag.ExitOnError)
	apiKey := fs.String("api-key", "", "Tailscale API Access Token")
	optional := fs.Bool("optional", false, "Skip instead of failing if TS_API_KEY is missing")
	fs.Parse(args)

	token := *apiKey
	if token == "" {
		token = os.Getenv("TS_API_KEY")
	}

	if token == "" {
		if *optional {
			logger.LogInfo("TS_API_KEY not found, skipping provisioning.")
			return
		}
		logger.LogFatal("Error: --api-key flag or TS_API_KEY environment variable is required.")
	}

	key, err := ProvisionKey(token, true)
	if err != nil {
		logger.LogFatal("Failed to provision key: %v", err)
	}

	logger.LogInfo("Successfully generated key: %s...", key[:10])
	updateEnv("TS_AUTHKEY", key)
	logger.LogInfo("Updated .env with new TS_AUTHKEY.")
}

// ProvisionKey generates a new auth key using the Tailscale API.
// If updateEnvFile is true, it also updates the local .env file.
func ProvisionKey(token string, updateEnvFile bool) (string, error) {
	logger.LogInfo("Generating new Tailscale Auth Key...")

	url := "https://api.tailscale.com/api/v2/tailnet/-/keys"
	payload := map[string]interface{}{
		"capabilities": map[string]interface{}{
			"devices": map[string]interface{}{
				"create": map[string]interface{}{
					"reusable":      true,
					"ephemeral":     false,
					"preauthorized": true,
				},
			},
		},
		"expirySeconds": 86400,
		"description":   "Dialtone Auto-Provisioned Key",
	}

	jsonPayload, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(token, "")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Key, nil
}

func RunTest(args []string) {
	logger.LogInfo("Starting tsnet test...")

	s := &tsnet.Server{
		Hostname: "test-vpn",
		AuthKey:  os.Getenv("TS_AUTHKEY"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.Logf = func(format string, args ...any) {
		logger.LogInfo(fmt.Sprintf("tsnet: %s", format), args...)
	}

	status, err := s.Up(ctx)
	if err != nil {
		logger.LogFatal("Failed to start tsnet server: %v", err)
	}
	logger.LogInfo("tsnet server started successfully.")
	logger.LogInfo("Tailscale Status: %v", status)
	logger.LogInfo("Tailscale IPs: %v", status.TailscaleIPs)

	// Keep the server running for a bit to allow for connections/testing
	<-ctx.Done()
	logger.LogInfo("tsnet test finished.")
}

func RunLocalVPN(args []string) {
	fs := flag.NewFlagSet("vpn", flag.ExitOnError)
	hostname := fs.String("hostname", os.Getenv("DIALTONE_HOSTNAME"), "Tailscale hostname")
	stateDir := fs.String("state-dir", "", "State directory")
	ephemeral := fs.Bool("ephemeral", false, "Register as ephemeral node")
	verbose := fs.Bool("verbose", false, "Verbose logging")
	fs.Parse(args)

	if *hostname == "" {
		*hostname = "dialtone-vpn"
	}

	if *stateDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			logger.LogFatal("Failed to get home directory: %v", err)
		}
		*stateDir = filepath.Join(homeDir, ".config", "dialtone-vpn")
	}

	if err := os.MkdirAll(*stateDir, 0700); err != nil {
		logger.LogFatal("Failed to create state directory: %v", err)
	}

	ts := &tsnet.Server{
		Hostname:  *hostname,
		Dir:       *stateDir,
		Ephemeral: *ephemeral,
		AuthKey:   os.Getenv("TS_AUTHKEY"),
		UserLogf:  logger.LogInfo,
	}
	if *verbose {
		ts.Logf = logger.LogInfo
	}
	defer ts.Close()

	// Pre-flight check for stale MagicDNS entry
	util.CheckStaleHostname(*hostname)

	logger.LogInfo("VPN Mode: Connecting to Tailscale as %s...", *hostname)
	logger.LogInfo("VPN Mode: State directory: %s", *stateDir)
	ln, err := ts.Listen("tcp", ":80")
	if err != nil {
		logger.LogFatal("VPN Mode: Failed to listen on :80: %v", err)
	}
	defer ln.Close()

	logger.LogInfo("VPN Mode: Waiting for Tailscale connection...")
	status, err := ts.Up(context.Background())
	if err != nil {
		logger.LogFatal("TS Up failed: %v", err)
	}

	ipStr := "none"
	if len(status.TailscaleIPs) > 0 {
		ipStr = status.TailscaleIPs[0].String()
	}
	logger.LogInfo("VPN Mode: Connected (IP: %s)", ipStr)
	logger.LogInfo("VPN Mode: Serving dashboard at http://%s/vpn", *hostname)

	// Use CreateWebHandler for unified dashboard
	// Pass 0 for ports since NATS isn't running
	// Pass nil for NATS server
	lc, _ := ts.LocalClient()
	webHandler := web.CreateWebHandler(*hostname, "vpn-v1", 0, 0, 80, 0, 0, nil, lc, status.TailscaleIPs, false, webFS)

	server := &http.Server{
		Handler: webHandler,
	}

	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.LogInfo("Tailscale HTTP server error: %v", err)
		}
	}()

	// Local listener for Cloudflare Tunnel
	localWebAddr := "127.0.0.1:8080"
	localWebLn, err := net.Listen("tcp", localWebAddr)
	if err == nil {
		go func() {
			logger.LogInfo("VPN Mode (Local): Serving at http://%s", localWebAddr)
			if err := http.Serve(localWebLn, webHandler); err != nil {
				logger.LogInfo("Local web server error: %v", err)
			}
		}()
	} else {
		logger.LogInfo("Warning: Failed to start local web server for VPN: %v", err)
	}

	util.WaitForShutdown()
	logger.LogInfo("Shutting down VPN mode...")
}

func updateEnv(key, value string) {
	// Update current process environment
	os.Setenv(key, value)

	envFile := "env/.env"
	content, _ := os.ReadFile(envFile)
	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, key+"=") {
			lines[i] = key + "=" + value
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, key+"="+value)
	}
	_ = os.WriteFile(envFile, []byte(strings.Join(lines, "\n")), 0644)
}

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	ssh_plugin "dialtone/dev/plugins/ssh/src_v1/go"
	tsnetv1 "dialtone/dev/plugins/tsnet/src_v1/go"
	"github.com/pkg/sftp"
	sshlib "golang.org/x/crypto/ssh"
)

type deployOptions struct {
	Host      string
	Port      string
	User      string
	Pass      string
	Ephemeral bool
	Relay     bool
	Service   bool
	SmokeTest bool
}

func RunSyncCode(versionDir string, args []string) error {
	if versionDir == "" {
		versionDir = "src_v1"
	}
	fs := flag.NewFlagSet("robot-sync-code", flag.ContinueOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	remoteDir := fs.String("remote-dir", "", "Remote source root (default: /home/<user>/dialtone_src)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*host) == "" || strings.TrimSpace(*pass) == "" {
		return fmt.Errorf("sync-code requires --host and --pass (or ROBOT_HOST/ROBOT_PASSWORD in env/.env)")
	}
	if strings.TrimSpace(*user) == "" {
		return fmt.Errorf("sync-code requires --user (or ROBOT_USER in env/.env)")
	}
	if strings.TrimSpace(*remoteDir) == "" {
		*remoteDir = path.Join("/home", strings.TrimSpace(*user), "dialtone", "src")
	}

	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return err
	}
	repoRoot := rt.RepoRoot
	client, err := ssh_plugin.DialSSH(strings.TrimSpace(*host), strings.TrimSpace(*port), strings.TrimSpace(*user), *pass)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	remoteRoot := path.Dir(*remoteDir)
	if _, err := ssh_plugin.RunSSHCommand(client, "mkdir -p "+shellQuote(*remoteDir)); err != nil {
		return fmt.Errorf("failed creating remote dir: %w", err)
	}

	localSrc := rt.SrcRoot
	if err := syncPath(client, localSrc, *remoteDir, "go.mod"); err != nil {
		return err
	}
	if err := syncPath(client, localSrc, *remoteDir, "go.sum"); err != nil {
		return err
	}
	// Sync dialtone.sh to the remote root (parent of src)
	if err := ssh_plugin.UploadFile(client, filepath.Join(repoRoot, "dialtone.sh"), path.Join(remoteRoot, "dialtone.sh")); err != nil {
		return fmt.Errorf("failed to sync dialtone.sh: %w", err)
	}
	if _, err := ssh_plugin.RunSSHCommand(client, "chmod +x "+shellQuote(path.Join(remoteRoot, "dialtone.sh"))); err != nil {
		return fmt.Errorf("failed to chmod dialtone.sh: %w", err)
	}

	robotBase := path.Join("plugins", "robot", versionDir)
	robotSyncPaths := []string{
		path.Join(robotBase, "cmd"),
		path.Join(robotBase, "ui", "src"),
		path.Join(robotBase, "ui", "public"),
		path.Join(robotBase, "ui", "index.html"),
		path.Join(robotBase, "ui", "package.json"),
		path.Join(robotBase, "ui", "bun.lock"),
		path.Join(robotBase, "ui", "tsconfig.json"),
		path.Join(robotBase, "ui", "vite.config.ts"),
	}
	for _, p := range robotSyncPaths {
		if err := syncPath(client, localSrc, *remoteDir, p); err != nil {
			return err
		}
	}
	if err := syncPath(client, localSrc, *remoteDir, path.Join("plugins", "mavlink")); err != nil {
		return err
	}
	if err := syncPath(client, localSrc, *remoteDir, path.Join("plugins", "camera")); err != nil {
		return err
	}
	if err := syncPath(client, localSrc, *remoteDir, path.Join("plugins", "logs")); err != nil {
		return err
	}
	if err := syncPath(client, localSrc, *remoteDir, path.Join("plugins", "ui", "src_v1", "ui")); err != nil {
		return err
	}

	buildHint := "cd " + shellQuote(*remoteDir) + " && go build ./plugins/robot/" + versionDir + "/cmd/server/main.go"
	logs.Info("[SYNC-CODE] Complete. Remote build command:")
	logs.Raw("  %s", buildHint)
	return nil
}

func RunDeploy(versionDir string, args []string) error {
	if versionDir == "" {
		versionDir = "src_v1"
	}
	fs := flag.NewFlagSet("robot-deploy", flag.ContinueOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	ephemeral := fs.Bool("ephemeral", false, "Register as ephemeral node on Tailscale")
	relay := fs.Bool("relay", false, "Configure local Cloudflare relay for robot Web UI from this host")
	service := fs.Bool("service", false, "Install/restart dialtone-robot.service on the robot")
	smoke := fs.Bool("smoke-test", false, "Run UI smoke test against drone-1.dialtone.earth after deploy")
	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := deployOptions{
		Host:      strings.TrimSpace(*host),
		Port:      strings.TrimSpace(*port),
		User:      strings.TrimSpace(*user),
		Pass:      *pass,
		Ephemeral: *ephemeral,
		Relay:     *relay,
		Service:   *service,
		SmokeTest: *smoke,
	}
	if opts.Port == "" {
		opts.Port = "22"
	}
	if opts.Host == "" || opts.Pass == "" {
		return fmt.Errorf("deploy requires --host and --pass (or ROBOT_HOST/ROBOT_PASSWORD in env/.env)")
	}

	return deployRobot(versionDir, opts)
}

func deployRobot(versionDir string, opts deployOptions) error {
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return err
	}
	repoRoot := rt.RepoRoot
	preset := configv1.NewPluginPreset(rt, "robot", versionDir)
	if err := ensureRobotAuthKey(repoRoot); err != nil {
		return err
	}

	logs.Info("[DEPLOY] Connecting to %s...", opts.Host)
	client, err := ssh_plugin.DialSSH(opts.Host, opts.Port, opts.User, opts.Pass)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	if err := validateSudo(client); err != nil {
		return err
	}

	// NEW: Pre-deployment checks
	logs.Info("[DEPLOY] Running pre-deployment resource checks...")
	if err := checkRemoteResources(client); err != nil {
		return err
	}

	goos, goarch, err := detectRemoteTarget(client)
	if err != nil {
		return err
	}
	logs.Info("[DEPLOY] Remote target: %s/%s", goos, goarch)

	if err := buildRobotUI(repoRoot, versionDir); err != nil {
		return err
	}
	localBin, err := buildRobotBinary(repoRoot, versionDir, goos, goarch)
	if err != nil {
		return err
	}

	remoteRoot := path.Join("/home", opts.User, ".dialtone", "robot", versionDir)
	remoteBinDir := path.Join(remoteRoot, "bin")
	remoteUIDir := path.Join(remoteRoot, "ui", "dist")
	remoteBin := path.Join(remoteBinDir, "robot-src_v1")
	remoteBinTmp := remoteBin + ".new"

	logs.Info("[DEPLOY] Preparing remote directories...")
	if _, err := ssh_plugin.RunSSHCommand(client, "mkdir -p "+shellQuote(remoteBinDir)+" "+shellQuote(remoteUIDir)); err != nil {
		return fmt.Errorf("failed to prepare remote directories: %w", err)
	}

	logs.Info("[DEPLOY] Uploading robot binary...")
	if err := ssh_plugin.UploadFile(client, localBin, remoteBinTmp); err != nil {
		return fmt.Errorf("failed to upload binary: %w", err)
	}
	if _, err := ssh_plugin.RunSSHCommand(client, "chmod +x "+shellQuote(remoteBinTmp)); err != nil {
		return fmt.Errorf("failed to chmod remote binary: %w", err)
	}

	logs.Info("[DEPLOY] Uploading UI dist...")
	localDist := preset.UIDist
	if err := uploadDir(client, localDist, remoteUIDir); err != nil {
		return fmt.Errorf("failed to upload UI dist: %w", err)
	}

	hostname := strings.TrimSpace(os.Getenv("DIALTONE_HOSTNAME"))
	if hostname == "" {
		hostname = "drone-1"
	}
	logs.Info("[DEPLOY] Pruning existing Tailscale nodes for %s to ensure clean takeover...", hostname)
	if err := pruneTailscaleNodes(hostname); err != nil {
		return fmt.Errorf("DEPLOY FAILED: Cannot prune old Tailscale nodes. Is TS_API_KEY correct and valid? Error: %w", err)
	}

	if opts.Service {
		logs.Info("[DEPLOY] Installing/restarting dialtone-robot.service...")
		if err := setupRemoteRobotService(client, opts, versionDir); err != nil {
			return err
		}
		if _, err := ssh_plugin.RunSSHCommand(client, "mv "+shellQuote(remoteBinTmp)+" "+shellQuote(remoteBin)); err != nil {
			return fmt.Errorf("failed to swap remote binary: %w", err)
		}
		if _, err := sudoRun(client, "systemctl restart dialtone-robot.service"); err != nil {
			return fmt.Errorf("failed to restart dialtone-robot.service after binary swap: %w", err)
		}
	} else {
		if _, err := ssh_plugin.RunSSHCommand(client, "mv "+shellQuote(remoteBinTmp)+" "+shellQuote(remoteBin)); err != nil {
			return fmt.Errorf("failed to swap remote binary: %w", err)
		}
	}

	if err := verifyDeployment(client, opts); err != nil {
		return err
	}

	if opts.Relay {
		logs.Info("[DEPLOY] Configuring Legion Cloudflare relay service...")
		hostname := strings.TrimSpace(os.Getenv("DIALTONE_DOMAIN"))
		if hostname == "" {
			hostname = strings.TrimSpace(os.Getenv("DIALTONE_HOSTNAME"))
		}
		if hostname == "" {
			hostname = "drone-1"
		}
		if err := setupLocalCloudflareProxyService(hostname, opts.Host); err != nil {
			return err
		}
	}

	if opts.SmokeTest {
		logs.Info("[DEPLOY] Running post-deployment UI smoke test...")
		if err := RunPostDeployUIValidation(); err != nil {
			logs.Error("[DEPLOY] UI smoke test FAILED: %v", err)
			return err
		}
		logs.Info("   [VERIFY] UI smoke test OK")
	}

	logs.Info("[DEPLOY] Deployment complete")
	return nil
}

func buildRobotUI(repoRoot, versionDir string) error {
	rt, err := configv1.ResolveRuntime(repoRoot)
	if err != nil {
		return err
	}
	preset := configv1.NewPluginPreset(rt, "robot", versionDir)
	uiDir := preset.UI
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "bun", "src_v1", "exec", "--cwd", uiDir, "run", "build")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	logs.Info("[DEPLOY] Building Robot UI...")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build UI failed: %w", err)
	}
	return nil
}

func buildRobotBinary(repoRoot, versionDir, goos, goarch string) (string, error) {
	_ = versionDir
	outDir := filepath.Join(repoRoot, ".dialtone", "bin")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	binaryName := fmt.Sprintf("robot-src_v1-%s-%s", goos, goarch)
	out := filepath.Join(outDir, binaryName)

	// Check if we need Podman for cross-compilation
	// If current arch matches target, we can build locally
	if goos == runtime.GOOS && (goarch == runtime.GOARCH || (runtime.GOARCH == "amd64" && goarch == "x86_64")) {
		dialtoneEnv := logs.GetDialtoneEnv()
		goBin := filepath.Join(dialtoneEnv, "go", "bin", "go")
		if _, err := os.Stat(goBin); err != nil {
			fallback, lookErr := exec.LookPath("go")
			if lookErr != nil {
				return "", fmt.Errorf("go binary not found (managed and PATH)")
			}
			goBin = fallback
		}

		cmd := exec.Command(goBin, "build", "-o", out, "./plugins/robot/src_v1/cmd/server/main.go")
		rt, rtErr := configv1.ResolveRuntime(repoRoot)
		if rtErr != nil {
			return "", rtErr
		}
		cmd.Dir = rt.SrcRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch, "GOROOT="+filepath.Join(dialtoneEnv, "go"))
		logs.Info("[DEPLOY] Cross-compiling robot server for %s/%s (Local)...", goos, goarch)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("local build failed: %w", err)
		}
		return out, nil
	}

	// Cross-compilation: use Podman
	logs.Info("[DEPLOY] Cross-compiling robot server for %s/%s (Podman)...", goos, goarch)

	// Check if podman is available
	if _, err := exec.LookPath("podman"); err != nil {
		return "", fmt.Errorf("podman is required for cross-compilation but not found in PATH")
	}

	// Use Dockerfile.arm for cross-compiling to arm/arm64
	dockerfilePath := filepath.Join(repoRoot, "containers", "Dockerfile.arm")
	imageName := "dialtone-builder-arm"

	buildImg := exec.Command("podman", "build", "-t", imageName, "-f", dockerfilePath, ".")
	buildImg.Dir = repoRoot
	buildImg.Stdout = os.Stdout
	buildImg.Stderr = os.Stderr
	if err := buildImg.Run(); err != nil {
		return "", fmt.Errorf("podman build failed: %w", err)
	}

	rt, err := configv1.ResolveRuntime(repoRoot)
	if err != nil {
		return "", err
	}
	srcDir := rt.SrcRoot
	remoteBinPath := "/src/plugins/robot/bin/" + binaryName
	var gcc string
	if goarch == "arm64" || goarch == "aarch64" {
		gcc = "aarch64-linux-gnu-gcc"
	} else {
		gcc = "arm-linux-gnueabihf-gcc"
	}

	dialtoneEnv := logs.GetDialtoneEnv()
	// Use host's go mod cache to avoid re-downloading
	goModCache := filepath.Join(os.Getenv("HOME"), "go", "pkg", "mod")
	if _, err := os.Stat(goModCache); os.IsNotExist(err) {
		// Fallback to DIALTONE_ENV/go/pkg/mod if exists
		altCache := filepath.Join(dialtoneEnv, "go", "pkg", "mod")
		if _, err := os.Stat(altCache); err == nil {
			goModCache = altCache
		}
	}

	podmanArgs := []string{"run", "--rm",
		"-v", srcDir + ":/src:z",
		"-w", "/src",
		"-e", "CGO_ENABLED=1",
		"-e", "GOOS=" + goos,
		"-e", "GOARCH=" + goarch,
		"-e", "CC=" + gcc,
		"-e", "GOPATH=/go",
	}

	if _, err := os.Stat(goModCache); err == nil {
		podmanArgs = append(podmanArgs, "-v", goModCache+":/go/pkg/mod:z")
	}

	podmanArgs = append(podmanArgs, imageName,
		"go", "build", "-o", remoteBinPath, "./plugins/robot/src_v1/cmd/server/main.go")

	runCmd := exec.Command("podman", podmanArgs...)
	runCmd.Dir = repoRoot
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	if err := runCmd.Run(); err != nil {
		return "", fmt.Errorf("podman run build failed: %w", err)
	}

	// Move the built binary to the expected out path
	builtBin := filepath.Join(srcDir, "plugins", "robot", "bin", binaryName)
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return "", err
	}
	if err := os.Rename(builtBin, out); err != nil {
		// Fallback to copy if rename across filesystems fails
		input, err := os.ReadFile(builtBin)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(out, input, 0755); err != nil {
			return "", err
		}
	}

	return out, nil
}

func setupRemoteRobotService(client *sshlib.Client, opts deployOptions, versionDir string) error {
	serviceName := "dialtone-robot.service"
	remoteRoot := path.Join("/home", opts.User, ".dialtone", "robot", versionDir)
	bin := path.Join(remoteRoot, "bin", "robot-src_v1")
	envPath := path.Join(remoteRoot, "robot.env")
	hostname := strings.TrimSpace(os.Getenv("DIALTONE_HOSTNAME"))
	if hostname == "" {
		hostname = "drone-1"
	}
	robotAuthKey := strings.TrimSpace(os.Getenv("ROBOT_TS_AUTHKEY"))
	if robotAuthKey == "" {
		robotAuthKey = strings.TrimSpace(os.Getenv("TS_AUTHKEY"))
	}
	ephemeral := "false"
	if opts.Ephemeral {
		ephemeral = "true"
	}
	mavEndpoint := strings.TrimSpace(os.Getenv("ROBOT_MAVLINK_ENDPOINT"))
	if mavEndpoint == "" {
		mavEndpoint = strings.TrimSpace(os.Getenv("MAVLINK_ENDPOINT"))
	}
	envLines := []string{
		"ROBOT_WEB_PORT=8080",
		"NATS_PORT=4222",
		"NATS_WS_PORT=4223",
		"ROBOT_TSNET=1",
		"ROBOT_TSNET_WEB_PORT=80",
		"ROBOT_TSNET_NATS_PORT=4222",
		"ROBOT_TSNET_WS_PORT=4223",
		"ROBOT_TSNET_EPHEMERAL=" + ephemeral,
		"DIALTONE_HOSTNAME=" + hostname,
	}
	if robotAuthKey != "" {
		envLines = append(envLines, "ROBOT_TS_AUTHKEY="+robotAuthKey)
	}
	if mavEndpoint != "" {
		envLines = append(envLines, "ROBOT_MAVLINK_ENDPOINT="+mavEndpoint)
	}
	envContent := strings.Join(envLines, "\n") + "\n"
	if err := writeRemoteFile(client, envPath, envContent); err != nil {
		return fmt.Errorf("failed to write remote env file: %w", err)
	}
	if _, err := ssh_plugin.RunSSHCommand(client, "chmod 600 "+shellQuote(envPath)); err != nil {
		return fmt.Errorf("failed to chmod remote env file: %w", err)
	}
	serviceTemplate := `[Unit]
Description=Dialtone Robot Service
After=network.target tailscaled.service

[Service]
ExecStart=%s
WorkingDirectory=%s
User=%s
EnvironmentFile=%s
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
`
	serviceContent := fmt.Sprintf(serviceTemplate, bin, remoteRoot, opts.User, envPath)
	tmpPath := "/tmp/" + serviceName
	if err := writeRemoteFile(client, tmpPath, serviceContent); err != nil {
		return fmt.Errorf("failed to write remote service file: %w", err)
	}

	commands := []string{
		fmt.Sprintf("cp %s /etc/systemd/system/%s", shellQuote(tmpPath), serviceName),
		"systemctl daemon-reload",
		"systemctl enable " + serviceName,
		"systemctl restart " + serviceName,
		"systemctl stop dialtone.service || true",
		"systemctl disable dialtone.service || true",
	}
	for _, cmd := range commands {
		if _, err := sudoRun(client, cmd); err != nil {
			return fmt.Errorf("failed remote sudo command %q: %w", cmd, err)
		}
	}
	return nil
}

func checkRemoteResources(client *sshlib.Client) error {
	// Check disk space (need at least 100MB free in /home)
	out, err := ssh_plugin.RunSSHCommand(client, "df -m /home | tail -n 1 | awk '{print $4}'")
	if err == nil {
		freeMB := 0
		fmt.Sscanf(strings.TrimSpace(out), "%d", &freeMB)
		if freeMB < 100 {
			return fmt.Errorf("insufficient disk space on remote: %d MB free (need at least 100MB)", freeMB)
		}
		logs.Info("   [CHECK] Disk space: %d MB free (OK)", freeMB)
	}

	// Check MAVLink endpoint if configured
	mavEP := strings.TrimSpace(os.Getenv("ROBOT_MAVLINK_ENDPOINT"))
	if mavEP == "" {
		mavEP = strings.TrimSpace(os.Getenv("MAVLINK_ENDPOINT"))
	}
	if strings.HasPrefix(mavEP, "serial:") {
		parts := strings.Split(mavEP, ":")
		if len(parts) >= 2 {
			dev := parts[1]
			_, err := ssh_plugin.RunSSHCommand(client, "ls "+shellQuote(dev))
			if err != nil {
				logs.Warn("   [CHECK] MAVLink serial device %s not found or inaccessible", dev)
			} else {
				logs.Info("   [CHECK] MAVLink serial device %s found (OK)", dev)
			}
		}
	}

	return nil
}

func verifyDeployment(client *sshlib.Client, opts deployOptions) error {
	if opts.Service {
		ok := false
		for i := 0; i < 15; i++ {
			out, err := sudoRun(client, "systemctl is-active dialtone-robot.service")
			if err == nil && strings.TrimSpace(out) == "active" {
				ok = true
				break
			}
			time.Sleep(1 * time.Second)
		}
		if !ok {
			status, _ := sudoRun(client, "systemctl --no-pager --full status dialtone-robot.service | head -n 60")
			return fmt.Errorf("remote dialtone-robot.service is not active\n%s", strings.TrimSpace(status))
		}
		logs.Info("   [VERIFY] Service dialtone-robot.service is active (OK)")
	}

	logs.Info("   [VERIFY] Waiting for health check http://127.0.0.1:8080/health ...")
	var last string
	var healthy bool
	for i := 0; i < 90; i++ {
		out, err := ssh_plugin.RunSSHCommand(client, "curl -fsS --max-time 5 http://127.0.0.1:8080/health || true")
		if err == nil && strings.TrimSpace(out) == "ok" {
			healthy = true
			break
		}
		last = strings.TrimSpace(out)
		time.Sleep(1 * time.Second)
	}
	if !healthy {
		return fmt.Errorf("remote health check failed, expected ok got %q", last)
	}
	logs.Info("   [VERIFY] Local health check OK")

	// Verify NATS WebSocket reachability via /natsws
	logs.Info("   [VERIFY] Checking NATS WebSocket (/natsws) ...")
	wsOut, err := ssh_plugin.RunSSHCommand(client, "curl -is --max-time 5 http://127.0.0.1:8080/natsws | head -n 1")
	if err == nil && (strings.Contains(wsOut, "101") || strings.Contains(wsOut, "400") || strings.Contains(wsOut, "Upgrade Required")) {
		// HTTP 101 Switching Protocols or 400 Bad Request (if not a proper WS handshake) indicates the endpoint exists
		logs.Info("   [VERIFY] NATS WebSocket (/natsws) reachable (OK)")
	} else {
		logs.Warn("   [VERIFY] NATS WebSocket (/natsws) check unexpected response: %q", strings.TrimSpace(wsOut))
	}

	// Verify TSNet if DIALTONE_HOSTNAME is set
	tsHost := os.Getenv("DIALTONE_HOSTNAME")
	if tsHost == "" {
		tsHost = "drone-1"
	}
	logs.Info("   [VERIFY] Checking Tailscale reachability for %s ...", tsHost)
	// We check if the process is listening on port 80 via tsnet (if configured)
	// This is harder to verify from within the robot via localhost if it's strictly tsnet-bound,
	// but we can check the process list for the listener.
	out, _ := ssh_plugin.RunSSHCommand(client, "ss -ltnp | grep ':80 ' || true")
	if strings.Contains(out, "robot-src_v1") {
		logs.Info("   [VERIFY] TSNet Web (80) listener found (OK)")
	}

	return nil
}

func setupLocalCloudflareProxyService(hostname, robotHost string) error {
	tokenKey := "CF_TUNNEL_TOKEN_" + strings.ToUpper(strings.ReplaceAll(hostname, "-", "_"))
	token := strings.TrimSpace(os.Getenv(tokenKey))
	if token == "" {
		return fmt.Errorf("missing %s in env/.env", tokenKey)
	}

	cloudflaredBin, err := ensureLocalCloudflaredBinary()
	if err != nil {
		return err
	}
	unitName := fmt.Sprintf("dialtone-proxy-%s.service", hostname)
	unitPath := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user", unitName)
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return err
	}
	originURL := fmt.Sprintf("http://%s:8080", robotHost)
	unit := strings.Join([]string{
		"[Unit]",
		"Description=Dialtone Cloudflare Proxy for " + hostname,
		"After=network.target",
		"",
		"[Service]",
		"Type=simple",
		"ExecStart=" + cloudflaredBin + " tunnel --no-autoupdate run --token " + token + " --url " + originURL,
		"Restart=always",
		"RestartSec=2",
		"",
		"[Install]",
		"WantedBy=default.target",
		"",
	}, "\n")
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return err
	}
	commands := [][]string{
		{"systemctl", "--user", "daemon-reload"},
		{"systemctl", "--user", "enable", "--now", unitName},
		{"systemctl", "--user", "restart", unitName},
		{"systemctl", "--user", "is-active", unitName},
	}
	for _, argv := range commands {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed local command %v: %w", argv, err)
		}
	}
	return nil
}

func ensureLocalCloudflaredBinary() (string, error) {
	envRoot := logs.GetDialtoneEnv()
	bin := filepath.Join(envRoot, "cloudflare", "cloudflared")
	if _, err := os.Stat(bin); err == nil {
		return bin, nil
	}
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		return "", err
	}
	url := ""
	switch runtime.GOOS {
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			url = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64"
		case "arm64":
			url = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64"
		}
	case "darwin":
		switch runtime.GOARCH {
		case "amd64":
			url = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-darwin-amd64.tgz"
		case "arm64":
			url = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-darwin-arm64.tgz"
		}
	}
	if url == "" {
		return "", fmt.Errorf("unsupported platform for cloudflared install: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	tmp := bin + ".tmp"
	cmd := exec.Command("curl", "-fsSL", url, "-o", tmp)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("cloudflared download failed: %w", err)
	}
	if strings.HasSuffix(url, ".tgz") {
		tmpDir := filepath.Join(os.TempDir(), "dialtone-cloudflared")
		_ = os.RemoveAll(tmpDir)
		if err := os.MkdirAll(tmpDir, 0o755); err != nil {
			return "", err
		}
		tarCmd := exec.Command("tar", "-xzf", tmp, "-C", tmpDir)
		if err := tarCmd.Run(); err != nil {
			return "", fmt.Errorf("cloudflared extract failed: %w", err)
		}
		if err := os.Rename(filepath.Join(tmpDir, "cloudflared"), bin); err != nil {
			return "", err
		}
		_ = os.Remove(tmp)
	} else {
		if err := os.Rename(tmp, bin); err != nil {
			return "", err
		}
	}
	if err := os.Chmod(bin, 0o755); err != nil {
		return "", err
	}
	return bin, nil
}

func uploadDir(client *sshlib.Client, localRoot, remoteRoot string) error {
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return err
	}
	defer sftpClient.Close()
	if err := filepath.WalkDir(localRoot, func(localPath string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(localRoot, localPath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		remotePath := remoteRoot
		if rel != "." {
			remotePath = path.Join(remoteRoot, rel)
		}
		if d.IsDir() {
			return sftpClient.MkdirAll(remotePath)
		}
		if err := sftpClient.MkdirAll(path.Dir(remotePath)); err != nil {
			return err
		}
		in, err := os.Open(localPath)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := sftpClient.Create(remotePath)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
		if fi, err := os.Stat(localPath); err == nil {
			_ = sftpClient.Chmod(remotePath, fi.Mode())
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func validateSudo(client *sshlib.Client) error {
	_, err := ssh_plugin.RunSSHCommand(client, "sudo -n true")
	if err != nil {
		return fmt.Errorf("remote sudo validation failed (needs passwordless sudo): %w", err)
	}
	return nil
}

func detectRemoteTarget(client *sshlib.Client) (string, string, error) {
	osOut, err := ssh_plugin.RunSSHCommand(client, "uname -s")
	if err != nil {
		return "", "", fmt.Errorf("detect remote os failed: %w", err)
	}
	archOut, err := ssh_plugin.RunSSHCommand(client, "uname -m")
	if err != nil {
		return "", "", fmt.Errorf("detect remote arch failed: %w", err)
	}
	osName := strings.ToLower(strings.TrimSpace(osOut))
	archName := strings.ToLower(strings.TrimSpace(archOut))
	goos := "linux"
	switch osName {
	case "linux":
		goos = "linux"
	case "darwin":
		goos = "darwin"
	default:
		return "", "", fmt.Errorf("unsupported remote OS %q", osName)
	}
	goarch := "arm64"
	switch archName {
	case "aarch64", "arm64":
		goarch = "arm64"
	case "armv7l", "arm":
		goarch = "arm"
	case "x86_64", "amd64":
		goarch = "amd64"
	default:
		return "", "", fmt.Errorf("unsupported remote arch %q", archName)
	}
	return goos, goarch, nil
}

func writeRemoteFile(client *sshlib.Client, remotePath, content string) error {
	cmd := fmt.Sprintf("cat > %s <<'EOF'\n%s\nEOF", shellQuote(remotePath), content)
	_, err := ssh_plugin.RunSSHCommand(client, cmd)
	return err
}

func sudoRun(client *sshlib.Client, command string) (string, error) {
	return ssh_plugin.RunSSHCommand(client, "sudo -n "+command)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func syncPath(client *sshlib.Client, localSrcRoot, remoteSrcRoot, rel string) error {
	localPath := filepath.Join(localSrcRoot, filepath.FromSlash(rel))
	remotePath := path.Join(remoteSrcRoot, rel)
	fi, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("sync missing local path %s: %w", localPath, err)
	}
	logs.Info("[SYNC-CODE] Sync %s", rel)
	if fi.IsDir() {
		if _, err := ssh_plugin.RunSSHCommand(client, "mkdir -p "+shellQuote(remotePath)); err != nil {
			return fmt.Errorf("failed preparing remote dir %s: %w", remotePath, err)
		}
		if err := uploadDir(client, localPath, remotePath); err != nil {
			return fmt.Errorf("failed syncing dir %s: %w", rel, err)
		}
		return nil
	}
	if err := ssh_plugin.UploadFile(client, localPath, remotePath); err != nil {
		return fmt.Errorf("failed syncing file %s: %w", rel, err)
	}
	return nil
}

func ensureRobotAuthKey(repoRoot string) error {
	current := strings.TrimSpace(os.Getenv("ROBOT_TS_AUTHKEY"))
	if current != "" {
		return nil
	}

	envPath := filepath.Join(repoRoot, "env", ".env")
	apiKey := strings.TrimSpace(os.Getenv("TS_API_KEY"))
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("TAILSCALE_API_KEY"))
	}
	hostname := strings.TrimSpace(os.Getenv("DIALTONE_HOSTNAME"))
	if hostname == "" {
		hostname = "drone-1"
	}
	tailnet := resolveRobotTailnet()

	if apiKey != "" {
		logs.Info("[DEPLOY] Provisioning dedicated ROBOT_TS_AUTHKEY for %s on %s...", hostname, tailnet)
		key, err := provisionRobotAuthKeyWithAPI(apiKey, tailnet, "dialtone-robot-"+hostname, []string{"dialtone", "robot", hostname})
		if err != nil && strings.Contains(strings.ToLower(err.Error()), "requested tags") {
			logs.Warn("[DEPLOY] Tailnet does not allow requested tags; retrying ROBOT_TS_AUTHKEY provisioning without tags")
			key, err = provisionRobotAuthKeyWithAPI(apiKey, tailnet, "dialtone-robot-"+hostname, nil)
		}
		if err != nil {
			return fmt.Errorf("failed to provision ROBOT_TS_AUTHKEY: %w", err)
		}
		if err := tsnetv1.UpsertEnvVar(envPath, "ROBOT_TS_AUTHKEY", key); err != nil {
			return fmt.Errorf("failed writing ROBOT_TS_AUTHKEY to %s: %w", envPath, err)
		}
		_ = os.Setenv("ROBOT_TS_AUTHKEY", key)
		logs.Info("[DEPLOY] Wrote ROBOT_TS_AUTHKEY to %s", envPath)
		return nil
	}

	fallback := strings.TrimSpace(os.Getenv("TS_AUTHKEY"))
	if fallback == "" {
		fallback = strings.TrimSpace(os.Getenv("TAILSCALE_AUTHKEY"))
	}
	if fallback == "" {
		return fmt.Errorf("missing ROBOT_TS_AUTHKEY and cannot provision one: set TS_API_KEY locally")
	}
	logs.Warn("[DEPLOY] TS_API_KEY missing; reusing existing TS_AUTHKEY for ROBOT_TS_AUTHKEY")
	if err := tsnetv1.UpsertEnvVar(envPath, "ROBOT_TS_AUTHKEY", fallback); err != nil {
		return fmt.Errorf("failed writing fallback ROBOT_TS_AUTHKEY to %s: %w", envPath, err)
	}
	_ = os.Setenv("ROBOT_TS_AUTHKEY", fallback)
	return nil
}

func resolveRobotTailnet() string {
	for _, key := range []string{"ROBOT_TS_TAILNET", "TS_TAILNET", "TAILSCALE_TAILNET"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	if tailnet, err := tsnetv1.DetectTailnetFromLocalStatus(); err == nil && strings.TrimSpace(tailnet) != "" {
		return strings.TrimSpace(tailnet)
	}
	return "shad-artichoke.ts.net"
}

func provisionRobotAuthKeyWithAPI(apiKey, tailnet, description string, tags []string) (string, error) {
	tagValues := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if !strings.HasPrefix(t, "tag:") {
			t = "tag:" + t
		}
		tagValues = append(tagValues, t)
	}
	reqBody := map[string]any{
		"capabilities": map[string]any{
			"devices": map[string]any{
				"create": map[string]any{
					"reusable":      true,
					"ephemeral":     true,
					"preauthorized": true,
					"tags":          tagValues,
				},
			},
		},
		"expirySeconds": 24 * 30 * 3600,
		"description":   description,
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	endpoint := fmt.Sprintf("https://api.tailscale.com/api/v2/tailnet/%s/keys", url.PathEscape(strings.TrimSpace(tailnet)))
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(strings.TrimSpace(apiKey), "")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("tailscale api POST %s failed: %s", endpoint, strings.TrimSpace(string(body)))
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}

	keyVal := strings.TrimSpace(extractAuthKey(parsed))
	if keyVal == "" {
		return "", fmt.Errorf("tailscale api returned empty auth key payload")
	}
	return keyVal, nil
}

func RunPostDeployUIValidation() error {
	hostname := os.Getenv("DIALTONE_HOSTNAME")
	if hostname == "" {
		hostname = "drone-1"
	}
	url := fmt.Sprintf("http://%s", hostname)

	logs.Info("   [SMOKE] Strictly verifying robot UI at %s (via embedded tsnet)...", url)

	// Use tsnet to join the network for verification
	tsHost := "dialtone-deployer-" + hostname
	cfg, err := tsnetv1.ResolveConfig(tsHost, "")
	if err != nil {
		return err
	}

	// Ensure we have an auth key for this ephemeral node
	if !cfg.AuthKeyPresent {
		apiKey := strings.TrimSpace(os.Getenv("TS_API_KEY"))
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("TAILSCALE_API_KEY"))
		}
		if apiKey != "" {
			logs.Info("   [SMOKE] Provisioning ephemeral auth key for deployer...")
			key, perr := provisionRobotAuthKeyWithAPI(apiKey, cfg.Tailnet, "dialtone-deployer-"+hostname, []string{"dialtone", "deployer", "ephemeral"})
			if perr != nil && strings.Contains(strings.ToLower(perr.Error()), "requested tags") {
				logs.Warn("   [SMOKE] Tailnet does not allow requested tags; retrying without tags")
				key, perr = provisionRobotAuthKeyWithAPI(apiKey, cfg.Tailnet, "dialtone-deployer-"+hostname, nil)
			}
			if perr == nil {
				_ = os.Setenv("TS_AUTHKEY", key)
				cfg.AuthKeyPresent = true
				cfg.AuthKeyEnv = "TS_AUTHKEY"
				logs.Info("   [SMOKE] Successfully provisioned ephemeral auth key.")
			} else {
				logs.Error("   [SMOKE] Failed to provision auth key: %v", perr)
			}
		} else {
			logs.Warn("   [SMOKE] TS_API_KEY missing; cannot auto-provision auth key for verification")
		}
	}

	srv := tsnetv1.BuildServer(cfg)
	// We make it ephemeral so it doesn't leave junk nodes
	srv.Ephemeral = true
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	logs.Info("   [SMOKE] Joining Tailnet as %s ...", tsHost)
	if _, err := srv.Up(ctx); err != nil {
		return fmt.Errorf("failed to join Tailnet for verification: %w", err)
	}

	// Give DNS/Network map a moment to propagate
	time.Sleep(3 * time.Second)

	// Now we use the tsnet HTTP client to check the robot
	client := srv.HTTPClient()

	logs.Info("   [SMOKE] Probing %s ...", url)

	var bodyStr string
	var lastErr error
	for attempt := 1; attempt <= 15; attempt++ {
		targetURL := url
		if attempt > 5 {
			// If hostname resolution keeps failing, try to find the IP via API
			ip, iperr := getTailscaleIP(hostname)
			if iperr == nil && ip != "" {
				if attempt == 6 {
					logs.Info("   [SMOKE] Hostname resolution sluggish; switching to IP probe: http://%s", ip)
				}
				targetURL = "http://" + ip
			} else if attempt == 6 {
				logs.Warn("   [SMOKE] Could not resolve IP via API: %v", iperr)
			}
		}

		resp, err := client.Get(targetURL)
		if err != nil {
			lastErr = err
			if attempt%3 == 0 {
				logs.Warn("   [SMOKE] Probe attempt %d failed: %v", attempt, err)
			}
			time.Sleep(3 * time.Second)
			continue
		}

		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			lastErr = rerr
			time.Sleep(1 * time.Second)
			continue
		}
		bodyStr = string(body)
		lastErr = nil
		break
	}

	if lastErr != nil {
		return fmt.Errorf("failed to reach robot at %s via Tailscale after retries: %w", url, lastErr)
	}

	if strings.Contains(strings.ToLower(bodyStr), "sleeping ...") {
		return fmt.Errorf("VERIFICATION FAILED: Reached the relay sleep page instead of the robot UI at %s", url)
	}

	if !strings.Contains(strings.ToLower(bodyStr), "dialtone.robot") {
		return fmt.Errorf("VERIFICATION FAILED: Robot UI content not found at %s. (Response length: %d)", url, len(bodyStr))
	}

	logs.Info("   [SMOKE] Verification SUCCESS: Robot UI is reachable and active at %s", url)
	return nil
}

func getTailscaleIP(hostname string) (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("TS_API_KEY"))
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("TAILSCALE_API_KEY"))
	}
	if apiKey == "" {
		return "", fmt.Errorf("TS_API_KEY not set")
	}
	tailnet := resolveRobotTailnet()

	endpoint := fmt.Sprintf("https://api.tailscale.com/api/v2/tailnet/%s/devices", url.PathEscape(tailnet))
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(apiKey, "")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to list devices: %s", resp.Status)
	}

	var data struct {
		Devices []struct {
			Hostname  string   `json:"hostname"`
			Name      string   `json:"name"`
			Addresses []string `json:"addresses"`
		} `json:"devices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	for _, d := range data.Devices {
		if d.Hostname == hostname || strings.HasPrefix(d.Name, hostname+".") {
			if len(d.Addresses) > 0 {
				return d.Addresses[0], nil
			}
		}
	}

	return "", fmt.Errorf("device not found")
}

func pruneTailscaleNodes(hostname string) error {
	apiKey := strings.TrimSpace(os.Getenv("TS_API_KEY"))
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("TAILSCALE_API_KEY"))
	}
	if apiKey == "" {
		return fmt.Errorf("TS_API_KEY not set; cannot prune nodes")
	}
	tailnet := resolveRobotTailnet()

	// Use tsnet plugin's internal client-from-args logic if possible,
	// or replicate simple list/delete.
	// We'll use the API directly since we have the apiKey and tailnet.

	endpoint := fmt.Sprintf("https://api.tailscale.com/api/v2/tailnet/%s/devices", url.PathEscape(tailnet))
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(apiKey, "")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("TS_API_KEY is invalid or expired. Please update it in env/.env (HTTP 401)")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to list devices: %s", resp.Status)
	}

	var data struct {
		Devices []struct {
			ID       string   `json:"id"`
			Hostname string   `json:"hostname"`
			Name     string   `json:"name"`
			Tags     []string `json:"tags"`
		} `json:"devices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}

	for _, d := range data.Devices {
		// Tailscale hostname might be 'drone-1' or 'drone-1.shad-artichoke.ts.net'
		if d.Hostname == hostname || strings.HasPrefix(d.Name, hostname+".") {
			isDialtone := false
			for _, t := range d.Tags {
				if t == "tag:dialtone" {
					isDialtone = true
					break
				}
			}
			if !isDialtone {
				logs.Warn("   [PRUNE] Skipping node %s (id=%s) because it lacks tag:dialtone (could be OS node)", d.Name, d.ID)
				continue
			}

			logs.Info("   [PRUNE] Deleting conflicting node: %s (id=%s)", d.Name, d.ID)
			deleteURL := fmt.Sprintf("https://api.tailscale.com/api/v2/device/%s", url.PathEscape(d.ID))
			delReq, _ := http.NewRequest("DELETE", deleteURL, nil)
			delReq.SetBasicAuth(apiKey, "")
			delResp, err := client.Do(delReq)
			if err != nil {
				logs.Warn("   [PRUNE] Failed to delete %s: %v", d.ID, err)
				continue
			}
			delResp.Body.Close()
			if delResp.StatusCode != http.StatusOK && delResp.StatusCode != http.StatusNoContent {
				logs.Warn("   [PRUNE] Unexpected status deleting %s: %s", d.ID, delResp.Status)
			} else {
				logs.Info("   [PRUNE] Successfully deleted %s", d.ID)
			}
		}
	}

	return nil
}

func isHostReachable(host, port string) bool {
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func extractAuthKey(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if v, ok := payload["key"].(string); ok {
		return v
	}
	if keyObj, ok := payload["key"].(map[string]any); ok {
		if v, ok := keyObj["key"].(string); ok {
			return v
		}
	}
	if authKey, ok := payload["authKey"].(string); ok {
		return authKey
	}
	return ""
}

package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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
	Proxy     bool
	Service   bool
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
		*remoteDir = path.Join("/home", strings.TrimSpace(*user), "dialtone_src")
	}

	repoRoot, err := findRepoRootFromWD()
	if err != nil {
		return err
	}
	client, err := ssh_plugin.DialSSH(strings.TrimSpace(*host), strings.TrimSpace(*port), strings.TrimSpace(*user), *pass)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	if _, err := ssh_plugin.RunSSHCommand(client, "mkdir -p "+shellQuote(*remoteDir)); err != nil {
		return fmt.Errorf("failed creating remote dir: %w", err)
	}

	localSrc := filepath.Join(repoRoot, "src")
	if err := syncPath(client, localSrc, *remoteDir, "go.mod"); err != nil {
		return err
	}
	if err := syncPath(client, localSrc, *remoteDir, "go.sum"); err != nil {
		return err
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
	logs.Raw("  " + buildHint)
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
	proxy := fs.Bool("proxy", false, "Expose Web UI via Cloudflare proxy from this host")
	service := fs.Bool("service", false, "Install/restart dialtone-robot.service on the robot")
	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := deployOptions{
		Host:      strings.TrimSpace(*host),
		Port:      strings.TrimSpace(*port),
		User:      strings.TrimSpace(*user),
		Pass:      *pass,
		Ephemeral: *ephemeral,
		Proxy:     *proxy,
		Service:   *service,
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
	repoRoot, err := findRepoRootFromWD()
	if err != nil {
		return err
	}
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
	localDist := filepath.Join(repoRoot, "src", "plugins", "robot", versionDir, "ui", "dist")
	if err := uploadDir(client, localDist, remoteUIDir); err != nil {
		return fmt.Errorf("failed to upload UI dist: %w", err)
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

	if opts.Proxy {
		logs.Info("[DEPLOY] Configuring Legion Cloudflare proxy service...")
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

	logs.Info("[DEPLOY] Deployment complete")
	return nil
}

func buildRobotUI(repoRoot, versionDir string) error {
	uiDir := filepath.Join(repoRoot, "src", "plugins", "robot", versionDir, "ui")
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
	out := filepath.Join(outDir, fmt.Sprintf("robot-src_v1-%s-%s", goos, goarch))

	goBin := filepath.Join(logs.GetDialtoneEnv(), "go", "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		fallback, lookErr := exec.LookPath("go")
		if lookErr != nil {
			return "", fmt.Errorf("go binary not found (managed and PATH)")
		}
		goBin = fallback
	}

	cmd := exec.Command(goBin, "build", "-o", out, "./plugins/robot/src_v1/cmd/server/main.go")
	cmd.Dir = filepath.Join(repoRoot, "src")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch)
	logs.Info("[DEPLOY] Cross-compiling robot server for %s/%s...", goos, goarch)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("build failed: %w", err)
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

func verifyDeployment(client *sshlib.Client, opts deployOptions) error {
	if opts.Service {
		ok := false
		for i := 0; i < 10; i++ {
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
	}

	var last string
	for i := 0; i < 90; i++ {
		out, err := ssh_plugin.RunSSHCommand(client, "curl -fsS --max-time 5 http://127.0.0.1:8080/health || true")
		if err == nil && strings.TrimSpace(out) == "ok" {
			return nil
		}
		last = strings.TrimSpace(out)
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("remote health check failed, expected ok got %q", last)
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

func findRepoRootFromWD() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", fmt.Errorf("repo root not found")
		}
		cwd = parent
	}
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
					"reusable":      false,
					"ephemeral":     false,
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

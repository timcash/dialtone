package repl

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshplugin "dialtone/dev/plugins/ssh/src_v1/go"
	sshlib "golang.org/x/crypto/ssh"
)

type deployOptions struct {
	Host      string
	Port      string
	User      string
	Pass      string
	RemoteDir string
	Service   bool
	Repo      string
	NATSURL   string
	Room      string
	Embedded  bool
}

func RunDeploy(args []string) error {
	fs := flag.NewFlagSet("repl-deploy", flag.ContinueOnError)
	host := fs.String("host", strings.TrimSpace(os.Getenv("ROBOT_HOST")), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", strings.TrimSpace(os.Getenv("ROBOT_USER")), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	remoteDir := fs.String("remote-dir", "", "Remote install root (default: /home/<user>/.dialtone/repl/src_v1)")
	service := fs.Bool("service", false, "Install/restart dialtone_repl.service on remote host")
	repo := fs.String("repo", "timcash/dialtone", "GitHub repo owner/name for auto-update")
	natsURL := fs.String("nats-url", defaultNATSURL, "NATS URL for service worker")
	room := fs.String("room", defaultRoom, "REPL room for worker")
	embedded := fs.Bool("embedded-nats", false, "Run worker with embedded NATS (default false for robot deploy)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := deployOptions{
		Host:      strings.TrimSpace(*host),
		Port:      strings.TrimSpace(*port),
		User:      strings.TrimSpace(*user),
		Pass:      *pass,
		RemoteDir: strings.TrimSpace(*remoteDir),
		Service:   *service,
		Repo:      strings.TrimSpace(*repo),
		NATSURL:   strings.TrimSpace(*natsURL),
		Room:      sanitizeRoom(*room),
		Embedded:  *embedded,
	}
	if opts.Port == "" {
		opts.Port = "22"
	}
	if opts.Room == "" {
		opts.Room = defaultRoom
	}
	if opts.Host == "" || opts.Pass == "" {
		return fmt.Errorf("deploy requires --host and --pass (or ROBOT_HOST/ROBOT_PASSWORD in env/dialtone.json)")
	}
	if opts.User == "" {
		return fmt.Errorf("deploy requires --user (or ROBOT_USER in env/dialtone.json)")
	}
	if opts.RemoteDir == "" {
		opts.RemoteDir = path.Join("/home", opts.User, ".dialtone", "repl", "src_v1")
	}

	logs.Info("[DEPLOY] Connecting to %s@%s:%s", opts.User, opts.Host, opts.Port)
	client, err := sshplugin.DialSSH(opts.Host, opts.Port, opts.User, opts.Pass)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	goos, goarch, err := detectRemoteTarget(client)
	if err != nil {
		return err
	}
	logs.Info("[DEPLOY] Remote target detected: %s/%s", goos, goarch)

	localBinary, err := buildDeployBinary(goos, goarch)
	if err != nil {
		return err
	}

	remoteBinDir := path.Join(opts.RemoteDir, "bin")
	remoteBin := path.Join(remoteBinDir, "dialtone_repl")
	remoteTmpBin := path.Join(remoteBinDir, fmt.Sprintf("dialtone_repl.upload-%d", time.Now().UnixNano()))
	remoteEnv := path.Join(opts.RemoteDir, "repl.env")
	if _, err := sshplugin.RunSSHCommand(client, "mkdir -p "+shellQuote(remoteBinDir)); err != nil {
		return fmt.Errorf("failed to create remote bin dir: %w", err)
	}
	if err := sshplugin.UploadFile(client, localBinary, remoteTmpBin); err != nil {
		return fmt.Errorf("failed to upload binary: %w", err)
	}
	if _, err := sshplugin.RunSSHCommand(client, "chmod +x "+shellQuote(remoteTmpBin)+" && mv -f "+shellQuote(remoteTmpBin)+" "+shellQuote(remoteBin)); err != nil {
		return fmt.Errorf("failed to chmod remote binary: %w", err)
	}
	logs.Info("[DEPLOY] Uploaded %s", remoteBin)

	envLines := []string{}
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		envLines = append(envLines, "GITHUB_TOKEN="+token)
	}
	if len(envLines) > 0 {
		envContent := strings.Join(envLines, "\n") + "\n"
		if err := writeRemoteFile(client, remoteEnv, envContent); err != nil {
			return fmt.Errorf("failed to write remote env: %w", err)
		}
		if _, err := sshplugin.RunSSHCommand(client, "chmod 600 "+shellQuote(remoteEnv)); err != nil {
			return fmt.Errorf("failed to chmod remote env: %w", err)
		}
		logs.Info("[DEPLOY] Wrote %s", remoteEnv)
	}

	if opts.Service {
		if err := setupRemoteReplService(client, opts, remoteBin, remoteEnv, len(envLines) > 0); err != nil {
			return err
		}
		if err := verifyRemoteReplService(client); err != nil {
			return err
		}
		logs.Info("[DEPLOY] Service active on %s", opts.Host)
	} else {
		logs.Info("[DEPLOY] Upload complete (service install skipped)")
	}
	return nil
}

func buildDeployBinary(goos, goarch string) (string, error) {
	paths, err := ResolvePaths("")
	if err != nil {
		return "", err
	}
	out := filepath.Join(paths.StandaloneBinDir, fmt.Sprintf("dialtone_repl-%s-%s", goos, goarch))
	if goos == "windows" {
		out += ".exe"
	}
	if err := os.MkdirAll(paths.StandaloneBinDir, 0o755); err != nil {
		return "", err
	}

	goBin := filepath.Join(logs.GetDialtoneEnv(), "go", "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		fallback, lookErr := exec.LookPath("go")
		if lookErr != nil {
			return "", fmt.Errorf("managed go binary not found at %s and fallback go not in PATH", goBin)
		}
		goBin = fallback
	}

	pkg := filepath.Join(paths.Preset.Cmd, "repld", "main.go")
	args := []string{"build", "-o", out, pkg}
	cmd := exec.Command(goBin, args...)
	cmd.Dir = paths.Runtime.SrcRoot
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out, nil
}

func setupRemoteReplService(client *sshlib.Client, opts deployOptions, remoteBin, remoteEnv string, hasEnv bool) error {
	if _, err := sshplugin.RunSSHCommand(client, "sudo -n true"); err != nil {
		return fmt.Errorf("remote sudo validation failed (needs passwordless sudo): %w", err)
	}
	serviceName := "dialtone_repl.service"
	serviceTemplate := `[Unit]
Description=Dialtone REPL Service
After=network.target

[Service]
Type=simple
User=%s
WorkingDirectory=%s
ExecStart=%s service --mode run --repo %s --nats-url %s --room %s --check-interval 5m --embedded-nats=%s
Restart=always
RestartSec=2
%s

[Install]
WantedBy=multi-user.target
`
	envLine := ""
	if hasEnv {
		envLine = "EnvironmentFile=" + remoteEnv
	}
	content := fmt.Sprintf(
		serviceTemplate,
		opts.User,
		opts.RemoteDir,
		remoteBin,
		shellQuoteArg(opts.Repo),
		shellQuoteArg(opts.NATSURL),
		shellQuoteArg(opts.Room),
		boolToString(opts.Embedded),
		envLine,
	)
	tmpPath := "/tmp/" + serviceName
	if err := writeRemoteFile(client, tmpPath, content); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}
	commands := []string{
		fmt.Sprintf("cp %s /etc/systemd/system/%s", shellQuote(tmpPath), serviceName),
		"systemctl daemon-reload",
		"systemctl enable " + serviceName,
		"systemctl restart " + serviceName,
	}
	for _, c := range commands {
		if _, err := sshplugin.RunSSHCommand(client, "sudo -n "+c); err != nil {
			return fmt.Errorf("remote sudo command failed %q: %w", c, err)
		}
	}
	return nil
}

func verifyRemoteReplService(client *sshlib.Client) error {
	ok := false
	for i := 0; i < 20; i++ {
		out, err := sshplugin.RunSSHCommand(client, "sudo -n systemctl is-active dialtone_repl.service")
		if err == nil && strings.TrimSpace(out) == "active" {
			ok = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if ok {
		return nil
	}
	status, _ := sshplugin.RunSSHCommand(client, "sudo -n systemctl --no-pager --full status dialtone_repl.service | head -n 80")
	return fmt.Errorf("remote dialtone_repl.service is not active\n%s", strings.TrimSpace(status))
}

func detectRemoteTarget(client *sshlib.Client) (string, string, error) {
	osOut, err := sshplugin.RunSSHCommand(client, "uname -s")
	if err != nil {
		return "", "", fmt.Errorf("detect remote os failed: %w", err)
	}
	archOut, err := sshplugin.RunSSHCommand(client, "uname -m")
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
	_, err := sshplugin.RunSSHCommand(client, cmd)
	return err
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func shellQuoteArg(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "'", "")
}

func boolToString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

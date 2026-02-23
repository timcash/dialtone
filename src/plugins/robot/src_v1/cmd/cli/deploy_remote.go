package cli

import (
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	ssh_plugin "dialtone/dev/plugins/ssh/src_v1/go"
	"github.com/pkg/sftp"
	sshlib "golang.org/x/crypto/ssh"
)

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
	out, err := ssh_plugin.RunSSHCommand(client, "df -m /home | tail -n 1 | awk '{print $4}'")
	if err == nil {
		freeMB := 0
		fmt.Sscanf(strings.TrimSpace(out), "%d", &freeMB)
		if freeMB < 100 {
			return fmt.Errorf("insufficient disk space on remote: %d MB free (need at least 100MB)", freeMB)
		}
		logs.Info("   [CHECK] Disk space: %d MB free (OK)", freeMB)
	}

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

	logs.Info("   [VERIFY] Checking NATS WebSocket (/natsws) ...")
	wsOut, err := ssh_plugin.RunSSHCommand(client, "curl -is --max-time 5 http://127.0.0.1:8080/natsws | head -n 1")
	if err == nil && (strings.Contains(wsOut, "101") || strings.Contains(wsOut, "400") || strings.Contains(wsOut, "Upgrade Required")) {
		logs.Info("   [VERIFY] NATS WebSocket (/natsws) reachable (OK)")
	} else {
		logs.Warn("   [VERIFY] NATS WebSocket (/natsws) check unexpected response: %q", strings.TrimSpace(wsOut))
	}

	tsHost := os.Getenv("DIALTONE_HOSTNAME")
	if tsHost == "" {
		tsHost = "drone-1"
	}
	logs.Info("   [VERIFY] Checking Tailscale reachability for %s ...", tsHost)
	out, _ := ssh_plugin.RunSSHCommand(client, "ss -ltnp | grep ':80 ' || true")
	if strings.Contains(out, "robot-src_v1") {
		logs.Info("   [VERIFY] TSNet Web (80) listener found (OK)")
	}

	return nil
}

func uploadDir(client *sshlib.Client, localRoot, remoteRoot string) error {
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return err
	}
	defer sftpClient.Close()
	return filepath.WalkDir(localRoot, func(localPath string, d os.DirEntry, walkErr error) error {
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
	})
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

func isHostReachable(host, port string) bool {
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

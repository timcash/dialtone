package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func setupLocalCloudflareProxyService(hostname, robotHost string) error {
	tokenKey := "CF_TUNNEL_TOKEN_" + strings.ToUpper(strings.ReplaceAll(hostname, "-", "_"))
	token := strings.TrimSpace(os.Getenv(tokenKey))
	if token == "" {
		return fmt.Errorf("missing %s in env/dialtone.json", tokenKey)
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

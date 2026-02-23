package ops

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Sleep starts the lightweight sleep server (local process) for src_v1.
// Default behavior is daemon mode via user systemd service.
// Pass --foreground to run directly in the current terminal.
func Sleep(repoRoot string, args []string) error {
	foreground := hasFlag(args, "--foreground", "-f")

	if shouldConfigureSleepProxy() && !hasHelpArg(args) {
		if err := configureSleepProxy(repoRoot); err != nil {
			if requireSleepProxyConfig() {
				return err
			}
			fmt.Fprintf(os.Stderr, "[ROBOT SLEEP] warning: %v; continuing without proxy reconfiguration\n", err)
		}
	}

	if !foreground {
		return startSleepDaemon(repoRoot)
	}

	cmdArgs := []string{"go", "src_v1", "exec", "run", "./plugins/robot/src_v1/cmd/sleep/main.go"}
	cmdArgs = append(cmdArgs, filterFlags(args, "--foreground", "-f")...)

	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), cmdArgs...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func shouldConfigureSleepProxy() bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("ROBOT_SLEEP_CONFIGURE_PROXY")))
	return raw != "0" && raw != "false" && raw != "no"
}

func requireSleepProxyConfig() bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("ROBOT_SLEEP_REQUIRE_PROXY")))
	return raw == "1" || raw == "true" || raw == "yes"
}

func hasHelpArg(args []string) bool {
	for _, a := range args {
		switch strings.TrimSpace(a) {
		case "-h", "--help", "help":
			return true
		}
	}
	return false
}

func hasFlag(args []string, values ...string) bool {
	for _, a := range args {
		for _, v := range values {
			if strings.TrimSpace(a) == v {
				return true
			}
		}
	}
	return false
}

func filterFlags(args []string, values ...string) []string {
	if len(args) == 0 {
		return nil
	}
	skip := map[string]struct{}{}
	for _, v := range values {
		skip[v] = struct{}{}
	}
	out := make([]string, 0, len(args))
	for _, a := range args {
		if _, ok := skip[strings.TrimSpace(a)]; ok {
			continue
		}
		out = append(out, a)
	}
	return out
}

func configureSleepProxy(repoRoot string) error {
	name := chooseNonEmpty(getenvTrim("DIALTONE_DOMAIN"), getenvTrim("DIALTONE_HOSTNAME"), "drone-1")
	targetURL := chooseNonEmpty(getenvTrim("ROBOT_SLEEP_PROXY_URL"), "http://127.0.0.1:8080")
	token := resolveCloudflareTunnelToken(name)
	if token == "" {
		return fmt.Errorf("missing Cloudflare tunnel token for %s (set CF_TUNNEL_TOKEN_%s or CF_TUNNEL_TOKEN)", name, strings.ToUpper(strings.ReplaceAll(name, "-", "_")))
	}
	cfBin := resolveCloudflaredPath(repoRoot)
	if cfBin == "" {
		return fmt.Errorf("cloudflared binary not found (expected in DIALTONE_ENV/cloudflare or PATH)")
	}
	serviceName := fmt.Sprintf("dialtone-proxy-%s.service", name)
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	serviceDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return err
	}
	servicePath := filepath.Join(serviceDir, serviceName)
	serviceContent := fmt.Sprintf(`[Unit]
Description=Dialtone Cloudflare Proxy for %s
After=network.target

[Service]
Type=simple
ExecStart=%s tunnel --no-autoupdate run --token %s --url %s
Restart=always
RestartSec=2

[Install]
WantedBy=default.target
`, name, cfBin, token, targetURL)
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return err
	}
	if err := runSystemctlUser("daemon-reload"); err != nil {
		return err
	}
	if err := runSystemctlUser("enable", serviceName); err != nil {
		return err
	}
	if err := runSystemctlUser("restart", serviceName); err != nil {
		if err := runSystemctlUser("start", serviceName); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stdout, "[ROBOT SLEEP] proxy active: %s -> %s\n", serviceName, targetURL)
	return nil
}

func startSleepDaemon(repoRoot string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	serviceDir := filepath.Join(home, ".config", "systemd", "user")
	serviceName := "dialtone-robot-sleep.service"
	servicePath := filepath.Join(serviceDir, serviceName)

	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return err
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=Dialtone Robot Sleep Server (relay)
After=network.target

[Service]
Type=simple
WorkingDirectory=%s
ExecStart=%s go src_v1 exec run ./plugins/robot/src_v1/cmd/sleep/main.go
Environment=DIALTONE_ENV=%s
Environment=DIALTONE_ENV_FILE=%s
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
`, repoRoot, filepath.Join(repoRoot, "dialtone.sh"), chooseNonEmpty(getenvTrim("DIALTONE_ENV"), filepath.Join(repoRoot, "..", "dialtone_dependencies")), filepath.Join(repoRoot, "env", ".env"))

	changed := true
	if b, readErr := os.ReadFile(servicePath); readErr == nil && string(b) == serviceContent {
		changed = false
	}
	if changed {
		if err := os.WriteFile(servicePath, []byte(serviceContent), fs.FileMode(0644)); err != nil {
			return err
		}
	}

	if err := runSystemctlUser("daemon-reload"); err != nil {
		return err
	}
	if err := runSystemctlUser("enable", serviceName); err != nil {
		return err
	}
	if err := runSystemctlUser("restart", serviceName); err != nil {
		if err := runSystemctlUser("start", serviceName); err != nil {
			return err
		}
	}

	fmt.Fprintf(os.Stdout, "[ROBOT SLEEP] daemon active: %s\n", serviceName)
	fmt.Fprintf(os.Stdout, "[ROBOT SLEEP] status: systemctl --user status %s\n", serviceName)
	return nil
}

func runSystemctlUser(args ...string) error {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func resolveCloudflareTunnelToken(name string) string {
	key := "CF_TUNNEL_TOKEN_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	token := strings.TrimSpace(os.Getenv(key))
	if token != "" {
		return token
	}
	return strings.TrimSpace(os.Getenv("CF_TUNNEL_TOKEN"))
}

func resolveCloudflaredPath(repoRoot string) string {
	envRoot := chooseNonEmpty(getenvTrim("DIALTONE_ENV"), filepath.Join(repoRoot, "..", "dialtone_dependencies"))
	candidate := filepath.Join(envRoot, "cloudflare", "cloudflared")
	if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
		return candidate
	}
	if p, err := exec.LookPath("cloudflared"); err == nil {
		return p
	}
	return ""
}

package ops

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	cloudflarev1 "dialtone/dev/plugins/cloudflare/src_v1/go"
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	replv3 "dialtone/dev/plugins/repl/src_v3/go/repl"
)

func findCloudflared() string {
	if override := strings.TrimSpace(os.Getenv("DIALTONE_CLOUDFLARED_BIN")); override != "" {
		return override
	}
	if rt, err := configv1.ResolveRuntime(""); err == nil {
		cfPath := filepath.Join(rt.DialtoneEnv, "cloudflare", "cloudflared")
		if _, statErr := os.Stat(cfPath); statErr == nil {
			return cfPath
		}
	}
	if p, err := exec.LookPath("cloudflared"); err == nil {
		return p
	}
	return "cloudflared"
}

func resolveDefaultTunnelURL(explicit string) string {
	if v := strings.TrimSpace(explicit); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("DIALTONE_BOOTSTRAP_HTTP_URL")); v != "" {
		return v
	}
	host := strings.TrimSpace(os.Getenv("DIALTONE_BOOTSTRAP_HTTP_HOST"))
	port := strings.TrimSpace(os.Getenv("DIALTONE_BOOTSTRAP_HTTP_PORT"))
	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "8811"
	}
	return "http://" + host + ":" + port
}

func RunRuntime(command string, args []string) error {
	switch command {
	case "login":
		return runLogin(args)
	case "tunnel":
		return runTunnel(args)
	case "serve":
		return runServeTunnel(args)
	case "robot":
		return runRobot(args)
	case "proxy":
		return runProxy(args)
	case "provision":
		return runProvision(args)
	case "setup-service":
		return fmt.Errorf("setup-service is not yet migrated to src_v1 ops")
	case "shell":
		return runShell(args)
	default:
		return fmt.Errorf("unknown cloudflare runtime command: %s", command)
	}
}

func Dev() error {
	paths, err := resolveCloudflarePaths()
	if err != nil {
		return err
	}
	cmd := runDialtone(paths.Runtime.RepoRoot, "bun", "src_v1", "exec", "--cwd", paths.Preset.UI, "run", "dev", "--host", "127.0.0.1", "--port", "3000")
	return cmd.Run()
}

func Test(version string) error {
	paths, err := resolveCloudflarePaths()
	if err != nil {
		return err
	}
	cmd := runDialtone(paths.Runtime.RepoRoot, "go", "src_v1", "exec", "run", "./plugins/cloudflare/src_v1/test")
	cmd.Dir = paths.Runtime.SrcRoot
	return cmd.Run()
}

func ParseUIRunPort(args []string) (int, error) {
	for i := 0; i < len(args); i++ {
		if args[i] == "--port" {
			if i+1 >= len(args) {
				return 0, fmt.Errorf("missing value for --port")
			}
			p, err := strconvAtoi(args[i+1])
			if err != nil {
				return 0, err
			}
			return p, nil
		}
	}
	return 3000, nil
}

func runLogin(_ []string) error {
	cf := findCloudflared()
	cmd := exec.Command(cf, "tunnel", "login")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runTunnel(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ./dialtone.sh cloudflare src_v1 tunnel <create|list|status|run|start|route|cleanup|stop> ...")
	}
	cf := findCloudflared()
	sub := args[0]
	subArgs := args[1:]

	switch sub {
	case "create":
		cmd := exec.Command(cf, append([]string{"tunnel", "create"}, subArgs...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "list":
		cmd := exec.Command(cf, append([]string{"tunnel", "list"}, subArgs...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "status":
		cmd := exec.Command(cf, append([]string{"tunnel", "list"}, subArgs...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "cleanup":
		_ = exec.Command("pkill", "-f", "cloudflared").Run()
		return nil
	case "stop":
		_ = exec.Command("pkill", "-f", "cloudflared").Run()
		return nil
	case "route":
		if len(subArgs) == 0 {
			return fmt.Errorf("usage: ./dialtone.sh cloudflare src_v1 tunnel route <name> [hostname]")
		}
		tunnelName := subArgs[0]
		hostname := ""
		if len(subArgs) > 1 {
			hostname = subArgs[1]
		}
		if hostname == "" {
			dh := strings.TrimSpace(os.Getenv("DIALTONE_HOSTNAME"))
			if dh != "" {
				hostname = dh + ".dialtone.earth"
			}
		}
		if hostname == "" {
			return fmt.Errorf("hostname required (arg or DIALTONE_HOSTNAME)")
		}
		cmd := exec.Command(cf, "tunnel", "route", "dns", tunnelName, hostname)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "run":
		if len(subArgs) < 1 {
			return fmt.Errorf("usage: ./dialtone.sh cloudflare src_v1 tunnel run <name> [--url <url>] [--token <token>]")
		}
		tunnelName := subArgs[0]
		fs := flag.NewFlagSet("tunnel-run", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		urlFlag := fs.String("url", "", "service URL")
		tokenFlag := fs.String("token", os.Getenv("CF_TUNNEL_TOKEN"), "tunnel token")
		if err := fs.Parse(subArgs[1:]); err != nil {
			return err
		}
		targetURL := resolveDefaultTunnelURL(*urlFlag)
		token := cloudflarev1.ResolveTunnelToken(tunnelName, *tokenFlag)
		cmd, err := cloudflarev1.BuildTunnelRunCommand(cf, tunnelName, targetURL, token)
		if err != nil {
			return err
		}
		return cmd.Run()
	case "start":
		if len(subArgs) < 1 {
			return fmt.Errorf("usage: ./dialtone.sh cloudflare src_v1 tunnel start <name> [--url <url>] [--token <token>]")
		}
		tunnelName := subArgs[0]
		fs := flag.NewFlagSet("tunnel-start", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		urlFlag := fs.String("url", "", "service URL")
		tokenFlag := fs.String("token", os.Getenv("CF_TUNNEL_TOKEN"), "tunnel token")
		if err := fs.Parse(subArgs[1:]); err != nil {
			return err
		}
		targetURL := resolveDefaultTunnelURL(*urlFlag)
		token := cloudflarev1.ResolveTunnelToken(tunnelName, *tokenFlag)
		cmd, err := cloudflarev1.BuildTunnelRunCommand(cf, tunnelName, targetURL, token)
		if err != nil {
			return err
		}
		return cmd.Run()
	default:
		return fmt.Errorf("unknown tunnel subcommand: %s", sub)
	}
}

func runShell(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ./dialtone.sh cloudflare src_v1 shell <up|down|status> [args]")
	}
	switch strings.TrimSpace(args[0]) {
	case "up":
		fs := flag.NewFlagSet("shell-up", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		name := fs.String("name", "shell", "Tunnel name")
		host := fs.String("host", strings.TrimSpace(os.Getenv("DIALTONE_BOOTSTRAP_HTTP_HOST")), "Bootstrap HTTP host")
		port := fs.Int("port", 0, "Bootstrap HTTP port")
		token := fs.String("token", "", "Tunnel token override")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		h := strings.TrimSpace(*host)
		if h == "" {
			h = "127.0.0.1"
		}
		p := *port
		if p <= 0 {
			if raw := strings.TrimSpace(os.Getenv("DIALTONE_BOOTSTRAP_HTTP_PORT")); raw != "" {
				if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
					p = parsed
				}
			}
		}
		if p <= 0 {
			p = 8811
		}
		if err := replv3.EnsureBootstrapHTTPRunning(h, p); err != nil {
			return err
		}
		url := fmt.Sprintf("http://%s:%d", h, p)
		runArgs := []string{"start", strings.TrimSpace(*name), "--url", url}
		if strings.TrimSpace(*token) != "" {
			runArgs = append(runArgs, "--token", strings.TrimSpace(*token))
		}
		logs.Info("starting shell tunnel connector: name=%s url=%s", strings.TrimSpace(*name), url)
		logs.Info("run the same command on other hosts to add more connectors for HA/load distribution")
		return runTunnel(runArgs)
	case "down":
		return runTunnel([]string{"stop"})
	case "status":
		url := resolveDefaultTunnelURL("")
		logs.Info("shell bootstrap url: %s", url)
		return runTunnel([]string{"status"})
	default:
		return fmt.Errorf("unknown shell subcommand: %s", args[0])
	}
}

func runServeTunnel(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: ./dialtone.sh cloudflare src_v1 serve <port-or-url>")
	}
	target := strings.TrimSpace(args[0])
	if !strings.Contains(target, "://") {
		target = "http://localhost:" + strings.TrimPrefix(target, ":")
	}
	cf := findCloudflared()
	cmd := exec.Command(cf, "tunnel", "--url", target)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runRobot(args []string) error {
	fs := flag.NewFlagSet("robot", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	name := fs.String("name", "", "robot/tunnel name")
	token := fs.String("token", "", "cloudflare tunnel token")
	urlFlag := fs.String("url", "", "target URL")
	if err := fs.Parse(args); err != nil {
		return err
	}

	robotName := strings.TrimSpace(*name)
	if robotName == "" && len(fs.Args()) > 0 {
		robotName = strings.TrimSpace(fs.Args()[0])
	}
	if robotName == "" {
		robotName = strings.TrimSpace(os.Getenv("DIALTONE_DOMAIN"))
	}
	if robotName == "" {
		robotName = strings.TrimSpace(os.Getenv("DIALTONE_HOSTNAME"))
	}
	if robotName == "" {
		return fmt.Errorf("robot name is required")
	}

	targetURL := strings.TrimSpace(*urlFlag)
	if targetURL == "" {
		targetURL = fmt.Sprintf("http://%s:80", robotName)
	}
	runToken := cloudflarev1.ResolveTunnelToken(robotName, *token)
	cmd, err := cloudflarev1.BuildTunnelRunCommand(findCloudflared(), robotName, targetURL, runToken)
	if err != nil {
		return err
	}
	return cmd.Run()
}

func runProxy(args []string) error {
	fs := flag.NewFlagSet("proxy", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	localPort := fs.Int("port", 8081, "local listen port")
	target := fs.String("target", "", "target host:port")
	if err := fs.Parse(args); err != nil {
		return err
	}
	targetAddr := strings.TrimSpace(*target)
	if targetAddr == "" && len(fs.Args()) > 0 {
		targetAddr = strings.TrimSpace(fs.Args()[0])
	}
	if targetAddr == "" {
		return fmt.Errorf("usage: ./dialtone.sh cloudflare src_v1 proxy <target> [--port <n>]")
	}

	addr := fmt.Sprintf("127.0.0.1:%d", *localPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen failed on %s: %w", addr, err)
	}
	logs.Info("TCP Proxy started: %s -> %s", addr, targetAddr)
	defer ln.Close()
	for {
		clientConn, err := ln.Accept()
		if err != nil {
			continue
		}
		go func(c net.Conn) {
			defer c.Close()
			targetConn, err := net.Dial("tcp", targetAddr)
			if err != nil {
				logs.Error("proxy dial failed: %v", err)
				return
			}
			defer targetConn.Close()
			go func() { _, _ = io.Copy(targetConn, c) }()
			_, _ = io.Copy(c, targetConn)
		}(clientConn)
	}
}

func runProvision(args []string) error {
	fs := flag.NewFlagSet("provision", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	name := fs.String("name", "", "tunnel name")
	domain := fs.String("domain", "dialtone.earth", "managed domain")
	apiToken := fs.String("api-token", os.Getenv("CLOUDFLARE_API_TOKEN"), "cloudflare api token")
	accountID := fs.String("account-id", os.Getenv("CLOUDFLARE_ACCOUNT_ID"), "cloudflare account id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	tunnelName := strings.TrimSpace(*name)
	if tunnelName == "" && len(fs.Args()) > 0 {
		tunnelName = strings.TrimSpace(fs.Args()[0])
	}
	if tunnelName == "" {
		tunnelName = strings.TrimSpace(os.Getenv("DIALTONE_HOSTNAME"))
	}
	if tunnelName == "" {
		return fmt.Errorf("usage: ./dialtone.sh cloudflare src_v1 provision <name> [--domain <domain>]")
	}
	envPath := strings.TrimSpace(os.Getenv("DIALTONE_ENV_FILE"))
	if envPath == "" {
		envPath = "env/dialtone.json"
	}
	res, err := cloudflarev1.ProvisionTunnelAndDNS(cloudflarev1.ProvisionRequest{
		TunnelName: tunnelName,
		Domain:     *domain,
		APIToken:   *apiToken,
		AccountID:  *accountID,
		EnvPath:    envPath,
	})
	if err != nil {
		return err
	}
	out := map[string]any{
		"hostname":    res.FullHostname,
		"tunnel_id":   res.TunnelID,
		"token_env":   res.EnvVarName,
		"dns_created": res.DNSCreated,
	}
	b, _ := json.Marshal(out)
	fmt.Println(string(b))
	return nil
}

func strconvAtoi(s string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(s))
}

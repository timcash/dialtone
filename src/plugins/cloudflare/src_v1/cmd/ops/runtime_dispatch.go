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
)

func findCloudflared() string {
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
		return fmt.Errorf("usage: ./dialtone.sh cloudflare src_v1 tunnel <create|list|run|route|cleanup> ...")
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
	case "cleanup":
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
			return fmt.Errorf("usage: ./dialtone.sh cloudflare src_v1 tunnel run <name> --url <url> [--token <token>]")
		}
		tunnelName := subArgs[0]
		fs := flag.NewFlagSet("tunnel-run", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		urlFlag := fs.String("url", "", "service URL")
		tokenFlag := fs.String("token", os.Getenv("CF_TUNNEL_TOKEN"), "tunnel token")
		if err := fs.Parse(subArgs[1:]); err != nil {
			return err
		}
		token := cloudflarev1.ResolveTunnelToken(tunnelName, *tokenFlag)
		cmd, err := cloudflarev1.BuildTunnelRunCommand(cf, tunnelName, *urlFlag, token)
		if err != nil {
			return err
		}
		return cmd.Run()
	default:
		return fmt.Errorf("unknown tunnel subcommand: %s", sub)
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
		envPath = "env/.env"
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

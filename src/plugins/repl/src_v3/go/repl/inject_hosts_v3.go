package repl

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"github.com/nats-io/nats.go"
)

func AddHost(args []string) error {
	fs := flag.NewFlagSet("repl-v3-add-host", flag.ContinueOnError)
	name := fs.String("name", "", "Mesh host alias")
	host := fs.String("host", "", "Host or DNS")
	user := fs.String("user", "", "SSH user")
	port := fs.String("port", "22", "SSH port")
	osName := fs.String("os", "linux", "Host OS")
	alias := fs.String("alias", "", "Comma-separated aliases")
	route := fs.String("route", "tailscale,private", "Comma-separated route preference")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*name) == "" || strings.TrimSpace(*host) == "" || strings.TrimSpace(*user) == "" {
		return fmt.Errorf("usage: ./dialtone.sh repl src_v3 add-host --name wsl --host <host> --user <user>")
	}
	cfgPath, err := resolveConfigPath()
	if err != nil {
		return err
	}
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = dialtoneConfig{
				DialtoneEnv:      strings.TrimSpace(os.Getenv("DIALTONE_ENV")),
				DialtoneRepoRoot: strings.TrimSpace(os.Getenv("DIALTONE_REPO_ROOT")),
				DialtoneUseNix:   strings.TrimSpace(os.Getenv("DIALTONE_USE_NIX")),
			}
		} else {
			return err
		}
	}
	n := meshNode{
		Name:            strings.TrimSpace(*name),
		User:            strings.TrimSpace(*user),
		Host:            strings.TrimSpace(*host),
		Port:            strings.TrimSpace(*port),
		OS:              strings.TrimSpace(*osName),
		Aliases:         parseCSV(*alias),
		RoutePreference: parseCSV(*route),
	}
	if len(n.Aliases) == 0 {
		n.Aliases = []string{n.Name}
	}
	if len(n.RoutePreference) == 0 {
		n.RoutePreference = []string{"tailscale", "private"}
	}
	mergeBootstrapHostDefaults(&n)
	n.HostCandidates = []string{n.Host}
	upserted := false
	for i := range cfg.MeshNodes {
		if strings.EqualFold(strings.TrimSpace(cfg.MeshNodes[i].Name), n.Name) {
			cfg.MeshNodes[i] = n
			upserted = true
			break
		}
	}
	if !upserted {
		cfg.MeshNodes = append(cfg.MeshNodes, n)
	}
	if err := saveConfig(cfgPath, cfg); err != nil {
		return err
	}
	if err := verifyHostPersisted(cfgPath, n); err != nil {
		return err
	}
	logs.Info("Verified mesh host %s persisted to %s", n.Name, cfgPath)
	if upserted {
		logs.Info("Updated mesh host %s (%s@%s:%s)", n.Name, n.User, n.Host, n.Port)
	} else {
		logs.Info("Added mesh host %s (%s@%s:%s)", n.Name, n.User, n.Host, n.Port)
	}
	logs.Info("You can now run: ./dialtone.sh ssh src_v1 run --host %s --cmd whoami", n.Name)
	return nil
}

func verifyHostPersisted(cfgPath string, expected meshNode) error {
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("verify mesh host write failed reading %s: %w", cfgPath, err)
	}
	for _, node := range cfg.MeshNodes {
		if !strings.EqualFold(strings.TrimSpace(node.Name), strings.TrimSpace(expected.Name)) {
			continue
		}
		if strings.TrimSpace(node.Host) != strings.TrimSpace(expected.Host) {
			return fmt.Errorf("verify mesh host write failed: expected host %q got %q", expected.Host, node.Host)
		}
		if strings.TrimSpace(node.User) != strings.TrimSpace(expected.User) {
			return fmt.Errorf("verify mesh host write failed: expected user %q got %q", expected.User, node.User)
		}
		return nil
	}
	return fmt.Errorf("verify mesh host write failed: host %q not present in %s", expected.Name, cfgPath)
}

func mergeBootstrapHostDefaults(n *meshNode) {
	if n == nil {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(n.Name), "wsl") {
		return
	}
	if port := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_WSL_PORT")); port != "" && strings.TrimSpace(n.Port) == "22" {
		n.Port = port
	}
	if osName := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_WSL_OS")); osName != "" && strings.TrimSpace(n.OS) == "linux" {
		n.OS = osName
	}
	if key := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_WSL_SSH_PRIVATE_KEY")); key != "" && strings.TrimSpace(n.SSHPrivateKey) == "" {
		n.SSHPrivateKey = key
	}
	if keyPath := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_WSL_SSH_PRIVATE_KEY_PATH")); keyPath != "" && strings.TrimSpace(n.SSHPrivateKeyPath) == "" {
		n.SSHPrivateKeyPath = keyPath
	}
}

func Inject(args []string) error {
	fs := flag.NewFlagSet("repl-v3-inject", flag.ContinueOnError)
	natsURL := fs.String("nats-url", resolveREPLNATSURL(), "NATS URL")
	room := fs.String("room", defaultRoom, "Shared room name")
	user := fs.String("user", DefaultPromptName(), "Logical user name")
	host := fs.String("host", "", "Target REPL host (routes as @host command)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	command := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if command == "" {
		return fmt.Errorf("usage: ./dialtone.sh repl src_v3 inject --user <name> [--host <name>] [--nats-url URL] [--room ROOM] <command>")
	}
	return InjectCommand(strings.TrimSpace(*natsURL), strings.TrimSpace(*room), strings.TrimSpace(*user), strings.TrimSpace(*host), command)
}

func InjectCommand(natsURL, room, user, host, command string) error {
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("command is required")
	}
	if strings.TrimSpace(natsURL) == "" {
		natsURL = resolveREPLNATSURL()
	}
	if strings.TrimSpace(room) == "" {
		room = defaultRoom
	}
	if strings.TrimSpace(user) == "" {
		user = DefaultPromptName()
	}
	host = normalizeHostTarget(host)
	command = strings.TrimPrefix(strings.TrimSpace(command), "/")
	if host != "" && !strings.HasPrefix(command, "@") {
		command = "@" + host + " " + command
	}
	if err := EnsureLeaderRunning(natsURL, room); err != nil {
		return err
	}
	nc, err := nats.Connect(strings.TrimSpace(natsURL), nats.Timeout(1500*time.Millisecond))
	if err != nil {
		return err
	}
	defer nc.Close()

	frame := busFrame{
		Type:      "command",
		From:      strings.TrimSpace(user),
		Room:      strings.TrimSpace(room),
		Version:   "src_v3",
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Message:   command,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	}
	raw, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	if err := nc.Publish(commandSubject, raw); err != nil {
		return err
	}
	return nc.FlushTimeout(1500 * time.Millisecond)
}

func normalizeHostTarget(host string) string {
	h := strings.TrimSpace(strings.ToLower(host))
	h = strings.TrimPrefix(h, "@")
	return strings.TrimSpace(h)
}

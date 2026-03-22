package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type sshOptions struct {
	host        string
	user        string
	password    string
	port        string
	command     string
	dryRun      bool
	showVersion bool
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	if err := run(os.Args[1:]); err != nil {
		exitIfErr(err, "ssh")
	}
}

func run(args []string) error {
	if len(args) > 0 {
		if args[0] == "run" {
			args = args[1:]
		}
	}
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		printUsage()
		return nil
	}
	if len(args) > 0 && args[0] == "test" {
		return runTest(args[1:])
	}

	cfg, err := parseArgs(args)
	if err != nil {
		return err
	}
	if cfg.host == "" {
		return fmt.Errorf("ssh requires --host")
	}
	if strings.EqualFold(strings.TrimSpace(cfg.host), "all") {
		return fmt.Errorf("ssh execute mode does not support --host all; use --host <name>")
	}

	node, err := resolveMeshNode(cfg.host)
	if err != nil {
		return err
	}
	return runSSHCommand(cfg, node)
}

func runSSHCommand(cfg sshOptions, node meshNode) error {
	candidates := orderedMeshHostsForSSH(node.HostCandidates, node.Host)
	if len(candidates) == 0 {
		return fmt.Errorf("no host candidates for %q", node.Name)
	}

	var lastErr error
	for _, host := range candidates {
		commandCfg := cfg
		commandCfg.host = host
		cmd, err := buildSSHCommandForHost(commandCfg, node, host)
		if err != nil {
			lastErr = err
			continue
		}
		if cfg.dryRun {
			fmt.Printf("command: %s", cmd.Path)
			for _, arg := range cmd.Args[1:] {
				fmt.Printf(" %q", arg)
			}
			fmt.Println()
			return nil
		}

		if err := execRunner(cmd); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr != nil {
		return fmt.Errorf("%s", lastErr)
	}
	return fmt.Errorf("ssh host candidates exhausted for %q", node.Name)
}

func runTest(args []string) error {
	cfg, err := parseArgs(args)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.host) == "" {
		return fmt.Errorf("ssh test requires --host")
	}
	cfg.command = "printf READY"
	if strings.EqualFold(strings.TrimSpace(cfg.host), "all") {
		nodes, err := loadMeshConfig()
		if err != nil {
			return err
		}
		if len(nodes) == 0 {
			return fmt.Errorf("no mesh nodes found in env/dialtone.json mesh_nodes or env/mesh.json")
		}
		failed := 0
		for _, node := range nodes {
			if err := runSSHCommandTest(cfg, node); err != nil {
				fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", node.Name, err)
				failed++
			} else {
				fmt.Printf("PASS %s\n", node.Name)
			}
		}
		if failed > 0 {
			return fmt.Errorf("%d ssh test(s) failed", failed)
		}
		return nil
	}

	node, err := resolveMeshNode(cfg.host)
	if err != nil {
		return err
	}
	if err := runSSHCommandTest(cfg, node); err != nil {
		return err
	}
	fmt.Printf("PASS %s\n", node.Name)
	return nil
}

func runSSHCommandTest(cfg sshOptions, node meshNode) error {
	return runSSHCommand(cfg, node)
}

func parseArgs(argv []string) (sshOptions, error) {
	opts := sshOptions{port: ""}
	positional := make([]string, 0)

	for i := 0; i < len(argv); i++ {
		current := strings.TrimSpace(argv[i])
		if current == "" {
			continue
		}

		switch {
		case strings.EqualFold(current, "--host"):
			if i+1 < len(argv) {
				opts.host = strings.TrimSpace(argv[i+1])
				i++
			}
		case strings.HasPrefix(current, "--host="):
			opts.host = strings.TrimSpace(strings.TrimPrefix(current, "--host="))
		case strings.EqualFold(current, "--node"):
			return sshOptions{}, fmt.Errorf("use --host instead of --node; --node is not supported")
		case strings.HasPrefix(current, "--node="):
			return sshOptions{}, fmt.Errorf("use --host instead of --node; --node is not supported")
		case strings.EqualFold(current, "--user"):
			if i+1 < len(argv) {
				opts.user = strings.TrimSpace(argv[i+1])
				i++
			}
		case strings.HasPrefix(current, "--user="):
			opts.user = strings.TrimSpace(strings.TrimPrefix(current, "--user="))
		case strings.EqualFold(current, "--password"):
			if i+1 < len(argv) {
				opts.password = argv[i+1]
				i++
			}
		case strings.HasPrefix(current, "--password="):
			opts.password = strings.TrimPrefix(current, "--password=")
		case strings.EqualFold(current, "--port"):
			if i+1 < len(argv) {
				opts.port = strings.TrimSpace(argv[i+1])
				i++
			}
		case strings.HasPrefix(current, "--port="):
			opts.port = strings.TrimSpace(strings.TrimPrefix(current, "--port="))
		case strings.EqualFold(current, "--command"):
			if i+1 < len(argv) {
				opts.command = strings.TrimSpace(argv[i+1])
				i++
			}
		case strings.HasPrefix(current, "--command="):
			opts.command = strings.TrimSpace(strings.TrimPrefix(current, "--command="))
		case strings.EqualFold(current, "--nixpkgs-url"):
			return sshOptions{}, fmt.Errorf("--nixpkgs-url is no longer supported; run ssh v1 through ./dialtone_mod so Dialtone manages the nix shell")
		case strings.HasPrefix(current, "--nixpkgs-url="):
			return sshOptions{}, fmt.Errorf("--nixpkgs-url is no longer supported; run ssh v1 through ./dialtone_mod so Dialtone manages the nix shell")
		case strings.EqualFold(current, "--dry-run"):
			opts.dryRun = true
		case strings.HasPrefix(current, "--"):
			return sshOptions{}, fmt.Errorf("unknown flag: %s", current)
		default:
			positional = append(positional, current)
		}
	}

	if opts.command == "" && len(positional) > 0 {
		opts.command = strings.Join(positional, " ")
	}

	return opts, nil
}

type meshNode struct {
	Name           string   `json:"name"`
	Aliases        []string `json:"aliases"`
	User           string   `json:"user"`
	Password       string   `json:"password"`
	Host           string   `json:"host"`
	HostCandidates []string `json:"host_candidates"`
	Port           string   `json:"port"`
}

type dialtoneEnvConfig struct {
	MeshNodes []meshNode `json:"mesh_nodes"`
}

func resolveMeshNode(raw string) (meshNode, error) {
	normalized := normalizeHost(raw)
	if normalized == "" {
		return meshNode{}, fmt.Errorf("ssh host is required")
	}

	nodes, err := loadMeshConfig()
	if err == nil {
		if node, ok := resolveMeshNodeFromConfig(nodes, raw); ok {
			return node, nil
		}
	}

	return meshNode{
		Name: raw,
		Host: raw,
		User: os.Getenv("USER"),
		Port: "22",
	}, nil
}

func resolveMeshNodeFromConfig(nodes []meshNode, raw string) (meshNode, bool) {
	normalized := normalizeHost(raw)
	if normalized == "" {
		return meshNode{}, false
	}

	for _, n := range nodes {
		if normalizeHost(n.Name) == normalized {
			return n, true
		}
		for _, alias := range n.Aliases {
			if normalizeHost(alias) == normalized {
				return n, true
			}
		}
	}

	return meshNode{}, false
}

func normalizeHost(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return ""
	}
	return strings.TrimSuffix(v, ".")
}

func loadMeshConfig() ([]meshNode, error) {
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return nil, err
	}

	dialtonePath := filepath.Join(repoRoot, "env", "dialtone.json")
	if raw, err := os.ReadFile(dialtonePath); err == nil {
		cfg := dialtoneEnvConfig{}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return nil, err
		}
		if len(cfg.MeshNodes) > 0 {
			return cfg.MeshNodes, nil
		}
	}

	meshPath := filepath.Join(repoRoot, "env", "mesh.json")
	raw, err := os.ReadFile(meshPath)
	if err != nil {
		return nil, err
	}
	nodes := []meshNode{}
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

func buildSSHCommand(cfg sshOptions, node meshNode) (*exec.Cmd, error) {
	host := selectSSHMeshHost(node.HostCandidates, node.Host)
	if host == "" {
		return nil, fmt.Errorf("mesh host is missing for %q", node.Name)
	}
	return buildSSHCommandForHost(cfg, node, host)
}

func buildSSHCommandForHost(cfg sshOptions, node meshNode, host string) (*exec.Cmd, error) {
	host = strings.TrimSuffix(strings.TrimSpace(host), ".")
	if host == "" {
		return nil, fmt.Errorf("mesh host is missing for %q", node.Name)
	}

	remoteUser := strings.TrimSpace(cfg.user)
	if remoteUser == "" {
		remoteUser = strings.TrimSpace(node.User)
	}
	if remoteUser == "" {
		remoteUser = strings.TrimSpace(os.Getenv("USER"))
	}

	remotePort := strings.TrimSpace(cfg.port)
	if remotePort == "" {
		remotePort = strings.TrimSpace(node.Port)
	}
	if remotePort == "" {
		remotePort = "22"
	}

	sshArgs := []string{
		"-F", "/dev/null",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "GlobalKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-o", "ConnectTimeout=5",
	}
	password := cfg.password
	if password == "" {
		password = strings.TrimSpace(node.Password)
	}
	if password == "" {
		sshArgs = append(sshArgs, "-o", "BatchMode=yes")
	} else {
		sshArgs = append(sshArgs,
			"-tt",
			"-o", "BatchMode=no",
			"-o", "PreferredAuthentications=password,keyboard-interactive",
			"-o", "PubkeyAuthentication=no",
		)
	}
	if remotePort != "" && remotePort != "22" {
		sshArgs = append(sshArgs, "-p", remotePort)
	}
	target := fmt.Sprintf("%s@%s", remoteUser, host)
	sshArgs = append(sshArgs, target)
	if cfg.command != "" {
		sshArgs = append(sshArgs, cfg.command)
	}

	sshBin, err := shellSSHBinary()
	if err != nil {
		return nil, err
	}
	if password != "" {
		return buildPasswordSSHCommand(sshBin, password, sshArgs), nil
	}
	return exec.Command(sshBin, sshArgs...), nil
}

func buildPasswordSSHCommand(sshBin string, password string, sshArgs []string) *exec.Cmd {
	expectScript := fmt.Sprintf(`
log_user 1
set timeout 15
set sshbin %s
set password %s
set sshargs [list %s]
eval spawn -noecho $sshbin $sshargs
expect {
    -re "(?i)password:" {
        send -- "$password\r"
        exp_continue
    }
    eof {
        catch wait result
        set code [lindex $result 3]
        if {$code eq ""} {
            exit 0
        }
        exit $code
    }
}
`, tclQuote(sshBin), tclQuote(password), tclJoinList(sshArgs))
	return exec.Command("expect", "-c", expectScript)
}

func tclQuote(value string) string {
	replacer := strings.NewReplacer(
		`\\`, `\\\\`,
		`"`, `\"`,
		`$`, `\$`,
		`[`, `\[`,
		`]`, `\]`,
	)
	return `"` + replacer.Replace(value) + `"`
}

func tclJoinList(values []string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, tclQuote(value))
	}
	return strings.Join(parts, " ")
}

func orderedMeshHostsForSSH(candidates []string, host string) []string {
	return preferTailnetHostsForSSH(append(append([]string{}, candidates...), host))
}

func selectSSHMeshHost(candidates []string, host string) string {
	ordered := orderedMeshHostsForSSH(candidates, host)
	for _, c := range ordered {
		c = strings.TrimSpace(c)
		if c != "" {
			return strings.TrimSuffix(c, ".")
		}
	}
	return strings.TrimSuffix(strings.TrimSpace(host), ".")
}

func preferTailnetHostsForSSH(candidates []string) []string {
	out := make([]string, 0, len(candidates))
	tailnet := make([]string, 0, len(candidates))
	others := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}

	for _, c := range candidates {
		c = strings.TrimSuffix(strings.TrimSpace(c), ".")
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		if strings.HasSuffix(strings.ToLower(c), ".ts.net") {
			tailnet = append(tailnet, c)
		} else {
			others = append(others, c)
		}
	}
	out = append(out, tailnet...)
	out = append(out, others...)
	return out
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  ./dialtone_mod ssh v1 [run] --host <name|ip> [--user <user>] [--password <password>] [--port <port>] [--command <cmd>] [--dry-run]")
	fmt.Println("  ./dialtone_mod ssh v1 test [--host <name|all|ip>] [--user <user>] [--password <password>] [--port <port>] [--dry-run]")
	fmt.Println("Aliases are loaded from env/dialtone.json mesh_nodes, with env/mesh.json as a fallback (for example gold, wsl, rover, grey).")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", context, err)
	os.Exit(1)
}

func shellSSHBinary() (string, error) {
	if os.Getenv("DIALTONE_NIX_ACTIVE") != "1" {
		return "", fmt.Errorf("ssh v1 must run inside the Dialtone nix shell; use ./dialtone_mod ssh v1 ...")
	}
	sshBin := strings.TrimSpace(os.Getenv("DIALTONE_SSH_BIN"))
	if sshBin == "" {
		return "", fmt.Errorf("ssh v1 requires DIALTONE_SSH_BIN from the Dialtone nix shell")
	}
	clean := filepath.Clean(sshBin)
	if !strings.HasPrefix(clean, "/nix/") {
		return "", fmt.Errorf("ssh v1 requires nix-provided ssh, got %s", clean)
	}
	return clean, nil
}

var execRunner = func(cmd *exec.Cmd) error {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

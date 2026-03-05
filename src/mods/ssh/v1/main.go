package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type sshOptions struct {
	host        string
	user        string
	port        string
	command     string
	nixpkgsURL  string
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

	cfg := parseArgs(args)
	if cfg.host == "" {
		return fmt.Errorf("ssh requires --host or positional host argument")
	}

	node, err := resolveMeshNode(cfg.host)
	if err != nil {
		return err
	}

	cmd, err := buildSSHCommand(cfg, node)
	if err != nil {
		return err
	}
	if cfg.dryRun {
		fmt.Printf("nix command: %s", cmd.Path)
		for _, arg := range cmd.Args[1:] {
			fmt.Printf(" %q", arg)
		}
		fmt.Println()
		return nil
	}

	return execRunner(cmd)
}

func parseArgs(argv []string) sshOptions {
	opts := sshOptions{
		port:       "",
		nixpkgsURL: "https://channels.nixos.org/nixpkgs-unstable/nixexprs.tar.xz",
	}
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
		case strings.EqualFold(current, "--user"):
			if i+1 < len(argv) {
				opts.user = strings.TrimSpace(argv[i+1])
				i++
			}
		case strings.HasPrefix(current, "--user="):
			opts.user = strings.TrimSpace(strings.TrimPrefix(current, "--user="))
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
			if i+1 < len(argv) {
				opts.nixpkgsURL = strings.TrimSpace(argv[i+1])
				i++
			}
		case strings.HasPrefix(current, "--nixpkgs-url="):
			opts.nixpkgsURL = strings.TrimSpace(strings.TrimPrefix(current, "--nixpkgs-url="))
		case strings.EqualFold(current, "--dry-run"):
			opts.dryRun = true
		case strings.HasPrefix(current, "--"):
			// Unknown flag; ignore to keep parsing permissive.
		default:
			positional = append(positional, current)
		}
	}

	if opts.host == "" && len(positional) > 0 {
		opts.host = strings.TrimSpace(positional[0])
	}
	if opts.command == "" && len(positional) > 1 {
		opts.command = strings.Join(positional[1:], " ")
	}

	return opts
}

type meshNode struct {
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
	User    string   `json:"user"`
	Host    string   `json:"host"`
	Port    string   `json:"port"`
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

	configPath := filepath.Join(repoRoot, "env", "mesh.json")
	raw, err := os.ReadFile(configPath)
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
	host := strings.TrimSpace(node.Host)
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

	nixBin := strings.TrimSpace(os.Getenv("NIX_BIN"))
	if nixBin == "" {
		var err error
		nixBin, err = locateNixBinary()
		if err != nil {
			return nil, err
		}
	}

	args := []string{
		"--extra-experimental-features", "nix-command",
		"--extra-experimental-features", "flakes",
		"shell",
		"-f", cfg.nixpkgsURL,
		"openssh",
		"--command", "ssh",
		"-F", "/dev/null",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "GSSAPIAuthentication=no",
	}
	if remotePort != "" && remotePort != "22" {
		args = append(args, "-p", remotePort)
	}
	target := fmt.Sprintf("%s@%s", remoteUser, host)
	args = append(args, target)
	if cfg.command != "" {
		args = append(args, cfg.command)
	}

	return exec.Command(nixBin, args...), nil
}

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod ssh v1 [run] --host <name|ip> [--user <user>] [--port <port>] [--command <cmd>] [--dry-run]")
	fmt.Println("Aliases are loaded from env/mesh.json (for example gold, wsl, rover, grey).")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", context, err)
	os.Exit(1)
}

func locateNixBinary() (string, error) {
	if p := strings.TrimSpace(os.Getenv("NIX_BIN")); p != "" {
		return p, nil
	}
	if p, err := exec.LookPath("nix"); err == nil {
		return p, nil
	}

	candidates := []string{
		"/usr/local/bin/nix",
		"/nix/var/nix/profiles/default/bin/nix",
		filepath.Join(os.Getenv("HOME"), ".nix-profile/bin/nix"),
		"/run/current-system/sw/bin/nix",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	matches, err := filepath.Glob("/nix/store/*-nix-*/bin/nix")
	if err == nil && len(matches) > 0 {
		return matches[len(matches)-1], nil
	}

	return "", errors.New("nix executable not found. Set NIX_BIN or install nix")
}

var execRunner = func(cmd *exec.Cmd) error {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

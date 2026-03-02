package ssh

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type MeshNode struct {
	Name                string
	Aliases             []string
	User                string
	Host                string
	HostCandidates      []string
	Port                string
	OS                  string
	PreferWSLPowerShell bool
	RepoCandidates      []string
}

type CommandOptions struct {
	User     string
	Port     string
	Password string
}

var defaultMeshNodes = []MeshNode{
	{
		Name:    "wsl",
		Aliases: []string{"wsl", "legion-wsl-1", "legion-wsl-1.shad-artichoke.ts.net"},
		User:    "user",
		Host:    "192.168.4.52",
		HostCandidates: []string{
			"legion-wsl-1.shad-artichoke.ts.net",
		},
		Port: "22",
		OS:   "linux",
		RepoCandidates: []string{
			"/home/user/dialtone",
		},
	},
	{
		Name:    "gold",
		Aliases: []string{"gold", "gold.shad-artichoke.ts.net"},
		User:    "user",
		Host:    "192.168.4.55",
		Port:    "22",
		OS:      "macos",
		RepoCandidates: []string{
			"/Users/user/dialtone",
			"/Users/user/Documents/dialtone",
		},
	},
	{
		Name:    "darkmac",
		Aliases: []string{"darkmac", "darkmac.shad-artichoke.ts.net"},
		User:    "tim",
		Host:    "192.168.4.31",
		HostCandidates: []string{
			"darkmac.shad-artichoke.ts.net",
		},
		Port: "22",
		OS:   "macos",
		RepoCandidates: []string{
			"/Users/tim/dialtone",
			"/Users/tim/dialtone",
			"/Users/tim/Documents/dialtone",
			"/Users/dialtone/dialtone",
		},
	},
	{
		Name:    "rover",
		Aliases: []string{"rover", "rover-1", "rover-1.shad-artichoke.ts.net"},
		User:    "tim",
		Host:    "192.168.4.36",
		HostCandidates: []string{
			"169.254.217.151", // Rover direct ethernet on the Legion switch
			"rover-1.shad-artichoke.ts.net",
		},
		Port: "22",
		OS:   "linux",
		RepoCandidates: []string{
			"/home/tim/dialtone",
			"/home/user/dialtone",
		},
	},
	{
		Name:    "legion",
		Aliases: []string{"legion", "legion.shad-artichoke.ts.net"},
		User:    "timca",
		Host:    "192.168.4.52",
		HostCandidates: []string{
			"127.0.0.1",
			"legion.shad-artichoke.ts.net",
		},
		Port:                "2223",
		OS:                  "windows",
		PreferWSLPowerShell: true,
		RepoCandidates: []string{
			"/home/user/dialtone",
			"/home/user/dialtone",
			"/home/tim/dialtone",
			"/mnt/c/Users/timca/dialtone",
			"/mnt/c/Users/tim/dialtone",
			"/mnt/c/Users/timca/code3/dialtone",
		},
	},
}

var (
	isWSLFunc       = isWSL
	execCommandFunc = exec.Command
	dialSSHFunc     = DialSSH
	runSSHFunc      = RunSSHCommand
	canReachHostFn  = canReachHostPort
)

func dialMeshClient(host, port, user, password string) (*ssh.Client, error) {
	// Always prefer key-based auth for mesh nodes. If a password is provided,
	// use it only as a fallback after key-only auth fails.
	client, err := dialSSHFunc(host, port, user, "")
	if err == nil {
		return client, nil
	}
	if strings.TrimSpace(password) == "" {
		return nil, err
	}
	client, passErr := dialSSHFunc(host, port, user, password)
	if passErr == nil {
		return client, nil
	}
	return nil, fmt.Errorf("key auth failed: %v; password fallback failed: %w", err, passErr)
}

func ListMeshNodes() []MeshNode {
	out := make([]MeshNode, len(defaultMeshNodes))
	copy(out, defaultMeshNodes)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func ResolveMeshNode(target string) (MeshNode, error) {
	t := normalizeTarget(target)
	if t == "" {
		return MeshNode{}, fmt.Errorf("target is required")
	}
	for _, n := range defaultMeshNodes {
		if normalizeTarget(n.Name) == t {
			return n, nil
		}
		for _, a := range n.Aliases {
			if normalizeTarget(a) == t {
				return n, nil
			}
		}
	}
	return MeshNode{}, fmt.Errorf("unknown mesh node %q", target)
}

func RunNodeCommand(target string, command string, opts CommandOptions) (string, error) {
	node, err := ResolveMeshNode(target)
	if err != nil {
		return "", err
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	if shouldUseLocalPowerShell(node) {
		return runPowerShellCommand(command)
	}

	user := strings.TrimSpace(opts.User)
	if user == "" {
		user = node.User
	}
	port := strings.TrimSpace(opts.Port)
	if port == "" {
		port = node.Port
	}
	host := resolvePreferredHost(node, port)
	client, err := dialMeshClient(host, port, user, opts.Password)
	if err != nil {
		return "", fmt.Errorf("ssh dial %s@%s:%s failed: %w", user, host, port, err)
	}
	defer client.Close()

	out, err := runSSHFunc(client, command)
	if err != nil {
		return out, fmt.Errorf("ssh command failed on %s: %w", node.Name, err)
	}
	return out, nil
}

// DialMeshNode resolves a mesh alias and opens an SSH client using the same
// host selection logic as RunNodeCommand.
func DialMeshNode(target string, opts CommandOptions) (*ssh.Client, MeshNode, string, string, error) {
	node, err := ResolveMeshNode(target)
	if err != nil {
		return nil, MeshNode{}, "", "", err
	}
	if shouldUseLocalPowerShell(node) {
		return nil, MeshNode{}, "", "", fmt.Errorf("mesh node %s uses powershell transport, ssh unavailable", node.Name)
	}
	user := strings.TrimSpace(opts.User)
	if user == "" {
		user = node.User
	}
	port := strings.TrimSpace(opts.Port)
	if port == "" {
		port = node.Port
	}
	if port == "" {
		port = "22"
	}
	host := resolvePreferredHost(node, port)
	client, err := dialMeshClient(host, port, user, opts.Password)
	if err != nil {
		return nil, MeshNode{}, "", "", fmt.Errorf("ssh dial %s@%s:%s failed: %w", user, host, port, err)
	}
	return client, node, host, port, nil
}

func UploadNodeFile(target, localPath, remotePath string, opts CommandOptions) error {
	node, err := ResolveMeshNode(target)
	if err != nil {
		return err
	}
	if shouldUseLocalPowerShell(node) {
		return fmt.Errorf("mesh upload for %s via powershell transport is not supported", node.Name)
	}
	user := strings.TrimSpace(opts.User)
	if user == "" {
		user = node.User
	}
	port := strings.TrimSpace(opts.Port)
	if port == "" {
		port = node.Port
	}
	host := resolvePreferredHost(node, port)
	client, err := dialMeshClient(host, port, user, opts.Password)
	if err != nil {
		return fmt.Errorf("ssh dial %s@%s:%s failed: %w", user, host, port, err)
	}
	defer client.Close()
	if err := UploadFile(client, localPath, remotePath); err != nil {
		return fmt.Errorf("upload failed on %s: %w", node.Name, err)
	}
	return nil
}

func resolvePreferredHost(node MeshNode, port string) string {
	seen := map[string]struct{}{}
	candidates := make([]string, 0, len(node.HostCandidates)+1)
	for _, h := range node.HostCandidates {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		candidates = append(candidates, h)
	}
	if strings.TrimSpace(node.Host) != "" {
		if _, ok := seen[node.Host]; !ok {
			candidates = append(candidates, node.Host)
		}
	}
	if len(candidates) == 0 {
		return strings.TrimSpace(node.Host)
	}
	for _, h := range candidates {
		if canReachHostFn(h, port, 450*time.Millisecond) {
			return h
		}
	}
	return strings.TrimSpace(node.Host)
}

func canReachHostPort(host, port string, timeout time.Duration) bool {
	host = strings.TrimSpace(host)
	port = strings.TrimSpace(port)
	if host == "" || port == "" {
		return false
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func shouldUseLocalPowerShell(node MeshNode) bool {
	return node.PreferWSLPowerShell && isWSLFunc()
}

func ResolveCommandTransport(target string) (string, error) {
	node, err := ResolveMeshNode(target)
	if err != nil {
		return "", err
	}
	if shouldUseLocalPowerShell(node) {
		return "powershell", nil
	}
	return "ssh", nil
}

func runPowerShellCommand(command string) (string, error) {
	powerShellPath := "powershell.exe"
	if _, err := exec.LookPath(powerShellPath); err != nil {
		fallback := "/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
		if _, serr := os.Stat(fallback); serr == nil {
			powerShellPath = fallback
		}
	}

	command = strings.TrimSpace(command)
	psCommand := command
	// Most callers provide POSIX shell command strings; route those through WSL bash.
	if looksLikePosixShell(command) {
		psCommand = "Set-Location C:\\; wsl.exe -e bash -lc '" + strings.ReplaceAll(command, "'", "''") + "'"
	}
	cmd := execCommandFunc(powerShellPath, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", psCommand)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("powershell command failed: %w", err)
	}
	return string(out), nil
}

func looksLikePosixShell(command string) bool {
	c := strings.TrimSpace(command)
	return strings.Contains(c, "&&") ||
		strings.Contains(c, "./") ||
		strings.Contains(c, "/home/") ||
		strings.Contains(c, "/mnt/") ||
		strings.Contains(c, "cd ~/") ||
		strings.Contains(c, "export ")
}

func isWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if strings.TrimSpace(os.Getenv("WSL_DISTRO_NAME")) != "" {
		return true
	}
	raw, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	v := strings.ToLower(string(raw))
	return strings.Contains(v, "microsoft") || strings.Contains(v, "wsl")
}

func normalizeTarget(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.TrimSuffix(v, ".")
	return v
}

func RunMeshCommandAll(command string, opts CommandOptions) map[string]error {
	results := map[string]error{}
	for _, node := range ListMeshNodes() {
		_, err := RunNodeCommand(node.Name, command, opts)
		results[node.Name] = err
		time.Sleep(20 * time.Millisecond)
	}
	return results
}

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
		Host:    "legion-wsl-1.shad-artichoke.ts.net",
		Port:    "22",
		OS:      "linux",
		RepoCandidates: []string{
			"/home/user/dialtone",
		},
	},
	{
		Name:    "chroma",
		Aliases: []string{"chroma", "chroma-1", "chroma-1.shad-artichoke.ts.net"},
		User:    "dev",
		Host:    "chroma-1.shad-artichoke.ts.net",
		Port:    "22",
		OS:      "macos",
		RepoCandidates: []string{
			"/Users/dev/dialtone",
			"/Users/dev/dialtone",
			"/Users/dev/Documents/dialtone",
		},
	},
	{
		Name:    "darkmac",
		Aliases: []string{"darkmac", "darkmac.shad-artichoke.ts.net"},
		User:    "tim",
		Host:    "darkmac.shad-artichoke.ts.net",
		Port:    "22",
		OS:      "macos",
		RepoCandidates: []string{
			"/Users/tim/dialtone",
			"/Users/tim/dialtone",
			"/Users/tim/Documents/dialtone",
		},
	},
	{
		Name:    "rover",
		Aliases: []string{"rover", "rover-1", "rover-1.shad-artichoke.ts.net"},
		User:    "tim",
		Host:    "rover-1.shad-artichoke.ts.net",
		HostCandidates: []string{
			"169.254.217.151", // Rover direct ethernet on the Legion switch
		},
		Port: "22",
		OS:   "linux",
		RepoCandidates: []string{
			"/home/tim/dialtone",
			"/home/user/dialtone",
		},
	},
	{
		Name:                "legion",
		Aliases:             []string{"legion", "legion.shad-artichoke.ts.net"},
		User:                "timca",
		Host:                "legion.shad-artichoke.ts.net",
		Port:                "2223",
		OS:                  "windows",
		PreferWSLPowerShell: false,
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
	client, err := dialSSHFunc(host, port, user, opts.Password)
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

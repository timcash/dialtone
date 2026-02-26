package ssh

import (
	"fmt"
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
	Port                string
	OS                  string
	PreferWSLPowerShell bool
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
	},
	{
		Name:    "chroma",
		Aliases: []string{"chroma", "chroma-1", "chroma-1.shad-artichoke.ts.net"},
		User:    "dev",
		Host:    "chroma-1.shad-artichoke.ts.net",
		Port:    "22",
		OS:      "macos",
	},
	{
		Name:    "darkmac",
		Aliases: []string{"darkmac", "darkmac.shad-artichoke.ts.net"},
		User:    "tim",
		Host:    "darkmac.shad-artichoke.ts.net",
		Port:    "22",
		OS:      "macos",
	},
	{
		Name:    "rover",
		Aliases: []string{"rover", "rover-1", "rover-1.shad-artichoke.ts.net"},
		User:    "tim",
		Host:    "rover-1.shad-artichoke.ts.net",
		Port:    "22",
		OS:      "linux",
	},
	{
		Name:                "legion",
		Aliases:             []string{"legion", "legion.shad-artichoke.ts.net"},
		User:                "timca",
		Host:                "legion.shad-artichoke.ts.net",
		Port:                "2223",
		OS:                  "windows",
		PreferWSLPowerShell: true,
	},
}

var (
	isWSLFunc       = isWSL
	execCommandFunc = exec.Command
	dialSSHFunc     = DialSSH
	runSSHFunc      = RunSSHCommand
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
	client, err := dialSSHFunc(node.Host, port, user, opts.Password)
	if err != nil {
		return "", fmt.Errorf("ssh dial %s@%s:%s failed: %w", user, node.Host, port, err)
	}
	defer client.Close()

	out, err := runSSHFunc(client, command)
	if err != nil {
		return out, fmt.Errorf("ssh command failed on %s: %w", node.Name, err)
	}
	return out, nil
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
		psCommand = "wsl.exe -e bash -lc '" + strings.ReplaceAll(command, "'", "''") + "'"
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

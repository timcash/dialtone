package ssh

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type MeshNode struct {
	Name                string   `json:"name"`
	Aliases             []string `json:"aliases"`
	User                string   `json:"user"`
	Host                string   `json:"host"`
	HostCandidates      []string `json:"host_candidates"`
	RoutePreference     []string `json:"route_preference"`
	Port                string   `json:"port"`
	OS                  string   `json:"os"`
	PreferWSLPowerShell bool     `json:"prefer_wsl_powershell"`
	RepoCandidates      []string `json:"repo_candidates"`
}

type CommandOptions struct {
	User     string
	Port     string
	Password string
}

var (
	defaultMeshNodes []MeshNode
	meshOnce         sync.Once
)

func loadMeshConfig() {
	meshOnce.Do(func() {
		// Try to find repo root to locate env/mesh.json
		cwd, _ := os.Getwd()
		cur := cwd
		repoRoot := ""
		for {
			if _, err := os.Stat(filepath.Join(cur, "dialtone.sh")); err == nil {
				repoRoot = cur
				break
			}
			parent := filepath.Dir(cur)
			if parent == cur {
				break
			}
			cur = parent
		}
		if repoRoot == "" {
			return
		}
		configPath := os.Getenv("DIALTONE_MESH_CONFIG")
		if configPath == "" {
			configPath = filepath.Join(repoRoot, "env", "dialtone.json")
		}
		data, err := os.ReadFile(configPath)
		if err != nil {
			return
		}
		// Support both direct array (legacy) or nested inside dialtone.json
		if len(data) > 0 && data[0] == '[' {
			_ = json.Unmarshal(data, &defaultMeshNodes)
		} else {
			var config struct {
				MeshNodes []MeshNode `json:"mesh_nodes"`
			}
			if err := json.Unmarshal(data, &config); err == nil {
				defaultMeshNodes = config.MeshNodes
			}
		}
	})
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
	loadMeshConfig()
	out := make([]MeshNode, len(defaultMeshNodes))
	copy(out, defaultMeshNodes)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func ResolveMeshNode(target string) (MeshNode, error) {
	loadMeshConfig()
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
	if port == "" {
		port = "22"
	}
	host, client, err := dialMeshNode(node, user, port, opts.Password)
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
	host, client, err := dialMeshNode(node, user, port, opts.Password)
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
		return uploadViaLocalPowerShell(localPath, remotePath)
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
	host, client, err := dialMeshNode(node, user, port, opts.Password)
	if err != nil {
		return fmt.Errorf("ssh dial %s@%s:%s failed: %w", user, host, port, err)
	}
	defer client.Close()
	if err := UploadFile(client, localPath, remotePath); err != nil {
		return fmt.Errorf("upload failed on %s: %w", node.Name, err)
	}
	return nil
}

func uploadViaLocalPowerShell(localPath, remotePath string) error {
	localPath = strings.TrimSpace(localPath)
	remotePath = strings.TrimSpace(remotePath)
	if localPath == "" || remotePath == "" {
		return fmt.Errorf("local and remote paths are required")
	}
	src := toWindowsPath(localPath)
	dst := strings.ReplaceAll(remotePath, "/", "\\")
	ps := fmt.Sprintf(`$src=%s; $dst=%s; $dir=Split-Path -Parent $dst; if($dir){ New-Item -ItemType Directory -Path $dir -Force | Out-Null }; Copy-Item -LiteralPath $src -Destination $dst -Force`, psSingleQuoted(src), psSingleQuoted(dst))
	if _, err := runPowerShellCommand(ps); err != nil {
		return err
	}
	return nil
}

func toWindowsPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		return strings.ReplaceAll(path, "/", "\\")
	}
	if out, err := execCommandFunc("wslpath", "-w", path).Output(); err == nil {
		if win := strings.TrimSpace(string(out)); win != "" {
			return win
		}
	}
	return strings.ReplaceAll(path, "/", "\\")
}

func psSingleQuoted(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}

func dialMeshNode(node MeshNode, user, port, password string) (string, *ssh.Client, error) {
	candidates := prioritizedMeshHostsForNode(node, resolveMeshCandidates(node))
	if len(candidates) == 0 {
		return "", nil, fmt.Errorf("mesh node %s has no host candidates", node.Name)
	}
	if strings.TrimSpace(port) == "" {
		port = "22"
	}

	attempted := map[string]struct{}{}
	errors := make([]string, 0, len(candidates))
	for _, host := range candidates {
		if !canReachHostFn(host, port, 450*time.Millisecond) {
			continue
		}
		attempted[host] = struct{}{}
		client, err := dialMeshClient(host, port, user, password)
		if err == nil {
			return host, client, nil
		}
		errors = append(errors, fmt.Sprintf("%s: %v", host, err))
	}
	for _, host := range candidates {
		if _, ok := attempted[host]; ok {
			continue
		}
		client, err := dialMeshClient(host, port, user, password)
		if err == nil {
			return host, client, nil
		}
		errors = append(errors, fmt.Sprintf("%s: %v", host, err))
	}
	if len(attempted) > 0 {
		return "", nil, fmt.Errorf("all reachable host attempts failed (%s)", strings.Join(errors, "; "))
	}
	return "", nil, fmt.Errorf("all host attempts failed (%s)", strings.Join(errors, "; "))
}

func resolvePreferredHost(node MeshNode, port string) string {
	candidates := prioritizedMeshHostsForNode(node, resolveMeshCandidates(node))
	if len(candidates) == 0 {
		return strings.TrimSpace(node.Host)
	}
	for _, h := range candidates {
		if canReachHostFn(h, port, 450*time.Millisecond) {
			return h
		}
	}
	return candidates[0]
}

func PreferredHost(node MeshNode, port string) string {
	port = strings.TrimSpace(port)
	if port == "" {
		port = strings.TrimSpace(node.Port)
	}
	if port == "" {
		port = "22"
	}
	return resolvePreferredHost(node, port)
}

func RouteHost(node MeshNode, route string, port string) string {
	port = strings.TrimSpace(port)
	if port == "" {
		port = strings.TrimSpace(node.Port)
	}
	if port == "" {
		port = "22"
	}
	route = normalizeRouteCategory(route)
	if route == "" {
		return ""
	}
	candidates := prioritizedRouteHostsForNode(node, route, resolveMeshCandidates(node))
	if len(candidates) == 0 {
		return ""
	}
	for _, h := range candidates {
		if canReachHostFn(h, port, 450*time.Millisecond) {
			return h
		}
	}
	return candidates[0]
}

func DialMeshNodeViaRoute(target string, route string, opts CommandOptions) (*ssh.Client, MeshNode, string, string, error) {
	node, err := ResolveMeshNode(target)
	if err != nil {
		return nil, MeshNode{}, "", "", err
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
	host, client, err := dialMeshNodeViaRoute(node, normalizeRouteCategory(route), user, port, opts.Password)
	if err != nil {
		return nil, MeshNode{}, "", "", fmt.Errorf("ssh dial %s@%s:%s failed: %w", user, host, port, err)
	}
	return client, node, host, port, nil
}

func resolveMeshCandidates(node MeshNode) []string {
	seen := map[string]struct{}{}
	candidates := make([]string, 0, len(node.HostCandidates)+1)
	for _, h := range node.HostCandidates {
		h = strings.TrimSpace(h)
		h = strings.TrimSuffix(h, ".")
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
		h := strings.TrimSuffix(strings.TrimSpace(node.Host), ".")
		if _, ok := seen[h]; !ok {
			candidates = append(candidates, h)
		}
	}
	return candidates
}

func prioritizedMeshHosts(candidates []string) []string {
	order := []int{
		meshHostPriorityLinkLocal,
		meshHostPriorityPrivate,
		meshHostPriorityTailnet,
		meshHostPriorityOther,
	}
	return prioritizedMeshHostsByCategory(candidates, order, meshRouteCategoryNormalize)
}

func prioritizedMeshHostsForNode(node MeshNode, candidates []string) []string {
	if len(node.RoutePreference) == 0 {
		return prioritizedMeshHosts(candidates)
	}
	order := resolveRoutePreferenceOrder(node.RoutePreference)
	return prioritizedMeshHostsByCategory(candidates, order, meshRouteCategoryNormalize)
}

func prioritizedRouteHostsForNode(node MeshNode, route string, candidates []string) []string {
	routePriority := routeCategoryPriority(route)
	filtered := make([]string, 0, len(candidates))
	for _, raw := range candidates {
		h := strings.TrimSpace(strings.TrimSuffix(raw, "."))
		if h == "" {
			continue
		}
		if meshRouteCategoryNormalize(h) != routePriority {
			continue
		}
		filtered = append(filtered, h)
	}
	if len(filtered) == 0 {
		return nil
	}
	return prioritizedMeshHostsByCategory(filtered, []int{routePriority}, meshRouteCategoryNormalize)
}

func prioritizedMeshHostsByCategory(candidates []string, categoryOrder []int, categoryNormalizer func(string) int) []string {
	if len(candidates) == 0 {
		return nil
	}
	buckets := map[int][]string{}
	for _, raw := range candidates {
		h := strings.TrimSpace(strings.TrimSuffix(raw, "."))
		if h == "" {
			continue
		}
		priority := categoryNormalizer(h)
		buckets[priority] = append(buckets[priority], h)
	}
	out := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, p := range categoryOrder {
		for _, h := range buckets[p] {
			if _, ok := seen[h]; ok {
				continue
			}
			seen[h] = struct{}{}
			out = append(out, h)
		}
	}
	return out
}

const (
	meshHostPriorityLinkLocal = 0
	meshHostPriorityPrivate   = 1
	meshHostPriorityTailnet   = 2
	meshHostPriorityOther     = 3
)

const (
	meshRouteLinkLocal = "link-local"
	meshRoutePrivate   = "private"
	meshRouteTailnet   = "tailscale"
	meshRouteOther     = "other"
)

var defaultMeshRouteOrder = []string{
	meshRouteTailnet,
	meshRoutePrivate,
	meshRouteLinkLocal,
	meshRouteOther,
}

func resolveRoutePreferenceOrder(preference []string) []int {
	seen := map[string]struct{}{}
	categoryOrder := make([]string, 0, len(preference))
	for _, raw := range preference {
		cat := normalizeRouteCategory(raw)
		if cat == "" {
			continue
		}
		if _, ok := seen[cat]; ok {
			continue
		}
		seen[cat] = struct{}{}
		categoryOrder = append(categoryOrder, cat)
	}
	if len(categoryOrder) == 0 {
		return []int{
			meshHostPriorityTailnet,
			meshHostPriorityPrivate,
			meshHostPriorityLinkLocal,
			meshHostPriorityOther,
		}
	}
	for _, cat := range defaultMeshRouteOrder {
		if _, ok := seen[cat]; ok {
			continue
		}
		categoryOrder = append(categoryOrder, cat)
		seen[cat] = struct{}{}
	}
	priorities := make([]int, 0, len(categoryOrder))
	for _, cat := range categoryOrder {
		priorities = append(priorities, routeCategoryPriority(cat))
	}
	return priorities
}

func normalizeRouteCategory(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case meshRouteLinkLocal:
		return meshRouteLinkLocal
	case "link", "linklocal", "link_local", "ll":
		return meshRouteLinkLocal
	case meshRoutePrivate:
		return meshRoutePrivate
	case "lan", "wlan", "private-ipv4", "privateipv4", "private_ip4", "private_ipv4":
		return meshRoutePrivate
	case meshRouteTailnet:
		return meshRouteTailnet
	case "ts.net", "tsnet", "ts", "tail":
		return meshRouteTailnet
	case "other":
		return meshRouteOther
	}
	return ""
}

func routeCategoryPriority(cat string) int {
	switch normalizeRouteCategory(cat) {
	case meshRouteLinkLocal:
		return meshHostPriorityLinkLocal
	case meshRoutePrivate:
		return meshHostPriorityPrivate
	case meshRouteTailnet:
		return meshHostPriorityTailnet
	default:
		return meshHostPriorityOther
	}
}

func meshRouteCategoryNormalize(raw string) int {
	switch {
	case isLinkLocalIPv4(raw):
		return meshHostPriorityLinkLocal
	case isPrivateIPv4(raw):
		return meshHostPriorityPrivate
	case isTailnetHost(raw):
		return meshHostPriorityTailnet
	default:
		return meshHostPriorityOther
	}
}

func meshHostPriority(host string) int {
	host = strings.TrimSpace(host)
	if host == "" {
		return meshHostPriorityOther
	}
	if isLinkLocalIPv4(host) {
		return meshHostPriorityLinkLocal
	}
	if isPrivateIPv4(host) {
		return meshHostPriorityPrivate
	}
	if isTailnetHost(host) {
		return meshHostPriorityTailnet
	}
	return meshHostPriorityOther
}

func isPrivateIPv4(raw string) bool {
	ip := net.ParseIP(raw)
	if ip == nil {
		return false
	}
	ip = ip.To4()
	if ip == nil {
		return false
	}
	first, second := int(ip[0]), int(ip[1])
	switch {
	case first == 10:
		return true
	case first == 172 && second >= 16 && second <= 31:
		return true
	case first == 192 && second == 168:
		return true
	case first == 100 && second >= 64 && second <= 127:
		return true
	default:
		return false
	}
}

func isLinkLocalIPv4(raw string) bool {
	ip := net.ParseIP(raw)
	if ip == nil {
		return false
	}
	ip = ip.To4()
	if ip == nil {
		return false
	}
	return int(ip[0]) == 169 && int(ip[1]) == 254
}

func isPrivateIPv4Address(raw string) bool {
	ip := net.ParseIP(raw)
	if ip == nil {
		return false
	}
	ip = ip.To4()
	if ip == nil {
		return false
	}
	return isPrivateIPv4AddressParts(int(ip[0]), int(ip[1]))
}

func isPrivateIPv4AddressParts(first, second int) bool {
	switch {
	case first == 10:
		return true
	case first == 172 && second >= 16 && second <= 31:
		return true
	case first == 192 && second == 168:
		return true
	case first == 100 && second >= 64 && second <= 127:
		return true
	default:
		return false
	}
}

func isTailnetHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	return strings.HasSuffix(h, ".ts.net")
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

func dialMeshNodeViaRoute(node MeshNode, route, user, port, password string) (string, *ssh.Client, error) {
	if route == "" {
		return "", nil, fmt.Errorf("route is required")
	}
	candidates := prioritizedRouteHostsForNode(node, route, resolveMeshCandidates(node))
	if len(candidates) == 0 {
		return "", nil, fmt.Errorf("mesh node %s has no %s host candidates", node.Name, route)
	}

	attempted := map[string]struct{}{}
	errors := make([]string, 0, len(candidates))
	for _, host := range candidates {
		if !canReachHostFn(host, port, 450*time.Millisecond) {
			continue
		}
		attempted[host] = struct{}{}
		client, err := dialMeshClient(host, port, user, password)
		if err == nil {
			return host, client, nil
		}
		errors = append(errors, fmt.Sprintf("%s: %v", host, err))
	}
	for _, host := range candidates {
		if _, ok := attempted[host]; ok {
			continue
		}
		client, err := dialMeshClient(host, port, user, password)
		if err == nil {
			return host, client, nil
		}
		errors = append(errors, fmt.Sprintf("%s: %v", host, err))
	}
	if len(attempted) > 0 {
		return candidates[0], nil, fmt.Errorf("all reachable %s host attempts failed (%s)", route, strings.Join(errors, "; "))
	}
	return candidates[0], nil, fmt.Errorf("all %s host attempts failed (%s)", route, strings.Join(errors, "; "))
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

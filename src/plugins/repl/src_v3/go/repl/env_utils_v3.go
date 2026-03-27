package repl

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

func resolveRoots() (repoRoot, srcRoot string, err error) {
	cwd, e := os.Getwd()
	if e != nil {
		return "", "", e
	}
	abs, _ := filepath.Abs(cwd)
	if filepath.Base(abs) == "src" {
		return filepath.Dir(abs), abs, nil
	}
	repoGuess := abs
	if _, statErr := os.Stat(filepath.Join(repoGuess, "src")); statErr != nil {
		return "", "", fmt.Errorf("unable to resolve repo root from %s", abs)
	}
	return repoGuess, filepath.Join(repoGuess, "src"), nil
}

func resolveConfigPath() (string, error) {
	raw := strings.TrimSpace(os.Getenv("DIALTONE_MESH_CONFIG"))
	if raw != "" {
		return raw, nil
	}
	if resolved := strings.TrimSpace(configv1.ResolveEnvFilePath("")); resolved != "" {
		return resolved, nil
	}
	repoRoot, _, err := resolveRoots()
	if err != nil {
		return "", err
	}
	return filepath.Join(repoRoot, "env", "dialtone.json"), nil
}

func loadConfig(path string) (dialtoneConfig, error) {
	var cfg dialtoneConfig
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func saveConfig(path string, cfg dialtoneConfig) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func parseCSV(raw string) []string {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func resolveGoBin() (string, error) {
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin != "" {
		return goBin, nil
	}
	path, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("go binary not found (DIALTONE_GO_BIN unset and go not in PATH)")
	}
	return path, nil
}

func endpointReachable(natsURL string, timeout time.Duration) bool {
	u, err := url.Parse(strings.TrimSpace(natsURL))
	if err != nil {
		return false
	}
	host := strings.TrimSpace(u.Hostname())
	port := strings.TrimSpace(u.Port())
	if port == "" {
		port = "4222"
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func listenURLFromClientURL(clientURL string) string {
	u, err := url.Parse(strings.TrimSpace(clientURL))
	if err != nil {
		return "nats://0.0.0.0:4222"
	}
	port := strings.TrimSpace(u.Port())
	if port == "" {
		port = "4222"
	}
	return "nats://0.0.0.0:" + port
}

func isLocalNATSEndpoint(natsURL string) bool {
	u, err := url.Parse(strings.TrimSpace(natsURL))
	if err != nil {
		return false
	}
	host := strings.TrimSpace(strings.ToLower(u.Hostname()))
	switch host {
	case "", "127.0.0.1", "localhost", "0.0.0.0", "::1", "::":
		return true
	default:
		return false
	}
}

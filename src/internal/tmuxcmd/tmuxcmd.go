package tmuxcmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Binary() string {
	if value := strings.TrimSpace(os.Getenv("DIALTONE_TMUX_BIN")); value != "" {
		return value
	}
	return "tmux"
}

func ConfigPath(repoRoot string) string {
	root := strings.TrimSpace(repoRoot)
	if root == "" {
		return ""
	}
	path := filepath.Join(root, ".tmux.conf")
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ""
	}
	return path
}

func Args(repoRoot string, args ...string) []string {
	out := make([]string, 0, len(args)+2)
	if configPath := ConfigPath(repoRoot); configPath != "" {
		out = append(out, "-f", configPath)
	}
	out = append(out, args...)
	return out
}

func Command(repoRoot string, args ...string) *exec.Cmd {
	return exec.Command(Binary(), Args(repoRoot, args...)...)
}

func ShellCommand(repoRoot string, args ...string) string {
	parts := append([]string{Binary()}, Args(repoRoot, args...)...)
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		quoted = append(quoted, shellQuote(part))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

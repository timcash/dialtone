package repl

import (
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func EnsureLeaderRunning(clientNATSURL, room string) error {
	clientNATSURL = strings.TrimSpace(clientNATSURL)
	if clientNATSURL == "" {
		clientNATSURL = defaultNATSURL
	}
	if strings.TrimSpace(room) == "" {
		room = defaultRoom
	}
	if _, err := leaderHealth(clientNATSURL, 1200*time.Millisecond); err == nil {
		return nil
	}
	if !isLocalNATSEndpoint(clientNATSURL) {
		return fmt.Errorf("repl v3 target nats endpoint is not reachable: %s", clientNATSURL)
	}
	repoRoot, srcRoot, err := resolveRoots()
	if err != nil {
		return err
	}
	listenURL := listenURLFromClientURL(clientNATSURL)
	cmd, err := leaderAutostartCommand(repoRoot, srcRoot, listenURL, room)
	if err != nil {
		return err
	}
	cmd.Env = append(os.Environ(),
		"DIALTONE_REPO_ROOT="+repoRoot,
		"DIALTONE_SRC_ROOT="+srcRoot,
	)
	dialtoneHome := configv1.DefaultDialtoneHome()
	if err := os.MkdirAll(filepath.Join(dialtoneHome, "repl-v3"), 0o755); err != nil {
		return err
	}
	stdoutPath := filepath.Join(dialtoneHome, "repl-v3", "leader-autostart.out.log")
	stderrPath := filepath.Join(dialtoneHome, "repl-v3", "leader-autostart.err.log")
	stdout, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer stdout.Close()
	stderr, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer stderr.Close()
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = nil
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := leaderHealth(clientNATSURL, 1200*time.Millisecond); err == nil {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("repl v3 leader did not start nats endpoint at %s", clientNATSURL)
}

func leaderAutostartCommand(repoRoot, srcRoot, listenURL, room string) (*exec.Cmd, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return nil, err
	}
	args := []string{
		"repl", "src_v3", "leader",
		"--embedded-nats",
		"--nats-url", listenURL,
		"--room", room,
		"--hostname", "DIALTONE-SERVER",
	}
	if len(os.Args) > 1 && strings.HasPrefix(strings.TrimSpace(os.Args[1]), "src_v") {
		args = []string{
			"src_v3", "leader",
			"--embedded-nats",
			"--nats-url", listenURL,
			"--room", room,
			"--hostname", "DIALTONE-SERVER",
		}
	}
	cmd := exec.Command(exe, args...)
	cmd.Dir = srcRoot
	return cmd, nil
}

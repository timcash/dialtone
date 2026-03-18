package repl

import (
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
	if endpointReachable(clientNATSURL, 700*time.Millisecond) {
		return nil
	}
	if !isLocalNATSEndpoint(clientNATSURL) {
		return fmt.Errorf("repl v3 target nats endpoint is not reachable: %s", clientNATSURL)
	}
	repoRoot, srcRoot, err := resolveRoots()
	if err != nil {
		return err
	}
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	listenURL := listenURLFromClientURL(clientNATSURL)
	cmd := exec.Command(goBin, "run", "./plugins/repl/scaffold/main.go", "src_v3", "leader",
		"--embedded-nats",
		"--nats-url", listenURL,
		"--room", room,
		"--hostname", "DIALTONE-SERVER",
	)
	cmd.Dir = srcRoot
	cmd.Env = append(os.Environ(),
		"DIALTONE_REPO_ROOT="+repoRoot,
		"DIALTONE_SRC_ROOT="+srcRoot,
	)
	if err := os.MkdirAll(filepath.Join(repoRoot, ".dialtone", "repl-v3"), 0o755); err != nil {
		return err
	}
	stdoutPath := filepath.Join(repoRoot, ".dialtone", "repl-v3", "leader-autostart.out.log")
	stderrPath := filepath.Join(repoRoot, ".dialtone", "repl-v3", "leader-autostart.err.log")
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
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if endpointReachable(clientNATSURL, 600*time.Millisecond) {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("repl v3 leader did not start nats endpoint at %s", clientNATSURL)
}

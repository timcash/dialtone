package app

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"dialtone/cli/src/core"
	"dialtone/cli/src/core/logger"

	"github.com/nats-io/nats.go"
)

// RunOpencodeServer starts the opencode AI assistant server with a TTY bridge
func RunOpencodeServer(port int) {
	logger.LogInfo("Starting opencode terminal bridge (bash)...")

	// Create a shim script for 'opencode' if it doesn't exist or ensure it's correct
	// Create a shim script for 'opencode' if it doesn't exist or ensure it's correct
	shimDir := "/home/tim/dialtone_deploy"
	if _, err := os.Stat(shimDir); os.IsNotExist(err) {
		shimDir = os.TempDir()
	}
	shimPath := filepath.Join(shimDir, "opencode")

	// Create the shim script
	// Note: In dev/test, 'dialtone' command might not be in path or might need to be 'go run ...'
	// For now we assume this is mostly for the robot, but safe to write to /tmp locally
	shimContent := fmt.Sprintf("#!/bin/bash\n/home/tim/dialtone ai chat \"$@\"")
	if shimDir == os.TempDir() {
		shimContent = "#!/bin/bash\necho \"[MOCK OPENCODE] $1\""
	}

	shimCmd := fmt.Sprintf("echo '%s' > %s && chmod +x %s", shimContent, shimPath, shimPath)
	_ = exec.Command("sh", "-c", shimCmd).Run()

	// We bridge to bash so the user has a full CLI, but opencode can be called from it
	// We also ensure common paths and dialtone itself are available
	bashCmd := fmt.Sprintf("export PATH=$PATH:%s; exec /bin/bash -i", shimDir)
	cmd := exec.Command("/bin/bash", "-c", bashCmd)

	// Create pipes BEFORE starting
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		logger.LogFatal("Failed to start bridge shell: %v", err)
	}

	// Start NATS Bridge with existing pipes
	go bridgeOpencodeToNATS(cmd, stdin, stdout, stderr)
	logger.LogInfo("Terminal bridge started (PID: %d).", cmd.Process.Pid)

	// Keep the function alive until the shell exits
	cmd.Wait()
}

func bridgeOpencodeToNATS(cmd *exec.Cmd, stdin io.WriteCloser, stdout io.ReadCloser, stderr io.ReadCloser) {
	// Try standard port first, then dialtone internal port
	ports := []int{4222, 14222}
	var nc *core.NatsClient
	var err error

	for _, p := range ports {
		url := fmt.Sprintf("nats://127.0.0.1:%d", p)
		logger.LogInfo("AI Bridge: Attempting NATS connection at %s...", url)
		nc, err = core.NewNatsClient(url)
		if err == nil {
			logger.LogInfo("AI Bridge: NATS Connected on port %d.", p)
			break
		}
	}

	if nc == nil {
		logger.LogInfo("AI Bridge: Failed to connect to NATS on all tried ports.")
		return
	}
	defer nc.Close()

	// Stream stdout to NATS
	go func() {
		buf := make([]byte, 2048)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				logger.LogInfo("AI Bridge: STDOUT %d bytes", n)
				nc.Publish("ai.opencode.output", buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	// Stream stderr to NATS
	go func() {
		buf := make([]byte, 2048)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				logger.LogInfo("AI Bridge: STDERR %d bytes", n)
				nc.Publish("ai.opencode.output", buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	// Stream NATS to stdin
	nc.Subscribe("ai.opencode.input", func(m *nats.Msg) {
		logger.LogInfo("AI Bridge: Received INPUT via NATS: %s", string(m.Data))
		// Manual echo for terminal visibility (required for diagnostic loopback)
		nc.Publish("ai.opencode.output", []byte("\x1b[32m[NATS-ECHO] \x1b[0m"))
		nc.Publish("ai.opencode.output", m.Data)
		nc.Publish("ai.opencode.output", []byte("\r\n"))

		stdin.Write(m.Data)
		stdin.Write([]byte("\n"))
	})

	cmd.Wait()
}

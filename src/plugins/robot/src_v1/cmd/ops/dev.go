package ops

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	test_v2 "dialtone/cli/src/libs/test_v2"
)

func Dev() error {
	fmt.Printf(">> [Robot] Dev: src_v1\n")
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	
	// 1. Ensure UI dev server is running in the background
	uiDir := filepath.Join(cwd, "src", "plugins", "robot", "src_v1", "ui")
	devCmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "dev", "--host", "127.0.0.1", "--port", "3000")
	devCmd.Stdout = os.Stdout
	devCmd.Stderr = os.Stderr
	if err := devCmd.Start(); err != nil {
		return fmt.Errorf("failed to start dev server: %w", err)
	}
	defer devCmd.Process.Kill()

	// 2. Wait for dev server
	if err := test_v2.WaitForPort(3000, 15*time.Second); err != nil {
		return fmt.Errorf("dev server failed to start: %w", err)
	}

	// 3. Launch or attach to Chrome dev session
	chromeCmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "chrome", "new", "http://127.0.0.1:3000", "--role", "dev", "--reuse-existing", "--gpu")
	chromeCmd.Stdout = os.Stdout
	chromeCmd.Stderr = os.Stderr
	if err := chromeCmd.Run(); err != nil {
		return fmt.Errorf("failed to launch chrome: %w", err)
	}

	// 4. Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	return nil
}

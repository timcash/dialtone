package cli

import (
	"dialtone/cli/src/libs/dialtest"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

func RunDev(versionDir string) error {
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "dag", versionDir)
	uiDir := filepath.Join(cwd, "src", "plugins", "dag", versionDir, "ui")
	devLogPath := filepath.Join(pluginDir, "dev.log")
	devPort := 3000
	devURL := fmt.Sprintf("http://127.0.0.1:%d", devPort)

	logFile, err := os.OpenFile(devLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open dev log at %s: %w", devLogPath, err)
	}
	defer logFile.Close()

	logOut := io.MultiWriter(os.Stdout, logFile)
	logf := func(format string, args ...any) {
		fmt.Fprintf(logOut, format+"\n", args...)
	}

	logf(">> [DAG] Dev: %s", versionDir)
	logf("   [DEV] Writing logs to %s", devLogPath)

	if _, err := os.Stat(uiDir); os.IsNotExist(err) {
		return fmt.Errorf("UI directory not found: %s", uiDir)
	}

	logf("   [DEV] Running vite dev...")
	cmd := runBun(cwd, uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(devPort))
	cmd.Stdout = logOut
	cmd.Stderr = logOut
	if err := cmd.Start(); err != nil {
		return err
	}

	var (
		mu      sync.Mutex
		session *dialtest.ChromeSession
	)

	go func() {
		if err := dialtest.WaitForPort(devPort, 30*time.Second); err != nil {
			logf("   [DEV] Warning: vite server not ready on port %d: %v", devPort, err)
			return
		}

		logf("   [DEV] Vite ready at %s", devURL)
		logf("   [DEV] Launching debug browser (HEADED) with console capture...")

		s, err := dialtest.StartChromeSession(dialtest.ChromeSessionOptions{
			Headless:      false,
			Role:          "dev",
			ReuseExisting: true,
			URL:           devURL,
			LogWriter:     logOut,
			LogPrefix:     "   [BROWSER]",
		})
		if err != nil {
			logf("   [DEV] Warning: failed to attach debug browser: %v", err)
			return
		}
		mu.Lock()
		session = s
		mu.Unlock()
	}()

	err = cmd.Wait()
	if err != nil {
		logf("   [DEV] Vite process exited with error: %v", err)
	} else {
		logf("   [DEV] Vite process exited.")
	}

	mu.Lock()
	if session != nil {
		session.Close()
	}
	mu.Unlock()

	return err
}

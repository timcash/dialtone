package cli

import (
	"dialtone/cli/src/libs/dialtest"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

func RunDev(versionDir string) error {
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "dag", versionDir, "ui")
	devPort := 3000
	devURL := fmt.Sprintf("http://127.0.0.1:%d", devPort)

	fmt.Printf(">> [DAG] Dev: %s\n", versionDir)

	if _, err := os.Stat(uiDir); os.IsNotExist(err) {
		return fmt.Errorf("UI directory not found: %s", uiDir)
	}

	fmt.Println("   [DEV] Running vite dev...")
	cmd := runBun(cwd, uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(devPort))
	if err := cmd.Start(); err != nil {
		return err
	}

	var (
		mu      sync.Mutex
		session *dialtest.ChromeSession
	)

	go func() {
		if err := dialtest.WaitForPort(devPort, 30*time.Second); err != nil {
			fmt.Printf("   [DEV] Warning: vite server not ready on port %d: %v\n", devPort, err)
			return
		}

		fmt.Printf("   [DEV] Vite ready at %s\n", devURL)
		fmt.Println("   [DEV] Launching debug browser (HEADED) with console capture...")

		s, err := dialtest.StartChromeSession(dialtest.ChromeSessionOptions{
			Headless:      false,
			Role:          "dev",
			ReuseExisting: true,
			URL:           devURL,
			LogWriter:     os.Stdout,
			LogPrefix:     "   [BROWSER]",
		})
		if err != nil {
			fmt.Printf("   [DEV] Warning: failed to attach debug browser: %v\n", err)
			return
		}
		mu.Lock()
		session = s
		mu.Unlock()
	}()

	err := cmd.Wait()

	mu.Lock()
	if session != nil {
		session.Close()
	}
	mu.Unlock()

	return err
}

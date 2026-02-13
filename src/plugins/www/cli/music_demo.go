package cli

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"syscall"
	"time"
)

func handleMusicDemo(webDir string) {
	logInfo("Setting up Music Demo Environment...")

	// 1. Port cleanup
	logInfo("Cleaning up port 5173...")
	_ = exec.Command("fuser", "-k", "5173/tcp").Run()
	time.Sleep(1500 * time.Millisecond)

	// 2. Kill existing Dialtone Chrome instances
	logInfo("Cleaning up Chrome processes...")
	_ = getDialtoneCmd("chrome", "kill", "all").Run()

	// 3. Start WWW Dev Server (Background)
	logInfo("Starting WWW Dev Server on 0.0.0.0...")
	// Bind to 0.0.0.0 so it's accessible externally if needed
	devCmd := exec.Command("npm", "run", "dev", "--", "--host", "0.0.0.0")
	devCmd.Dir = webDir

	stdout, err := devCmd.StdoutPipe()
	if err != nil {
		logFatal("Failed to attach to dev server stdout: %v", err)
	}
	stderr, err := devCmd.StderrPipe()
	if err != nil {
		logFatal("Failed to attach to dev server stderr: %v", err)
	}

	if err := devCmd.Start(); err != nil {
		logFatal("Failed to start dev server: %v", err)
	}

	// 4. Wait for dev server to be ready + detect actual port
	logInfo("Waiting for Dev Server...")
	port := 5173
	portCh := make(chan int, 1)

	go func() {
		reader := io.MultiReader(stdout, stderr)
		scanner := bufio.NewScanner(reader)
		// Vite output might show 127.0.0.1 or 0.0.0.0 or the local network IP
		re := regexp.MustCompile(`http://(?:127\.0\.0\.1|0\.0\.0\.0|localhost|[\d\.]+):(\d+)/`)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
			if match := re.FindStringSubmatch(line); len(match) == 2 {
				if p, err := strconv.Atoi(match[1]); err == nil {
					select {
					case portCh <- p:
					default:
					}
				}
			}
		}
	}()

	select {
	case detected := <-portCh:
		port = detected
	case <-time.After(10 * time.Second):
		logInfo("Dev server port not detected yet; falling back to %d", port)
	}

	ready := false
	for i := 0; i < 30; i++ {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d", port))
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(1 * time.Second)
	}

	if !ready {
		logFatal("Dev server failed to start within 30 seconds")
	}

	// 5. Launch GPU-enabled Chrome on Music section
	logInfo("Launching GPU-enabled Chrome...")
	baseURL := fmt.Sprintf("http://127.0.0.1:%d/#s-music", port)
	chromeCmd := getDialtoneCmd("chrome", "new", baseURL, "--gpu")
	chromeCmd.Stdout = os.Stdout
	chromeCmd.Stderr = os.Stderr
	if err := chromeCmd.Run(); err != nil {
		logFatal("Failed to launch Chrome: %v", err)
	}

	logInfo("Music Demo Environment is LIVE!")
	logInfo("Dev Server: %s", baseURL)
	logInfo("External Access: http://<your-ip>:%d/#s-music", port)
	logInfo("Press Ctrl+C to stop...")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	logInfo("Shutting down...")
}

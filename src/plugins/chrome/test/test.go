package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"dialtone/cli/src/core/browser"
)

func main() {
	logLine("info", "Starting Advanced Chrome plugin integration test")

	allPassed := true
	runTest := func(name string, fn func() error) {
		logLine("test", name)
		if err := fn(); err != nil {
			logLine("fail", fmt.Sprintf("%s - %v", name, err))
			allPassed = false
		} else {
			logLine("pass", name)
		}
	}

	defer func() {
		logLine("info", "Chrome plugin tests completed")
		if !allPassed {
			logLine("error", "Some tests failed")
			os.Exit(1)
		}
	}()

	runTest("Chrome list shows metrics headers", TestChromeListHeaders)
	runTest("Verify Headless/Headed detection and kill", TestProcessManagement)
	runTest("Verify 'chrome new' command", TestChromeNew)

	fmt.Println()
}

func TestChromeNew() error {
	logLine("step", "Running 'dialtone chrome new'")
	output := runCmd("./dialtone.sh", "chrome", "new")

	if !strings.Contains(output, "Chrome started successfully") {
		return fmt.Errorf("expected success message in output")
	}
	if !strings.Contains(output, "WebSocket URL") || !strings.Contains(output, "ws://") {
		return fmt.Errorf("expected websocket URL in output")
	}

	// Extract PID from output to clean it up
	var pid int
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "PID") {
			fmt.Sscanf(line, "PID : %d", &pid)
			break
		}
	}

	if pid > 0 {
		logLine("step", fmt.Sprintf("Cleaning up started process %d", pid))
		runCmd("./dialtone.sh", "chrome", "kill", fmt.Sprintf("%d", pid))
	} else {
		return fmt.Errorf("failed to extract PID from 'chrome new' output")
	}

	return nil
}

func TestChromeListHeaders() error {
	logLine("step", "Listing Chrome processes")
	output := runCmd("./dialtone.sh", "chrome", "list", "--verbose")
	headers := []string{"PID", "PPID", "HEADLESS", "%CPU", "MEM(MB)", "CHILDREN", "PLATFORM", "COMMAND"}
	for _, h := range headers {
		if !strings.Contains(output, h) {
			return fmt.Errorf("expected list output to include header: %s", h)
		}
	}
	return nil
}

func TestProcessManagement() error {
	chromePath := browser.FindChromePath()
	if chromePath == "" {
		return fmt.Errorf("chrome not found")
	}

	// 1. Start Headless Instance
	logLine("step", "Starting Headless Chrome manually")
	hlessCmd := exec.Command(chromePath, "--headless", "--remote-debugging-port=9333", "about:blank")
	if err := hlessCmd.Start(); err != nil {
		return fmt.Errorf("failed to start headless chrome: %v", err)
	}
	hlessPid := hlessCmd.Process.Pid
	defer hlessCmd.Process.Kill()
	logLine("info", fmt.Sprintf("Started Headless Chrome (PID: %d)", hlessPid))

	// 2. Start Headed Instance
	// NOTE: In some environments without X11 this might fail, but let's try with dummy flags
	logLine("step", "Starting Headed Chrome manually")
	var headedPid int
	headedKilled := false
	output := runCmd("./dialtone.sh", "chrome", "new")
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "PID") {
			fmt.Sscanf(line, "PID : %d", &headedPid)
			break
		}
	}
	if headedPid == 0 {
		logLine("warn", "Failed to extract PID from 'chrome new' output (skipping headed test)")
	} else {
		logLine("info", fmt.Sprintf("Started Headed Chrome (PID: %d)", headedPid))
		defer func() {
			if !headedKilled {
				runCmd("./dialtone.sh", "chrome", "kill", fmt.Sprintf("%d", headedPid))
			}
		}()
	}

	time.Sleep(2 * time.Second) // Let them settle

	// 3. Verify they are in the list with correct HEADLESS state
	logLine("step", "Verifying instances and states in dialtone chrome list")
	output = runCmd("./dialtone.sh", "chrome", "list")

	// Check Headless
	if !strings.Contains(output, fmt.Sprintf("%d", hlessPid)) {
		return fmt.Errorf("headless PID %d not found in list", hlessPid)
	}
	lines = strings.Split(output, "\n")
	foundHless := false
	for _, line := range lines {
		if strings.Contains(line, fmt.Sprintf("%d", hlessPid)) && strings.Contains(line, "YES") {
			foundHless = true
			break
		}
	}
	if !foundHless {
		return fmt.Errorf("headless PID %d not reported as HEADLESS=YES", hlessPid)
	}

	// Check Headed
	if headedPid > 0 {
		foundHeaded := false
		for _, line := range lines {
			if strings.Contains(line, fmt.Sprintf("%d", headedPid)) && strings.Contains(line, "NO") {
				foundHeaded = true
				break
			}
		}
		if !foundHeaded {
			return fmt.Errorf("headed PID %d not reported as HEADLESS=NO", headedPid)
		}
	}

	// 4. Kill Headless via dialtone
	logLine("step", fmt.Sprintf("Killing Headless PID %d via dialtone", hlessPid))
	runCmd("./dialtone.sh", "chrome", "kill", fmt.Sprintf("%d", hlessPid))
	time.Sleep(1 * time.Second)

	logLine("step", "Checking list after headless kill")
	output = runCmd("./dialtone.sh", "chrome", "list")
	if strings.Contains(output, fmt.Sprintf("%d", hlessPid)) {
		if err := exec.Command("ps", "-p", fmt.Sprintf("%d", hlessPid)).Run(); err == nil {
			return fmt.Errorf("headless PID %d still exists after kill", hlessPid)
		}
	}

	// 5. Kill Headed via dialtone (if started)
	if headedPid > 0 {
		logLine("step", fmt.Sprintf("Killing Headed PID %d via dialtone", headedPid))
		runCmd("./dialtone.sh", "chrome", "kill", fmt.Sprintf("%d", headedPid))
		headedKilled = true
		time.Sleep(1 * time.Second)

		logLine("step", "Checking list after headed kill")
		output = runCmd("./dialtone.sh", "chrome", "list")
		if strings.Contains(output, fmt.Sprintf("%d", headedPid)) {
			if err := exec.Command("ps", "-p", fmt.Sprintf("%d", headedPid)).Run(); err == nil {
				return fmt.Errorf("headed PID %d still exists after kill", headedPid)
			}
		}
	}

	return nil
}

func runCmd(name string, args ...string) string {
	logLine("cmd", fmt.Sprintf("%s %v", name, args))
	cmd := exec.Command(name, args...)
	output, _ := cmd.CombinedOutput()
	fmt.Print(string(output))
	return string(output)
}

func logLine(level, message string) {
	fmt.Printf("[%s] %s\n", level, message)
}

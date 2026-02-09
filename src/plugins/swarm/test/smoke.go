package test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func RunSmoke(dir string) error {
	fmt.Printf(">> [SWARM] Smoke: start for %s\n", dir)

	cwd, _ := os.Getwd()
	swarmDir := filepath.Join(cwd, "src", "plugins", "swarm", dir)
	smokeFile := filepath.Join(swarmDir, "SMOKE.md")

	// 1. Start Pear dashboard
	fmt.Println(">> [SWARM] Smoke: starting pear dashboard...")
	dashCmd := exec.Command("pear", "run", ".", "dashboard")
	dashCmd.Dir = swarmDir
	if err := dashCmd.Start(); err != nil {
		return fmt.Errorf("failed to start pear dashboard: %v", err)
	}
	defer dashCmd.Process.Kill()

	// 2. Wait for dashboard port 4000
	if err := waitForPort(4000, 15*time.Second); err != nil {
		return fmt.Errorf("dashboard not ready on 4000: %v", err)
	}

	// 3. Start browser via ./dialtone.sh chrome
	fmt.Println(">> [SWARM] Smoke: connecting to chrome...")
	wsURL, err := getChromeWS()
	if err != nil {
		return fmt.Errorf("failed to get chrome websocket: %v", err)
	}

	allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	defer cancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// 4. Navigate and check sections
	sections := []string{"s-demo"}
	allErrors := []string{}

	for _, section := range sections {
		fmt.Printf(">> [SWARM] Smoke: verifying section #%s\n", section)
		var statusText string
		err = chromedp.Run(ctx,
			chromedp.Navigate("http://127.0.0.1:4000/#"+section),
			chromedp.WaitVisible("#"+section, chromedp.ByQuery),
			chromedp.Evaluate(`document.getElementById('header-subtitle').innerText`, &statusText),
		)
		if err != nil {
			allErrors = append(allErrors, fmt.Sprintf("section %s failed: %v", section, err))
			continue
		}

		fmt.Printf(">> [SWARM] Smoke: section %s status: %s\n", section, statusText)

		// 5. Update SMOKE.md for each section
		status := "✅ PASSED"
		entry := fmt.Sprintf("## Section #%s (%s) - %s\n\nSubtitle: `%s`\n\n", section, time.Now().Format(time.Kitchen), status, statusText)
		appendToFile(smokeFile, entry)
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("smoke test issues:\n%s", strings.Join(allErrors, "\n"))
	}

	fmt.Println(">> [SWARM] Smoke: complete")
	return nil
}

func getChromeWS() (string, error) {
	out, err := exec.Command("./dialtone.sh", "chrome", "list", "--json").Output()
	if err != nil {
		return "", err
	}
	// Minimal parsing of chrome list output
	// Expected format: [{"WebsocketURL": "ws://..."}]
	s := string(out)
	if strings.Contains(s, "ws://") {
		start := strings.Index(s, "ws://")
		end := strings.Index(s[start:], "\"")
		return s[start : start+end], nil
	}
	// If none found, launch one
	fmt.Println(">> [SWARM] Smoke: launching new chrome...")
	exec.Command("./dialtone.sh", "chrome", "launch", "--headless").Run()
	time.Sleep(2 * time.Second)
	return getChromeWS()
}

func appendToFile(filename, content string) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	if info, _ := f.Stat(); info.Size() == 0 {
		f.WriteString("# Smoke Test Report\n\n")
	}
	f.WriteString(content)
}

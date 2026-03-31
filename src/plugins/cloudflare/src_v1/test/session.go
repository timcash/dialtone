package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
	cloudflarev1 "dialtone/dev/plugins/cloudflare/src_v1/go"
	test_v2 "dialtone/dev/plugins/test/src_v1/go"
)

var sharedServer *exec.Cmd
var sharedBrowser *test_v2.BrowserSession

const (
	testViewportWidth  = 390
	testViewportHeight = 844
	testServerPort     = 18080
)

var cloudflareSectionIDs = map[string]string{
	"hero":   "cloudflare-hero-stage",
	"status": "cloudflare-status-table",
	"docs":   "cloudflare-docs-docs",
	"three":  "cloudflare-three-stage",
	"xterm":  "cloudflare-log-xterm",
}

func ensureSharedServer() error {
	if sharedServer != nil {
		return nil
	}

	paths, err := cloudflarev1.ResolvePaths("", "src_v1")
	if err != nil {
		return err
	}

	_ = chrome.CleanupPort(testServerPort)

	cmd, err := testDialtoneCommand(paths.Runtime.RepoRoot, "cloudflare", "src_v1", "serve", "--port", fmt.Sprintf("%d", testServerPort))
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	if err := waitForPort(fmt.Sprintf("127.0.0.1:%d", testServerPort), 12*time.Second); err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return err
	}

	sharedServer = cmd
	return nil
}

func ensureSharedBrowser(emitProofOfLife bool) (*test_v2.BrowserSession, error) {
	if err := ensureSharedServer(); err != nil {
		return nil, err
	}

	if sharedBrowser == nil {
		browserURL := fmt.Sprintf("http://127.0.0.1:%d", testServerPort)
		if remoteNode := strings.TrimSpace(test_v2.RuntimeConfigSnapshot().BrowserNode); remoteNode != "" {
			rewritten, err := test_v2.RewriteBrowserURLForRemoteNode(browserURL, remoteNode)
			if err != nil {
				return nil, fmt.Errorf("rewrite cloudflare browser url for %s: %w", remoteNode, err)
			}
			if strings.TrimSpace(rewritten) != "" {
				browserURL = rewritten
			}
		}
		fmt.Printf("[TEST] cloudflare shared browser: start session url=%s\n", browserURL)
		session, err := test_v2.StartBrowser(test_v2.BrowserOptions{
			Headless:      true,
			Role:          "test",
			ReuseExisting: false,
			URL:           browserURL,
			LogWriter:     os.Stdout,
			LogPrefix:     "[BROWSER]",
		})
		if err != nil {
			return nil, err
		}
		fmt.Printf("[TEST] cloudflare shared browser: set viewport %dx%d\n", testViewportWidth, testViewportHeight)
		if err := session.SetViewport(testViewportWidth, testViewportHeight); err != nil {
			session.Close()
			return nil, err
		}
		sharedBrowser = session
	}

	if emitProofOfLife {
		fmt.Println("[TEST] cloudflare shared browser: emit browser proof-of-life")
		if err := sharedBrowser.Evaluate(`(() => {
			const marker = "[PROOFOFLIFE] Intentional Browser Test Error"
			const script = document.createElement("script")
			script.textContent = "console.error(" + JSON.stringify(marker) + ");"
			;(document.head || document.documentElement || document.body).appendChild(script)
			script.remove()
			return true
		})()`, nil); err != nil {
			return nil, err
		}
		fmt.Println("[TEST] cloudflare shared browser: proof-of-life script executed")
	}

	return sharedBrowser, nil
}

func teardownSharedEnv() {
	if sharedBrowser != nil {
		sharedBrowser.Close()
		sharedBrowser = nil
	}
	if sharedServer != nil {
		_ = sharedServer.Process.Kill()
		_, _ = sharedServer.Process.Wait()
		sharedServer = nil
	}
	_ = chrome.CleanupPort(testServerPort)
}

func testDialtoneCommand(repoRoot string, args ...string) (*exec.Cmd, error) {
	paths, err := cloudflarev1.ResolvePaths(repoRoot, "src_v1")
	if err != nil {
		return nil, err
	}
	cmdArgs := make([]string, 0, len(args)+2)
	if envFile := strings.TrimSpace(paths.Runtime.EnvFile); envFile != "" {
		cmdArgs = append(cmdArgs, "--env", envFile)
	}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command(filepath.Join(paths.Runtime.RepoRoot, "dialtone.sh"), cmdArgs...)
	cmd.Dir = paths.Runtime.RepoRoot
	return cmd, nil
}

func testRepoRoot() (string, error) {
	paths, err := cloudflarev1.ResolvePaths("", "src_v1")
	if err != nil {
		return "", err
	}
	return paths.Runtime.RepoRoot, nil
}

func screenshotPath(name string) (string, error) {
	paths, err := cloudflarev1.ResolvePaths("", "src_v1")
	if err != nil {
		return "", err
	}
	return filepath.Join(paths.PluginVersionRoot, "screenshots", name), nil
}

func cloudflareSectionID(sectionID string) string {
	sectionID = strings.TrimSpace(strings.ToLower(sectionID))
	if mapped, ok := cloudflareSectionIDs[sectionID]; ok {
		return mapped
	}
	return sectionID
}

func navigateToSection(session *test_v2.BrowserSession, sectionID string) error {
	targetID := cloudflareSectionID(sectionID)
	if targetID == "" {
		return fmt.Errorf("cloudflare section id is required")
	}
	if err := session.Evaluate(fmt.Sprintf(`(() => {
		const target = %q;
		if (typeof window.navigateTo === "function") {
			const nav = window.navigateTo(target);
			if (nav && typeof nav.catch === "function") {
				nav.catch((err) => console.error("[TEST] cloudflare navigate failed", err));
			}
			return true;
		}
		window.location.hash = target;
		return true;
	})()`, targetID), nil); err != nil {
		return err
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var active bool
		if err := session.Evaluate(fmt.Sprintf(`(() => {
			const target = %q;
			const section = document.getElementById(target);
			if (!section) return false;
			return section.getAttribute("data-active") === "true"
				&& document.body?.getAttribute("data-active-section") === target
				&& !section.hidden;
		})()`, targetID), &active); err == nil && active {
			return nil
		}
		time.Sleep(120 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for cloudflare section %q to become active", targetID)
}

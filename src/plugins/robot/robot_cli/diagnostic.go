package robot_cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"dialtone/dev/plugins/logs/src_v1/go"
	test_v2 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/chromedp/chromedp"
)

const (
	testViewportWidth  = 1280
	testViewportHeight = 720
)

func RunDiagnostic(versionDir string) error {
	logs.Info("Running diagnostic for Robot UI on %s...", versionDir)

	hostname := os.Getenv("DIALTONE_HOSTNAME")
	robotIP := os.Getenv("ROBOT_HOST")
	
	if hostname == "" {
		hostname = robotIP
	}
	
	if hostname == "" {
		logs.Fatal("Neither DIALTONE_HOSTNAME nor ROBOT_HOST environment variable is set. Cannot run diagnostic.")
	}

	// Use IP for diagnostic if provided, to avoid DNS/MagicDNS issues during testing
	diagTarget := hostname
	diagPort := "80"
	
	// Perform connectivity checks
	// 1. Basic Ping Check (LAN IP)
	if robotIP != "" {
		logs.Info("[DIAGNOSTIC] Step 1: Pinging LAN IP %s...", robotIP)
		pingCmd := exec.Command("ping", "-c", "2", "-W", "1", robotIP)
		if err := pingCmd.Run(); err != nil {
			logs.Warn("Ping to LAN IP %s failed.", robotIP)
		} else {
			logs.Info("Ping to LAN IP %s successful.", robotIP)
		}
	}

	// 2. HTTP Health Check (LAN IP on port 8080)
	if robotIP != "" {
		lanURL := fmt.Sprintf("http://%s:8080", robotIP)
		logs.Info("[DIAGNOSTIC] Step 2: Verifying LAN Web Server on %s...", lanURL)
		if err := checkHealth(lanURL); err != nil {
			logs.Warn("LAN health check failed (port 8080): %v", err)
		} else {
			logs.Info("LAN health check PASSED (port 8080).")
		}
	}

	// 3. HTTP Health Check (Tailscale Hostname on port 80)
	tsURL := fmt.Sprintf("http://%s", hostname)
	logs.Info("[DIAGNOSTIC] Step 3: Verifying Tailscale Web Server on %s...", tsURL)
	if err := checkHealth(tsURL); err != nil {
		logs.Warn("Tailscale health check failed (port 80): %v. (MagicDNS might still be propagating)", err)
		// If we have an IP and LAN check passed, we can proceed using the LAN URL for UI tests
		if robotIP != "" {
			diagTarget = robotIP
			diagPort = "8080"
			logs.Info("[DIAGNOSTIC] Proceeding with UI tests via LAN IP: %s:8080", robotIP)
		} else {
			return fmt.Errorf("Tailscale health check failed and no LAN IP available: %w", err)
		}
	} else {
		logs.Info("Tailscale health check PASSED (port 80).")
		diagTarget = hostname
		diagPort = "80"
	}

	targetURL := fmt.Sprintf("http://%s:%s", diagTarget, diagPort)
	
	// 4. Browser-based UI Validation
	logs.Info("[DIAGNOSTIC] Step 4: Starting browser for UI validation on %s...", targetURL)
	session, err := test_v2.StartBrowser(test_v2.BrowserOptions{
		Headless:      true,
		Role:          "diagnostic",
		ReuseExisting: false,
		URL:           targetURL,
		LogWriter:     os.Stdout,
		LogPrefix:     "[DIAGNOSTIC BROWSER]",
	})
	if err != nil {
		return fmt.Errorf("failed to start browser: %w", err)
	}
	defer session.Close()

	if err := session.Run(chromedp.EmulateViewport(testViewportWidth, testViewportHeight)); err != nil {
		return fmt.Errorf("failed to emulate viewport: %w", err)
	}

	// Debug: Check page content
	var title string
	var html string
	if err := session.Run(chromedp.Tasks{
		chromedp.Title(&title),
		chromedp.OuterHTML("html", &html),
	}); err == nil {
		logs.Info("[DIAGNOSTIC] Page Title: %s", title)
		if len(html) > 500 {
			logs.Debug("[DIAGNOSTIC] HTML Sample: %s", html[:500])
		} else {
			logs.Debug("[DIAGNOSTIC] HTML: %s", html)
		}
	} else {
		logs.Warn("[DIAGNOSTIC] Failed to get page content: %v", err)
	}

	repoRoot, _ := os.Getwd()
	screenshotsDir := filepath.Join(repoRoot, "src", "plugins", "robot", versionDir, "diagnostic_screenshots")
	_ = os.MkdirAll(screenshotsDir, 0755)

	// Define diagnostic steps
	steps := []struct {
		name       string
		sectionID  string
		validation func(ctx context.Context) error
	}{
		{
			name:      "Hero Section",
			sectionID: "hero",
			validation: func(ctx context.Context) error {
				return session.RunWithContext(ctx, chromedp.Tasks{
					chromedp.WaitVisible("[aria-label='Hero Section']", chromedp.ByQuery),
					chromedp.WaitVisible("[aria-label='Hero Canvas']", chromedp.ByQuery),
				})
			},
		},
		{
			name:      "Docs Section",
			sectionID: "docs",
			validation: func(ctx context.Context) error {
				return session.RunWithContext(ctx, test_v2.NavigateToSection("docs", "Docs Section"))
			},
		},
		{
			name:      "Telemetry Table Section",
			sectionID: "table",
			validation: func(ctx context.Context) error {
				if err := session.RunWithContext(ctx, test_v2.NavigateToSection("table", "Telemetry Section")); err != nil {
					return err
				}
				if err := session.RunWithContext(ctx, test_v2.WaitForAriaLabelAttrEquals("Robot Table", "data-ready", "true", 3*time.Second)); err != nil {
					return err
				}
				var rowCount int
				// Use a shorter loop since the context itself has a timeout
				for {
					if err := session.RunWithContext(ctx, chromedp.Evaluate(`document.querySelectorAll("table[aria-label='Robot Table'] tbody tr").length`, &rowCount)); err != nil {
						return err
					}
					if rowCount > 0 {
						break
					}
					select {
					case <-ctx.Done():
						return fmt.Errorf("robot table has no rows: %w", ctx.Err())
					case <-time.After(500 * time.Millisecond):
					}
				}
				return nil
			},
		},
		{
			name:      "3D Section Telemetry & Commands",
			sectionID: "three",
			validation: func(ctx context.Context) error {
				if err := session.RunWithContext(ctx, test_v2.NavigateToSection("three", "Three Section")); err != nil {
					return err
				}
				
				// Verify HUD elements are present
				if err := session.RunWithContext(ctx, chromedp.Tasks{
					chromedp.WaitVisible("#hud-alt", chromedp.ByID),
					chromedp.WaitVisible("#hud-spd", chromedp.ByID),
					chromedp.WaitVisible("#hud-mode", chromedp.ByID),
				}); err != nil {
					return fmt.Errorf("HUD elements not visible: %w", err)
				}

				// Wait for live data (non-0.0 values or non-default text)
				logs.Info("[DIAGNOSTIC] Waiting for live telemetry in HUD...")
				var alt, spd, mode string
				for i := 0; i < 10; i++ {
					if err := session.RunWithContext(ctx, chromedp.Tasks{
						chromedp.Text("#hud-alt", &alt, chromedp.ByID),
						chromedp.Text("#hud-spd", &spd, chromedp.ByID),
						chromedp.Text("#hud-mode", &mode, chromedp.ByID),
					}); err != nil {
						return err
					}
					// In mock mode or real flight, these should change.
					// We just check they aren't empty or stuck at initial placeholders if possible.
					if alt != "" && mode != "STABILIZE" { // MODE changes to GUIDED in mock
						logs.Info("[DIAGNOSTIC] Live Telemetry detected: ALT=%s, MODE=%s", alt, mode)
						break
					}
					time.Sleep(1 * time.Second)
				}

				// Test Command Button (ARM)
				logs.Info("[DIAGNOSTIC] Testing ARM button...")
				if err := session.RunWithContext(ctx, chromedp.Click("#three-arm", chromedp.ByID)); err != nil {
					return fmt.Errorf("failed to click ARM button: %w", err)
				}
				
				// Wait for any visual feedback or mode change if applicable
				time.Sleep(1 * time.Second)
				
				return nil
			},
		},
		{
			name:      "Terminal Section",
			sectionID: "xterm",
			validation: func(ctx context.Context) error {
				return session.RunWithContext(ctx, test_v2.NavigateToSection("xterm", "Xterm Section"))
			},
		},
		{
			name:      "Video Section",
			sectionID: "video",
			validation: func(ctx context.Context) error {
				if err := session.RunWithContext(ctx, test_v2.NavigateToSection("video", "Video Section")); err != nil {
					return err
				}
				return session.RunWithContext(ctx, test_v2.WaitForAriaLabelAttrEquals("Video Section", "data-playing", "true", 4*time.Second))
			},
		},
	}

	for i, step := range steps {
		timeout := 15 * time.Second
		logs.Info("[DIAGNOSTIC] Step %d: %s (Timeout: %v)...", i+1, step.name, timeout)
		
		// Create a context with timeout for this step
		stepCtx, stepCancel := context.WithTimeout(session.Context(), timeout)
		err := step.validation(stepCtx)
		stepCancel()

		screenshotPath := filepath.Join(screenshotsDir, fmt.Sprintf("diagnostic_step_%d_%s.png", i+1, step.sectionID))

		if err != nil {
			logs.Error("Diagnostic step '%s' FAILED: %v", step.name, err)
			// Capture a screenshot of the failure state before exiting
			if shotErr := session.CaptureScreenshot(screenshotPath); shotErr != nil {
				logs.Warn("Failed to capture failure screenshot for '%s': %v", step.name, shotErr)
			} else {
				logs.Info("Failure screenshot saved: %s", screenshotPath)
			}
			return fmt.Errorf("diagnostic step '%s' failed: %w", step.name, err)
		}

		logs.Info("Diagnostic step '%s' PASSED.", step.name)

		if shotErr := session.CaptureScreenshot(screenshotPath); shotErr != nil {
			logs.Warn("Failed to capture screenshot for '%s': %v", step.name, shotErr)
		} else {
			logs.Info("Screenshot saved: %s", screenshotPath)
		}
	}

	logs.Info("Robot UI diagnostic completed successfully.")
	return nil
}

func checkHealth(url string) error {
	client := http.Client{Timeout: 5 * time.Second}
	var lastErr error
	for i := 0; i < 3; i++ {
		resp, err := client.Get(url + "/health")
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				return nil
			}
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			resp.Body.Close()
		} else {
			lastErr = err
		}
		if i < 2 {
			time.Sleep(2 * time.Second)
		}
	}
	return lastErr
}

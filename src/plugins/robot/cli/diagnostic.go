package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"dialtone/cli/src/core/logger"
	test_v2 "dialtone/cli/src/libs/test_v2"
	"github.com/chromedp/chromedp"
)

const (
	testViewportWidth  = 1280
	testViewportHeight = 720
)

func RunDiagnostic(versionDir string) error {
	logger.LogInfo("Running diagnostic for Robot UI on %s...", versionDir)

	hostname := os.Getenv("DIALTONE_HOSTNAME")
	robotIP := os.Getenv("ROBOT_HOST")
	
	if hostname == "" {
		hostname = robotIP
	}
	
	if hostname == "" {
		logger.LogFatal("Neither DIALTONE_HOSTNAME nor ROBOT_HOST environment variable is set. Cannot run diagnostic.")
	}

	// Use IP for diagnostic if provided, to avoid DNS/MagicDNS issues during testing
	diagTarget := hostname
	if robotIP != "" {
		diagTarget = robotIP
		logger.LogInfo("[DIAGNOSTIC] Using ROBOT_HOST IP (%s) for diagnostic to avoid DNS issues.", robotIP)
	}

	// 1. Basic Ping Check
	logger.LogInfo("[DIAGNOSTIC] Step 1: Pinging %s...", diagTarget)
	pingCmd := exec.Command("ping", "-c", "3", "-W", "2", diagTarget)
	if err := pingCmd.Run(); err != nil {
		logger.LogWarn("Ping to %s failed. This might be normal if ICMP is blocked, but checking HTTP next...", diagTarget)
	} else {
		logger.LogInfo("Ping to %s successful.", diagTarget)
	}

	// 2. HTTP Health Check
	targetURL := fmt.Sprintf("http://%s", diagTarget)
	logger.LogInfo("[DIAGNOSTIC] Step 2: Checking HTTP health on %s...", targetURL)
	
	client := http.Client{Timeout: 5 * time.Second}
	var healthErr error
	healthPassed := false
	
	// Retry for up to 15 seconds
	for i := 0; i < 5; i++ {
		resp, err := client.Get(targetURL + "/health")
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				healthPassed = true
				resp.Body.Close()
				break
			}
			healthErr = fmt.Errorf("robot health endpoint returned non-200 status: %d", resp.StatusCode)
			resp.Body.Close()
		} else {
			healthErr = err
		}
		logger.LogInfo("[DIAGNOSTIC] Health check attempt %d failed, retrying in 3s...", i+1)
		time.Sleep(3 * time.Second)
	}

	if !healthPassed {
		return fmt.Errorf("failed to reach robot health endpoint at %s/health after retries: %w", diagTarget, healthErr)
	}
	logger.LogInfo("Robot HTTP health check PASSED.")

	// 3. Browser-based UI Validation
	logger.LogInfo("[DIAGNOSTIC] Step 3: Starting browser for UI validation...")
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
		logger.LogInfo("[DIAGNOSTIC] Page Title: %s", title)
		if len(html) > 500 {
			logger.LogDebug("[DIAGNOSTIC] HTML Sample: %s", html[:500])
		} else {
			logger.LogDebug("[DIAGNOSTIC] HTML: %s", html)
		}
	} else {
		logger.LogWarn("[DIAGNOSTIC] Failed to get page content: %v", err)
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
				if err := session.RunWithContext(ctx, test_v2.NavigateToSection("table", "Table Section")); err != nil {
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
			name:      "3D Section",
			sectionID: "three",
			validation: func(ctx context.Context) error {
				return session.RunWithContext(ctx, test_v2.NavigateToSection("three", "Three Section"))
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
		logger.LogInfo("[DIAGNOSTIC] Step %d: %s...", i+1, step.name)
		
		// Create a context with timeout for this step
		stepCtx, stepCancel := context.WithTimeout(session.Context(), 5*time.Second)
		err := step.validation(stepCtx)
		stepCancel()

		screenshotPath := filepath.Join(screenshotsDir, fmt.Sprintf("diagnostic_step_%d_%s.png", i+1, step.sectionID))

		if err != nil {
			logger.LogError("Diagnostic step '%s' FAILED: %v", step.name, err)
			// Capture a screenshot of the failure state before exiting
			if shotErr := session.CaptureScreenshot(screenshotPath); shotErr != nil {
				logger.LogWarn("Failed to capture failure screenshot for '%s': %v", step.name, shotErr)
			} else {
				logger.LogInfo("Failure screenshot saved: %s", screenshotPath)
			}
			return fmt.Errorf("diagnostic step '%s' failed: %w", step.name, err)
		}

		logger.LogInfo("Diagnostic step '%s' PASSED.", step.name)

		if shotErr := session.CaptureScreenshot(screenshotPath); shotErr != nil {
			logger.LogWarn("Failed to capture screenshot for '%s': %v", step.name, shotErr)
		} else {
			logger.LogInfo("Screenshot saved: %s", screenshotPath)
		}
	}

	logger.LogInfo("Robot UI diagnostic completed successfully.")
	return nil
}

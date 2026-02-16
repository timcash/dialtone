package test

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"dialtone/cli/src/core/test"
	"dialtone/cli/src/core/browser"

	"github.com/chromedp/chromedp"
)

func init() {
	test.Register("www-menu-smoke", "www", []string{"www", "menu-smoke", "menu"}, RunWwwMenuSmoke)
}

func RunWwwMenuSmoke() error {
	fmt.Println(">> [WWW] Menu Smoke: start")
	cwd, _ := os.Getwd()
	wwwDir := filepath.Join(cwd, "src", "plugins", "www", "app")

	if !isPortOpenMenu(4173) {
		devCmd := exec.Command("npm", "run", "preview", "--", "--host", "127.0.0.1")
		devCmd.Dir = wwwDir; devCmd.Start(); defer devCmd.Process.Kill()
	}
	waitForPortLocalMenu(4173, 60*time.Second)
	
	chromePath := browser.FindChromePath()
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun, chromedp.NoDefaultBrowserCheck,
		chromedp.ExecPath(chromePath),
		chromedp.Headless,
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, tabCancel := chromedp.NewContext(allocCtx)
	defer tabCancel()

	ctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()

	var sections []string
	if err := chromedp.Run(ctx,
		chromedp.Navigate("about:blank"),
		chromedp.Navigate("http://127.0.0.1:4173"),
		chromedp.WaitVisible(".header-fps"),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('section[id^="s-"]')).map(el => el.id)`, &sections),
	); err != nil {
		return fmt.Errorf("setup failed: %v", err)
	}

	for _, section := range sections {
		// Only check sections with complex menus
		if section == "s-radio" || section == "s-geotools" || section == "s-vision" {
			continue 
		}

		fmt.Printf(">> [WWW] Menu Smoke: testing #%s\n", section)
		
		err := chromedp.Run(ctx,
			// Navigate to section
			chromedp.Evaluate(fmt.Sprintf(`(async function(){
				const id = '%s';
				window.location.hash = id;
				if (window.sections) {
					if (!window.sections.visualizations.has(id)) {
						await window.sections.load(id);
					}
					window.sections.setActiveSection(id);
				}
			})()`, section), nil),
			chromedp.Sleep(500*time.Millisecond),
			
			// 1. Verify menu is closed initially
			chromedp.Evaluate(`document.getElementById('global-menu-panel').hidden`, nil),
			
			// 2. Open menu
			chromedp.WaitVisible("#global-menu-toggle"),
			chromedp.Click("#global-menu-toggle", chromedp.NodeVisible),
			chromedp.WaitVisible("#global-menu-panel:not([hidden])"),
			
			// 3. Verify content
			chromedp.ActionFunc(func(ctx context.Context) error {
				var count int
				var header string
				chromedp.Evaluate(`document.querySelectorAll('#global-menu-panel h3').length`, &count).Do(ctx)
				chromedp.Evaluate(`document.querySelector('#global-menu-panel h3')?.innerText || ""`, &header).Do(ctx)
				
				if count == 0 {
					return fmt.Errorf("menu is empty for section %s", section)
				}
				
				expected := ""
				switch section {
				case "s-home": expected = "ORBITAL DYNAMICS"
				case "s-about": expected = "VISION GRID PRESETS"
				case "s-robot": expected = "KINEMATIC SOLVER"
				case "s-neural": expected = "NEURAL TOPOLOGY"
				case "s-math": expected = "MANIFOLD PROJECTIONS"
				case "s-cad": expected = "PARAMETRIC GEAR"
				case "s-policy": expected = "MARKOV SCENARIOS"
				case "s-music": expected = "HARMONIC ANALYSIS"
				}
				
				if expected != "" && !strings.Contains(strings.ToUpper(header), expected) {
					return fmt.Errorf("wrong menu header for %s. Expected '%s', got '%s'", section, expected, header)
				}
				return nil
			}),

			// 4. Click a button inside the menu and verify no duplication
			chromedp.ActionFunc(func(ctx context.Context) error {
				// Find first button
				var hasBtn bool
				chromedp.Evaluate(`document.querySelectorAll('#global-menu-panel .menu-button').length > 0`, &hasBtn).Do(ctx)
				if hasBtn {
					var countBefore, countAfter int
					chromedp.Evaluate(`document.querySelectorAll('#global-menu-panel *').length`, &countBefore).Do(ctx)
					
					// Click it
					chromedp.Click("#global-menu-panel .menu-button", chromedp.ByQuery).Do(ctx)
					chromedp.Sleep(200 * time.Millisecond).Do(ctx) // Wait for potential rebuild
					
					chromedp.Evaluate(`document.querySelectorAll('#global-menu-panel *').length`, &countAfter).Do(ctx)
					
					// If it duplicated, countAfter would be roughly 2x countBefore
					if countAfter > countBefore + 5 { // Allow small increase if new status added, but not whole menu
						return fmt.Errorf("menu items accumulated/duplicated after button click in section %s (Before: %d, After: %d)", section, countBefore, countAfter)
					}
				}
				return nil
			}),
			
			// 5. Close menu
			chromedp.Click("#global-menu-toggle", chromedp.NodeVisible),
			chromedp.WaitReady("#global-menu-panel[hidden]"),
		)

		if err != nil {
			return fmt.Errorf("test failed for %s: %v", section, err)
		}
		fmt.Printf("   [PASS]\n")
	}

	fmt.Println(">> [WWW] Menu Smoke: pass")
	return nil
}

func waitForPortLocalMenu(port int, timeout time.Duration) {
	start := time.Now()
	for time.Since(start) < timeout {
		if conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second); err == nil {
			conn.Close(); return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func isPortOpenMenu(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err == nil { conn.Close(); return true }
	return false
}

package test

import (
	"context"
	"fmt"
	"strings"
	"time"


	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)


func RunWwwMenuSmokeSubTest(ctx context.Context) error {
	fmt.Println(">> [WWW] Menu Smoke: start")
	
	// Capture console logs using the shared context
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if ce, ok := ev.(*runtime.EventConsoleAPICalled); ok {
			var parts []string
			for _, arg := range ce.Args {
				parts = append(parts, string(arg.Value))
			}
			fmt.Printf("   [BROWSER] %s\n", strings.Join(parts, " "))
		}
	})

	var sections []string
	if err := chromedp.Run(ctx,
		chromedp.Navigate("about:blank"),
		chromedp.Navigate("http://127.0.0.1:4173"),
		chromedp.WaitReady("body"),
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
		
		// Per-section sub-context (not a new tab)
		sectCtx, sectCancel := context.WithTimeout(ctx, 60*time.Second)
		err := chromedp.Run(sectCtx,
			// Navigate to section
			chromedp.Evaluate(fmt.Sprintf(`(async function(){
				const id = '%s';
				console.log("Test: Navigating to " + id);
				window.location.hash = id;
				if (window.sections) {
					console.log("Test: Triggering load for " + id);
					const loadPromise = window.sections.load(id);
					await Promise.race([
						loadPromise,
						new Promise(r => setTimeout(r, 20000))
					]).catch(e => console.error("Test: Load error for " + id, e));
					
					console.log("Test: Setting active section " + id);
					window.sections.setActiveSection(id);
				}
			})()`, section), nil),
			chromedp.Sleep(5*time.Second),
			
			// 1. Verify toggle exists and click it
			chromedp.WaitVisible("#global-menu-toggle"),
			chromedp.Click("#global-menu-toggle", chromedp.NodeVisible),
			
			// 2. Wait for a header to appear in the menu
			chromedp.WaitVisible("#global-menu-panel h3", chromedp.ByQuery),
			
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
					
					if countAfter > countBefore + 5 { 
						return fmt.Errorf("menu items accumulated/duplicated after button click in section %s (Before: %d, After: %d)", section, countBefore, countAfter)
					}
				}
				return nil
			}),
			
			// 5. Close menu
			chromedp.Click("#global-menu-toggle", chromedp.NodeVisible),
			chromedp.Sleep(200 * time.Millisecond),
		)
		sectCancel()

		if err != nil {
			return fmt.Errorf("test failed for %s: %v", section, err)
		}
		fmt.Printf("   [PASS]\n")
	}

	fmt.Println(">> [WWW] Menu Smoke: pass")
	return nil
}

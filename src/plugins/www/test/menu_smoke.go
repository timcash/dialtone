package test

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dialtone/cli/src/core/test"
	"dialtone/cli/src/core/browser"

	"github.com/chromedp/chromedp"
)

func init() {
	test.Register("www-menu-smoke", "www", []string{"www", "smoke", "menu"}, RunWwwMenuSmoke)
}

func RunWwwMenuSmoke() error {
	fmt.Println(">> [WWW] Menu Smoke: start")
	
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

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	var sections []string
	if err := chromedp.Run(ctx,
		chromedp.Navigate("http://localhost:5173"),
		chromedp.WaitReady(".header-fps"),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('section[id^="s-"]')).map(el => el.id)`, &sections),
	); err != nil {
		return fmt.Errorf("setup failed: %v", err)
	}

	for _, section := range sections {
		// Skip sections without menus
		if section == "s-radio" || section == "s-geotools" || section == "s-vision" {
			continue 
		}

		fmt.Printf(">> [WWW] Menu Smoke: checking #%s\n", section)
		
		var menuVisible bool
		var menuRect struct {
			Top    float64 `json:"top"`
			Bottom float64 `json:"bottom"`
		}
		var btnRect struct {
			Top    float64 `json:"top"`
			Bottom float64 `json:"bottom"`
		}
		var menuHeader string

		err := chromedp.Run(ctx,
			// Navigate
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
			
			// Click Menu
			chromedp.WaitVisible("#global-menu-toggle"),
			chromedp.Click("#global-menu-toggle", chromedp.NodeVisible),
			
			// Wait for a header to appear in the menu
			chromedp.WaitVisible("#global-menu-panel h3", chromedp.ByQuery),
			
			// Capture state
			chromedp.Evaluate(`!document.getElementById('global-menu-panel').hidden`, &menuVisible),
			chromedp.Evaluate(`document.getElementById('global-menu-panel').getBoundingClientRect()`, &menuRect),
			chromedp.Evaluate(`document.getElementById('global-menu-toggle').getBoundingClientRect()`, &btnRect),
			chromedp.Evaluate(`document.querySelector('#global-menu-panel h3')?.innerText || ""`, &menuHeader),
			
			// Close
			chromedp.Click("#global-menu-toggle", chromedp.NodeVisible),
		)

		if err != nil {
			return fmt.Errorf("test failed for %s: %v", section, err)
		}

		if !menuVisible {
			return fmt.Errorf("menu panel failed to show for section %s", section)
		}

		// Verify it's ABOVE the button
		if menuRect.Bottom > btnRect.Top + 10 {
			return fmt.Errorf("menu panel for %s is NOT above the menu button", section)
		}

		// Verify expected content
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

		if expected != "" && !strings.Contains(strings.ToUpper(menuHeader), expected) {
			return fmt.Errorf("wrong menu content for %s. Expected '%s', got '%s'", section, expected, menuHeader)
		}

		fmt.Printf("   [PASS] Header: '%s'\n", menuHeader)
	}

	fmt.Println(">> [WWW] Menu Smoke: pass")
	return nil
}

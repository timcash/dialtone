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
		fmt.Printf(">> [WWW] Menu Smoke: checking #%s\n", section)
		
		var menuVisible bool
		var menuRect struct {
			Top    float64 `json:"top"`
			Bottom float64 `json:"bottom"`
			Left   float64 `json:"left"`
			Right  float64 `json:"right"`
		}
		var btnRect struct {
			Top    float64 `json:"top"`
			Bottom float64 `json:"bottom"`
		}
		var menuTitle string

		err := chromedp.Run(ctx,
			// Navigate to section and force logic that usually depends on IntersectionObserver
			chromedp.Evaluate(fmt.Sprintf(`(async function(){
				const id = '%s';
				window.location.hash = id;
				if (window.sections) {
					// Wait for load if not already loaded
					if (!window.sections.visualizations.has(id)) {
						console.log("Test: Triggering load for " + id);
						await window.sections.load(id);
					}
					
					window.sections.activeSectionId = id;
					const config = window.sections.configs.get(id);
					const sectionEl = document.getElementById(id);
					if (sectionEl) {
						window.sections.updateHeader(config?.header, sectionEl);
						window.sections.updateMenu(config?.menu);
						
						const control = window.sections.visualizations.get(id);
						if (control && control.updateUI) {
							console.log("Test: Triggering updateUI for " + id);
							document.getElementById('global-menu-panel').innerHTML = '';
							control.updateUI();
						}
					}
				}
			})()`, section), nil),
			chromedp.Sleep(1200*time.Millisecond),
			
			chromedp.WaitVisible("#global-menu-toggle"),
			chromedp.Click("#global-menu-toggle", chromedp.NodeVisible),
			chromedp.Sleep(800*time.Millisecond),
			
			chromedp.Evaluate(`!document.getElementById('global-menu-panel').hidden`, &menuVisible),
			chromedp.Evaluate(`document.getElementById('global-menu-panel').getBoundingClientRect()`, &menuRect),
			chromedp.Evaluate(`document.getElementById('global-menu-toggle').getBoundingClientRect()`, &btnRect),
			chromedp.Evaluate(`document.querySelector('#global-menu-panel .menu-header')?.innerText || ""`, &menuTitle),
			
			chromedp.Click("#global-menu-toggle", chromedp.NodeVisible),
		)

		if err != nil {
			return fmt.Errorf("test failed for %s: %v", section, err)
		}

		if !menuVisible {
			return fmt.Errorf("menu panel failed to show for section %s", section)
		}

		if menuRect.Bottom > btnRect.Top + 10 {
			return fmt.Errorf("menu panel for %s is NOT above the menu button (Menu Bottom: %.1f, Btn Top: %.1f)", section, menuRect.Bottom, btnRect.Top)
		}

		fmt.Printf("   [PASS] Menu shown above button. Active menu title: '%s'\n", menuTitle)
		
		expectedTitle := ""
		switch section {
		case "s-home": expectedTitle = "ROTATION"
		case "s-robot": expectedTitle = "IK MODE"
		case "s-neural": expectedTitle = "ARCHITECTURE"
		case "s-math": expectedTitle = "CAMERA"
		case "s-cad": expectedTitle = "GEAR PARAMETERS"
		case "s-policy": expectedTitle = "PRESETS"
		case "s-music": expectedTitle = "MUSIC VISUALIZATION"
		case "s-vision": expectedTitle = "BIO-DIGITAL INTEGRATION"
		}

		if expectedTitle != "" && !strings.Contains(strings.ToUpper(menuTitle), expectedTitle) {
			return fmt.Errorf("wrong menu title for section %s. Expected something containing '%s', got '%s'", section, expectedTitle, menuTitle)
		}
	}

	fmt.Println(">> [WWW] Menu Smoke: pass")
	return nil
}

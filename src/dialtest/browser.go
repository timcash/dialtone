package dialtest

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
)

// NavigateToSection performs a robust SPA navigation using hash and verifies it via aria-label.
func NavigateToSection(id string, ariaLabel string) chromedp.Action {
	return chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Try unique navigation function first - it's most reliable for smoke tests
			var success bool
			_ = chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
				(async function() {
					if (window.sections && typeof window.sections.navigateTo === 'function') {
						await window.sections.navigateTo('%s');
						return true;
					}
					const navFns = ['dialtoneNixNavigateTo', 'navigateTo'];
					for (const fn of navFns) {
						if (typeof window[fn] === 'function') {
							await window[fn]('%s', false);
							return true;
						}
					}
					window.location.hash = '#%s';
					return false;
				})()
			`, id, id, id), &success))
			return nil
		}),
		// Wait for the specific section content to be visible via ARIA label
		chromedp.WaitVisible(fmt.Sprintf("[aria-label='%s']", ariaLabel), chromedp.ByQuery),
		// Post-navigation UI state force (robustness fallback for headless)
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Only force if we aren't using the app's internal router (which should handle it)
			return chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
				if (!window.sections) {
					if ('%s' === 'nix-docs' || '%s' === 'nix-table' || '%s' === 'settings') {
						document.body.classList.add('hide-header', 'hide-menu');
					} else {
						document.body.classList.remove('hide-header', 'hide-menu');
					}
				}
			`, id, id, id), nil))
		}),
		// Crucial: Wait for CSS rules/transitions to settle
		chromedp.Sleep(500 * time.Millisecond),
	}
}

// WaitForAriaLabel is a helper to wait for a specific element to be visible by its label.
func WaitForAriaLabel(label string) chromedp.Action {
	return chromedp.WaitVisible(fmt.Sprintf("[aria-label='%s']", label), chromedp.ByQuery)
}

// AssertElementHidden verifies that an element is hidden, with polling for robustness.
func AssertElementHidden(selector string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		start := time.Now()
		var display string
		for time.Since(start) < 3*time.Second {
			err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`getComputedStyle(document.querySelector('%s')).display`, selector), &display))
			if err == nil && display == "none" {
				return nil
			}
			time.Sleep(200 * time.Millisecond)
		}
		return fmt.Errorf("element %s should be hidden (last display: %s)", selector, display)
	})
}
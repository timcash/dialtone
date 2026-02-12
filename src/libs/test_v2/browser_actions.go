package test_v2

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func NavigateToSection(id string, ariaLabel string) chromedp.Action {
	return chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			var success bool
			_ = chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
				(async function() {
					if (window.sections && typeof window.sections.navigateTo === 'function') {
						await window.sections.navigateTo('%s');
						return true;
					}
					if (typeof window.navigateTo === 'function') {
						await window.navigateTo('%s');
						return true;
					}
					window.location.hash = '#%s';
					return false;
				})()
			`, id, id, id), &success))
			return nil
		}),
		chromedp.WaitVisible(fmt.Sprintf("[aria-label='%s']", ariaLabel), chromedp.ByQuery),
		chromedp.Sleep(250 * time.Millisecond),
	}
}

func WaitForAriaLabel(label string) chromedp.Action {
	return chromedp.WaitVisible(fmt.Sprintf("[aria-label='%s']", label), chromedp.ByQuery)
}

func ClickAriaLabel(label string) chromedp.Action {
	return chromedp.Click(fmt.Sprintf("[aria-label='%s']", label), chromedp.ByQuery)
}

func AssertAriaLabelTextContains(label string, substr string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var text string
		if err := chromedp.Run(ctx, chromedp.Text(fmt.Sprintf("[aria-label='%s']", label), &text, chromedp.ByQuery)); err != nil {
			return err
		}
		if text == "" {
			return fmt.Errorf("aria-label %s has empty text", label)
		}
		if !strings.Contains(text, substr) {
			return fmt.Errorf("aria-label %s text %q does not contain %q", label, text, substr)
		}
		return nil
	})
}

func AssertAriaLabelAttrEquals(label string, attr string, expected string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var value string
		if err := chromedp.Run(ctx, chromedp.AttributeValue(fmt.Sprintf("[aria-label='%s']", label), attr, &value, nil, chromedp.ByQuery)); err != nil {
			return err
		}
		if value != expected {
			return fmt.Errorf("aria-label %s attr %s expected %q got %q", label, attr, expected, value)
		}
		return nil
	})
}

func WaitForAriaLabelAttrEquals(label string, attr string, expected string, timeout time.Duration) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		start := time.Now()
		var value string
		for time.Since(start) < timeout {
			if err := chromedp.Run(ctx, chromedp.AttributeValue(fmt.Sprintf("[aria-label='%s']", label), attr, &value, nil, chromedp.ByQuery)); err == nil && value == expected {
				return nil
			}
			time.Sleep(120 * time.Millisecond)
		}
		return fmt.Errorf("aria-label %s attr %s expected %q got %q", label, attr, expected, value)
	})
}

func AssertElementHidden(selector string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		start := time.Now()
		var display string
		for time.Since(start) < 3*time.Second {
			err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
				(function(){
				  const el = document.querySelector('%s');
				  if (!el) return 'none';
				  return getComputedStyle(el).display;
				})()
			`, selector), &display))
			if err == nil && display == "none" {
				return nil
			}
			time.Sleep(150 * time.Millisecond)
		}
		return fmt.Errorf("element %s should be hidden (last display: %s)", selector, display)
	})
}

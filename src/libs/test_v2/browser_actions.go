package test_v2

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

func ComposeSectionID(pluginName string, subName string, underlayKind string) string {
	p := strings.TrimSpace(strings.ToLower(pluginName))
	s := strings.TrimSpace(strings.ToLower(subName))
	u := strings.TrimSpace(strings.ToLower(underlayKind))
	if p == "" || s == "" || u == "" {
		return ""
	}
	return fmt.Sprintf("%s-%s-%s", p, s, u)
}

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

func TypeAriaLabel(label string, value string) chromedp.Action {
	return chromedp.SetValue(fmt.Sprintf("[aria-label='%s']", label), value, chromedp.ByQuery)
}

func PressEnterAriaLabel(label string) chromedp.Action {
	return chromedp.SendKeys(fmt.Sprintf("[aria-label='%s']", label), kb.Enter, chromedp.ByQuery)
}

func TypeAndSubmitAriaLabel(label string, value string) chromedp.Action {
	return chromedp.Tasks{
		WaitForAriaLabel(label),
		ClickAriaLabel(label),
		TypeAriaLabel(label, value),
		PressEnterAriaLabel(label),
	}
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

func AssertAriaLabelInsideViewport(label string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		type rectResult struct {
			Ok     bool    `json:"ok"`
			Top    float64 `json:"top"`
			Left   float64 `json:"left"`
			Bottom float64 `json:"bottom"`
			Right  float64 `json:"right"`
			W      float64 `json:"w"`
			H      float64 `json:"h"`
		}
		var out rectResult
		if err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const el = document.querySelector("[aria-label='%s']");
				if (!el) return { ok: false, top: 0, left: 0, bottom: 0, right: 0, w: window.innerWidth, h: window.innerHeight };
				const r = el.getBoundingClientRect();
				const ok = r.top >= 0 && r.left >= 0 && r.bottom <= window.innerHeight && r.right <= window.innerWidth && r.width > 0 && r.height > 0;
				return { ok, top: r.top, left: r.left, bottom: r.bottom, right: r.right, w: window.innerWidth, h: window.innerHeight };
			})()
		`, label), &out)); err != nil {
			return err
		}
		if !out.Ok {
			return fmt.Errorf("aria-label %s is outside viewport (rect top=%0.1f left=%0.1f bottom=%0.1f right=%0.1f viewport=%0.1fx%0.1f)", label, out.Top, out.Left, out.Bottom, out.Right, out.W, out.H)
		}
		return nil
	})
}

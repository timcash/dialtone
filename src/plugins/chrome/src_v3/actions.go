package src_v3

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"github.com/chromedp/chromedp"
)

func (d *daemonState) navigateManaged(rawURL string) error {
	url := normalizeURL(rawURL)
	d.mu.Lock()
	d.consoleLines = nil
	d.currentURL = url
	d.mu.Unlock()
	return d.withManagedContext(30*time.Second, func(ctx context.Context) error {
		return chromedp.Run(ctx, chromedp.Navigate(url))
	})
}

func (d *daemonState) clickAriaLabel(label string) error {
	selector := ariaSelector(label)
	selectorJSON, err := json.Marshal(selector)
	if err != nil {
		return err
	}
	script := fmt.Sprintf(`(() => {
		const el = document.querySelector(%s);
		if (!el) return "missing";
		el.click();
		return "ok";
	})()`, string(selectorJSON))
	return d.withManagedContext(15*time.Second, func(ctx context.Context) error {
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			var result string
			if err := chromedp.Run(ctx, chromedp.Evaluate(script, &result)); err != nil {
				return err
			}
			if result == "ok" {
				return nil
			}
			time.Sleep(120 * time.Millisecond)
		}
		return fmt.Errorf("click target %q not found", label)
	})
}

func (d *daemonState) pressEnterAriaLabel(label string) error {
	selector := ariaSelector(label)
	return d.withManagedContext(15*time.Second, func(ctx context.Context) error {
		return chromedp.Run(ctx,
			chromedp.WaitVisible(selector, chromedp.ByQuery),
			chromedp.SendKeys(selector, "\r", chromedp.ByQuery),
		)
	})
}

func (d *daemonState) typeAriaLabel(label, value string) error {
	selector := ariaSelector(label)
	selectorJSON, err := json.Marshal(selector)
	if err != nil {
		return err
	}
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return err
	}
	script := fmt.Sprintf(`(() => {
		const el = document.querySelector(%s);
		if (!el) return "missing";
		el.focus();
		el.value = %s;
		el.dispatchEvent(new Event("input", { bubbles: true }));
		el.dispatchEvent(new Event("change", { bubbles: true }));
		return "ok";
	})()`, string(selectorJSON), string(valueJSON))
	return d.withManagedContext(15*time.Second, func(ctx context.Context) error {
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			var result string
			if err := chromedp.Run(ctx, chromedp.Evaluate(script, &result)); err != nil {
				return err
			}
			if result == "ok" {
				return nil
			}
			time.Sleep(120 * time.Millisecond)
		}
		return fmt.Errorf("type target %q not found", label)
	})
}

func (d *daemonState) waitForAriaLabel(label string, timeout time.Duration) error {
	selector := ariaSelector(label)
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return d.withManagedContext(timeout, func(ctx context.Context) error {
		return chromedp.Run(ctx, chromedp.WaitVisible(selector, chromedp.ByQuery))
	})
}

func (d *daemonState) waitForAriaLabelAttrEquals(label, attr, expected string, timeout time.Duration) error {
	if strings.TrimSpace(attr) == "" {
		return fmt.Errorf("wait-aria-attr requires attr")
	}
	selector := ariaSelector(label)
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return d.withManagedContext(timeout, func(ctx context.Context) error {
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			var actual string
			var ok bool
			if err := chromedp.Run(ctx, chromedp.AttributeValue(selector, attr, &actual, &ok, chromedp.ByQuery)); err == nil && ok && actual == expected {
				return nil
			}
			time.Sleep(120 * time.Millisecond)
		}
		return fmt.Errorf("timed out waiting for aria-label %q attr %q=%q", label, attr, expected)
	})
}

func (d *daemonState) readAriaLabelAttr(label, attr string) (string, error) {
	if strings.TrimSpace(attr) == "" {
		return "", fmt.Errorf("get-aria-attr requires attr")
	}
	selector := ariaSelector(label)
	var actual string
	var ok bool
	err := d.withManagedContext(5*time.Second, func(ctx context.Context) error {
		return chromedp.Run(ctx, chromedp.AttributeValue(selector, attr, &actual, &ok, chromedp.ByQuery))
	})
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("aria-label %q attr %q not found", label, attr)
	}
	return actual, nil
}

func (d *daemonState) setManagedHTML(markup string) error {
	if err := d.navigateManaged("about:blank"); err != nil {
		return err
	}
	raw, err := json.Marshal(markup)
	if err != nil {
		return err
	}
	script := fmt.Sprintf(`document.open(); document.write(%s); document.close();`, string(raw))
	return d.withManagedContext(15*time.Second, func(ctx context.Context) error {
		return chromedp.Run(ctx, chromedp.Evaluate(script, nil))
	})
}

func (d *daemonState) waitForConsoleContains(substr string, timeout time.Duration) ([]string, error) {
	substr = strings.TrimSpace(substr)
	if substr == "" {
		return nil, fmt.Errorf("wait-log requires contains")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		d.mu.Lock()
		lines := append([]string(nil), d.consoleLines...)
		d.mu.Unlock()
		for _, line := range lines {
			if strings.Contains(line, substr) {
				return lines, nil
			}
		}
		time.Sleep(120 * time.Millisecond)
	}
	return nil, fmt.Errorf("timed out waiting for console log containing %q", substr)
}

func (d *daemonState) captureScreenshotB64() (string, error) {
	logs.Info("chrome src_v3 screenshot start role=%s", d.role)
	var buf []byte
	if err := d.withManagedContext(20*time.Second, func(ctx context.Context) error {
		return chromedp.Run(ctx,
			chromedp.WaitVisible("body", chromedp.ByQuery),
			chromedp.Sleep(300*time.Millisecond),
			chromedp.CaptureScreenshot(&buf),
		)
	}); err != nil {
		logs.Error("chrome src_v3 screenshot failed role=%s err=%v", d.role, err)
		return "", err
	}
	encoded := base64.StdEncoding.EncodeToString(buf)
	logs.Info("chrome src_v3 screenshot complete role=%s bytes=%d b64_len=%d", d.role, len(buf), len(encoded))
	return encoded, nil
}

func (d *daemonState) readManagedURL() (string, error) {
	var current string
	err := d.withManagedContext(10*time.Second, func(ctx context.Context) error {
		return chromedp.Run(ctx, chromedp.Location(&current))
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(current), nil
}

func (d *daemonState) evaluateManagedScript(script string) (string, error) {
	script = strings.TrimSpace(script)
	if script == "" {
		return "", fmt.Errorf("eval requires script")
	}
	var result string
	err := d.withManagedContext(10*time.Second, func(ctx context.Context) error {
		return chromedp.Run(ctx, chromedp.Evaluate(script, &result))
	})
	if err != nil {
		return "", err
	}
	return result, nil
}

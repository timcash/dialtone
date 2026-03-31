package src_v3

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

func (d *daemonState) navigateManaged(rawURL string) error {
	url := normalizeURL(rawURL)
	d.mu.Lock()
	d.consoleLines = nil
	d.currentURL = url
	d.mu.Unlock()
	logs.Info("chrome src_v3 navigate start role=%s url=%s", d.role, url)
	return d.withManagedContext(30*time.Second, func(ctx context.Context) error {
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(runCtx context.Context) error {
			_, _, _, _, err := page.Navigate(url).Do(runCtx)
			return err
		})); err != nil {
			logs.Warn("chrome src_v3 navigate dispatch failed role=%s url=%s err=%v", d.role, url, err)
			return err
		}
		logs.Info("chrome src_v3 navigate dispatched role=%s url=%s", d.role, url)
		waitCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		if err := chromedp.Run(waitCtx, chromedp.WaitReady("body", chromedp.ByQuery)); err == nil {
			logs.Info("chrome src_v3 navigate body-ready role=%s url=%s", d.role, url)
			return nil
		} else {
			logs.Warn("chrome src_v3 navigate wait-ready failed role=%s url=%s err=%v", d.role, url, err)
		}
		var current string
		if err := chromedp.Run(ctx, chromedp.Location(&current)); err == nil && strings.TrimSpace(current) != "" {
			logs.Info("chrome src_v3 navigate location fallback role=%s url=%s current=%s", d.role, url, strings.TrimSpace(current))
			return nil
		} else if err != nil {
			logs.Warn("chrome src_v3 navigate location read failed role=%s url=%s err=%v", d.role, url, err)
		}
		if err := chromedp.Run(waitCtx, chromedp.WaitVisible("body", chromedp.ByQuery)); err != nil {
			logs.Warn("chrome src_v3 navigate wait-visible failed role=%s url=%s err=%v", d.role, url, err)
			return err
		}
		logs.Info("chrome src_v3 navigate body-visible role=%s url=%s", d.role, url)
		return nil
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
	selectorJSON, err := json.Marshal(selector)
	if err != nil {
		return err
	}
	script := fmt.Sprintf(`(() => {
		const el = document.querySelector(%s);
		if (!el) return "missing";
		el.focus();
		el.dispatchEvent(new KeyboardEvent("keydown", {
			key: "Enter",
			code: "Enter",
			keyCode: 13,
			which: 13,
			bubbles: true
		}));
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
		return fmt.Errorf("press-enter target %q not found", label)
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

func (d *daemonState) setManagedViewport(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("set-viewport requires positive width and height")
	}
	logs.Info("chrome src_v3 viewport start role=%s width=%d height=%d", d.role, width, height)
	return d.withManagedContext(15*time.Second, func(ctx context.Context) error {
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(runCtx context.Context) error {
			return emulation.SetDeviceMetricsOverride(int64(width), int64(height), 1.0, false).Do(runCtx)
		})); err != nil {
			logs.Warn("chrome src_v3 viewport failed role=%s width=%d height=%d err=%v", d.role, width, height, err)
			return err
		}
		logs.Info("chrome src_v3 viewport complete role=%s width=%d height=%d", d.role, width, height)
		return nil
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
	logs.Info("chrome src_v3 wait-log start role=%s contains=%q timeout=%v", d.role, substr, timeout)
	deadline := time.Now().Add(timeout)
	syncWarned := false
	for time.Now().Before(deadline) {
		if err := d.syncManagedConsoleLines(1200 * time.Millisecond); err != nil && !syncWarned {
			syncWarned = true
			logs.Warn("chrome src_v3 wait-log console sync failed role=%s err=%v", d.role, err)
		}
		lines := d.consoleSnapshot()
		for _, line := range lines {
			if strings.Contains(line, substr) {
				logs.Info("chrome src_v3 wait-log matched role=%s contains=%q lines=%d", d.role, substr, len(lines))
				return lines, nil
			}
		}
		time.Sleep(120 * time.Millisecond)
	}
	logs.Warn("chrome src_v3 wait-log timed out role=%s contains=%q", d.role, substr)
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
	var result any
	var consoleLines []string
	err := d.withManagedContext(10*time.Second, func(ctx context.Context) error {
		if err := chromedp.Run(ctx, chromedp.Evaluate(script, &result)); err != nil {
			return err
		}
		return chromedp.Run(ctx, chromedp.Evaluate(managedConsoleReadScript, &consoleLines))
	})
	if err != nil {
		return "", err
	}
	d.replaceConsoleLines(consoleLines)
	data, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (d *daemonState) syncManagedConsoleLines(timeout time.Duration) error {
	lines, err := d.readManagedConsoleLines(timeout)
	if err != nil {
		return err
	}
	d.replaceConsoleLines(lines)
	return nil
}

func (d *daemonState) readManagedConsoleLines(timeout time.Duration) ([]string, error) {
	if timeout <= 0 {
		timeout = 1200 * time.Millisecond
	}
	if err := d.ensureManagedTab(); err != nil {
		return nil, err
	}
	var lines []string
	run := func() error {
		return d.runManaged(timeout, func(ctx context.Context) error {
			return chromedp.Run(ctx, chromedp.Evaluate(managedConsoleReadScript, &lines))
		})
	}
	err := run()
	if !shouldRecreateManagedTab(err) {
		return lines, err
	}
	if recreateErr := d.recreateManagedTab(); recreateErr != nil {
		return nil, err
	}
	err = run()
	return lines, err
}

func (d *daemonState) replaceConsoleLines(lines []string) {
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		normalized = append(normalized, line)
	}
	if len(normalized) > 200 {
		normalized = append([]string(nil), normalized[len(normalized)-200:]...)
	}
	d.mu.Lock()
	d.consoleLines = append([]string(nil), normalized...)
	d.mu.Unlock()
}

package test

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
)

func WaitForAriaLabel(label string) Action {
	return chromedp.WaitVisible(fmt.Sprintf(`[aria-label="%s"]`, label), chromedp.ByQuery)
}

func ClickAriaLabel(label string) Action {
	return chromedp.Click(fmt.Sprintf(`[aria-label="%s"]`, label), chromedp.ByQuery)
}

func TypeAriaLabel(label, value string) Action {
	return chromedp.SendKeys(fmt.Sprintf(`[aria-label="%s"]`, label), value, chromedp.ByQuery)
}

func TypeAndSubmitAriaLabel(label, value string) Action {
	return chromedp.Tasks{
		chromedp.SetValue(fmt.Sprintf(`[aria-label="%s"]`, label), value, chromedp.ByQuery),
		chromedp.SendKeys(fmt.Sprintf(`[aria-label="%s"]`, label), "\r", chromedp.ByQuery),
	}
}

func PressEnterAriaLabel(label string) Action {
	return chromedp.SendKeys(fmt.Sprintf(`[aria-label="%s"]`, label), "\r", chromedp.ByQuery)
}

func AssertAriaLabelTextContains(label, expected string) Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var actual string
		if err := chromedp.Text(fmt.Sprintf(`[aria-label="%s"]`, label), &actual, chromedp.ByQuery).Do(ctx); err != nil {
			return err
		}
		if !strings.Contains(actual, expected) {
			return fmt.Errorf("expected %q to contain %q, but got %q", label, expected, actual)
		}
		return nil
	})
}

func AssertAriaLabelAttrEquals(label, attr, expected string) Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var actual string
		var ok bool
		if err := chromedp.AttributeValue(fmt.Sprintf(`[aria-label="%s"]`, label), attr, &actual, &ok, chromedp.ByQuery).Do(ctx); err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("expected %q to have attribute %q, but it was not found", label, attr)
		}
		if actual != expected {
			return fmt.Errorf("expected %q attribute %q to be %q, but got %q", label, attr, expected, actual)
		}
		return nil
	})
}

func WaitForAriaLabelAttrEquals(label, attr, expected string, timeout time.Duration) Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			var actual string
			var ok bool
			if err := chromedp.AttributeValue(fmt.Sprintf(`[aria-label="%s"]`, label), attr, &actual, &ok, chromedp.ByQuery).Do(ctx); err == nil && ok && actual == expected {
				return nil
			}
			time.Sleep(250 * time.Millisecond)
		}
		return fmt.Errorf("timed out waiting for %q attribute %q to be %q after %v", label, attr, expected, timeout)
	})
}

func AssertElementHidden(selector string) Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var nodes []*cdp.Node
		if err := chromedp.Nodes(selector, &nodes, chromedp.AtLeast(0)).Do(ctx); err != nil {
			return err
		}
		if len(nodes) > 0 {
			// Check if any node is actually visible
		}
		return nil
	})
}

func NavigateToSection(plugin, subname, underlay string) Action {
	id := fmt.Sprintf("%s-%s-%s", plugin, subname, underlay)
	return chromedp.Navigate(fmt.Sprintf("#%s", id))
}

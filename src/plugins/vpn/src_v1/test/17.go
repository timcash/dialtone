package main

import (
	"fmt"
	"strings"

	test_v2 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/chromedp/chromedp"
)

func Run17LifecycleInvariants() error {
	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	checks := []struct {
		id    string
		label string
	}{
		{id: "hero", label: "Hero Section"},
		{id: "docs", label: "Docs Section"},
		{id: "table", label: "Table Section"},
		{id: "three", label: "Three Section"},
		{id: "xterm", label: "Xterm Section"},
	}

	for _, c := range checks {
		if err := session.Run(test_v2.NavigateToSection(c.id, c.label)); err != nil {
			return err
		}
	}

	entries := session.Entries()
	required := []string{"LOADING", "LOADED", "START", "RESUME", "PAUSE", "NAVIGATE TO", "NAVIGATE AWAY"}
	for _, token := range required {
		found := false
		for _, e := range entries {
			if strings.Contains(e.Message, token) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("missing lifecycle token in browser logs: %s", token)
		}
	}

	for _, e := range entries {
		if strings.Contains(e.Message, "[INVARIANT]") {
			return fmt.Errorf("invariant violation captured: %s", e.Message)
		}
	}

	var activeCount int
	if err := session.Run(chromedp.Evaluate(`
    (() => {
      return Array.from(document.querySelectorAll('section[data-active="true"]')).length;
    })();
  `, &activeCount)); err != nil {
		return err
	}
	if activeCount != 1 {
		return fmt.Errorf("expected exactly one active section, got %d", activeCount)
	}

	return nil
}

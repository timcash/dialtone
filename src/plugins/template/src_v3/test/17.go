package main

import (
	"fmt"
	"strings"

	test_v2 "dialtone/cli/src/libs/test_v2"
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
		{id: "template-hero-stage", label: "Hero Section"},
		{id: "template-docs-docs", label: "Docs Section"},
		{id: "template-meta-table", label: "Table Section"},
		{id: "template-three-stage", label: "Three Section"},
		{id: "template-log-xterm", label: "Log Section"},
		{id: "template-demo-video", label: "Video Section"},
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

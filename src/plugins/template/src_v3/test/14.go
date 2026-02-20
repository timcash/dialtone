package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	test_v2 "dialtone/dev/plugins/dag/src_v3/suite"
	"github.com/chromedp/chromedp"
)

func Run14ThreeSectionValidation() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	if err := session.Run(test_v2.NavigateToSection("template-three-stage", "Three Section")); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabel("Three Canvas")); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabelAttrEquals("Three Canvas", "data-ready", "true", 4*time.Second)); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabel("Three Mode")); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabel("Three Add")); err != nil {
		return err
	}
	if err := session.Run(test_v2.ClickAriaLabel("Three Add")); err != nil {
		return err
	}

	var selected string
	if err := session.Run(chromedp.Evaluate(`(() => {
		const c = document.querySelector("[aria-label='Three Canvas']");
		if (!c) return "";
		return String(c.getAttribute('data-selected-node') || '');
	})()`, &selected)); err != nil {
		return err
	}
	if selected == "" {
		return fmt.Errorf("three stage did not select a node after Three Add")
	}

	shot := filepath.Join(repoRoot, "src", "plugins", "template", "src_v3", "screenshots", "test_step_4.png")
	return session.CaptureScreenshot(shot)
}

package main

import (
	"os"
	"path/filepath"
	"time"

	test_v2 "dialtone/dev/plugins/test"
)

func Run13TableSectionValidation() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	if err := session.Run(test_v2.NavigateToSection("template-meta-table", "Table Section")); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabel("Template Table")); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabelAttrEquals("Template Table", "data-ready", "true", 3*time.Second)); err != nil {
		return err
	}

	shot := filepath.Join(repoRoot, "src", "plugins", "template", "src_v3", "screenshots", "test_step_3.png")
	return session.CaptureScreenshot(shot)
}

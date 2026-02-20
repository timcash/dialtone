package main

import (
	test_v2 "dialtone/dev/plugins/dag/src_v3/suite"
	"time"
)

func Run15LogSectionValidation() error {
	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	if err := session.Run(test_v2.NavigateToSection("template-log-xterm", "Log Section")); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabel("Log Terminal")); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabelAttrEquals("Log Terminal", "data-ready", "true", 3*time.Second)); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabel("Log Input")); err != nil {
		return err
	}
	const cmd = "tail --lines 20"
	if err := session.Run(test_v2.TypeAndSubmitAriaLabel("Log Input", cmd)); err != nil {
		return err
	}
	if err := session.Run(test_v2.WaitForAriaLabelAttrEquals("Log Terminal", "data-last-command", cmd, 3*time.Second)); err != nil {
		return err
	}
	return nil
}

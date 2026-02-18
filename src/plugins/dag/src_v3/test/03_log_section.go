package main

import "time"

func Run03LogSectionEcho(ctx *testCtx) (string, error) {
	_, err := ctx.browser()
	if err != nil {
		return "", err
	}
	if err := ctx.navigate(ctx.appURL("/#dag-log-xterm")); err != nil {
		return "", err
	}
	if err := ctx.waitAria("Log Terminal", "log section terminal should be visible"); err != nil {
		return "", err
	}
	if err := ctx.waitAria("Log Command Input", "log thumbs command input should be visible"); err != nil {
		return "", err
	}
	cmd := "status --echo-test"
	if err := ctx.typeAria("Log Command Input", cmd, "enter log command"); err != nil {
		return "", err
	}
	if err := ctx.pressEnterAria("Log Command Input", "submit log command"); err != nil {
		return "", err
	}
	if err := ctx.waitLogTerminalContains("USER> "+cmd, 3*time.Second); err != nil {
		return "", err
	}
	return "Navigated to `dag-log-xterm`, entered a command, and verified command echo in terminal output.", nil
}

package test

import (
	"fmt"

	chrome "dialtone/dev/plugins/chrome/app"
)

func Run() error {
	logLine("info", "Starting Chrome plugin self-test")
	if err := chrome.KillDialtoneResources(); err != nil {
		return fmt.Errorf("pre-cleanup failed: %w", err)
	}

	devSession, err := chrome.StartSession(chrome.SessionOptions{
		RequestedPort: 0,
		GPU:           true,
		Headless:      false,
		Role:          "dev",
		TargetURL:     "about:blank",
		ReuseExisting: false,
	})
	if err != nil {
		return fmt.Errorf("failed to start dev session: %w", err)
	}
	logLine("pass", fmt.Sprintf("Started dev browser pid=%d", devSession.PID))

	reusedDev, err := chrome.StartSession(chrome.SessionOptions{
		RequestedPort: 0,
		GPU:           true,
		Headless:      false,
		Role:          "dev",
		ReuseExisting: true,
	})
	if err != nil {
		_ = chrome.CleanupSession(devSession)
		return fmt.Errorf("failed to reuse dev session: %w", err)
	}
	if reusedDev.PID != devSession.PID || reusedDev.IsNew {
		_ = chrome.CleanupSession(devSession)
		return fmt.Errorf("dev reuse mismatch: got pid=%d isNew=%t expected pid=%d isNew=false", reusedDev.PID, reusedDev.IsNew, devSession.PID)
	}
	logLine("pass", fmt.Sprintf("Reused dev browser pid=%d", reusedDev.PID))

	smokeSession, err := chrome.StartSession(chrome.SessionOptions{
		RequestedPort: 0,
		GPU:           true,
		Headless:      true,
		Role:          "smoke",
		ReuseExisting: false,
	})
	if err != nil {
		_ = chrome.CleanupSession(devSession)
		return fmt.Errorf("failed to start smoke session: %w", err)
	}
	logLine("pass", fmt.Sprintf("Started smoke browser pid=%d", smokeSession.PID))

	procs, err := chrome.ListResources(true)
	if err != nil {
		_ = chrome.CleanupSession(smokeSession)
		_ = chrome.CleanupSession(devSession)
		return fmt.Errorf("list resources failed: %w", err)
	}

	devProc, devFound := findProc(procs, devSession.PID)
	smokeProc, smokeFound := findProc(procs, smokeSession.PID)
	if !devFound || !smokeFound {
		_ = chrome.CleanupSession(smokeSession)
		_ = chrome.CleanupSession(devSession)
		return fmt.Errorf("expected dev/smoke pids in list: devFound=%t smokeFound=%t", devFound, smokeFound)
	}
	if devProc.Role != "dev" || devProc.IsHeadless {
		_ = chrome.CleanupSession(smokeSession)
		_ = chrome.CleanupSession(devSession)
		return fmt.Errorf("dev process metadata mismatch: role=%s headless=%t", devProc.Role, devProc.IsHeadless)
	}
	if smokeProc.Role != "smoke" || !smokeProc.IsHeadless {
		_ = chrome.CleanupSession(smokeSession)
		_ = chrome.CleanupSession(devSession)
		return fmt.Errorf("smoke process metadata mismatch: role=%s headless=%t", smokeProc.Role, smokeProc.IsHeadless)
	}
	logLine("pass", "Verified role/headless metadata for dev and smoke sessions")

	if err := chrome.CleanupSession(smokeSession); err != nil {
		_ = chrome.CleanupSession(devSession)
		return fmt.Errorf("failed smoke cleanup: %w", err)
	}
	logLine("pass", "Cleaned smoke browser")

	procs, err = chrome.ListResources(true)
	if err != nil {
		_ = chrome.CleanupSession(devSession)
		return fmt.Errorf("list resources post-smoke-cleanup failed: %w", err)
	}
	_, smokeStillPresent := findProc(procs, smokeSession.PID)
	_, devStillPresent := findProc(procs, devSession.PID)
	if smokeStillPresent {
		_ = chrome.CleanupSession(devSession)
		return fmt.Errorf("smoke browser still present after cleanup: pid=%d", smokeSession.PID)
	}
	if !devStillPresent {
		_ = chrome.CleanupSession(devSession)
		return fmt.Errorf("dev browser unexpectedly missing after smoke cleanup: pid=%d", devSession.PID)
	}
	logLine("pass", "Smoke cleanup preserved dev browser")

	if err := chrome.CleanupSession(devSession); err != nil {
		return fmt.Errorf("failed dev cleanup: %w", err)
	}
	logLine("pass", "Cleaned dev browser")
	logLine("info", "Chrome plugin self-test complete")
	return nil
}

func findProc(procs []chrome.ChromeProcess, pid int) (chrome.ChromeProcess, bool) {
	for _, p := range procs {
		if p.PID == pid {
			return p, true
		}
	}
	return chrome.ChromeProcess{}, false
}

func logLine(level, message string) {
	fmt.Printf("[%s] %s\n", level, message)
}

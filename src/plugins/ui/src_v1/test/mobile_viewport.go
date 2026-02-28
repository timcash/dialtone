package test

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

var (
	mobileViewportMu         sync.Mutex
	mobileViewportSessionKey string
)

func ApplyMobileViewport(sc *StepContext) error {
	browser, err := sc.Browser()
	if err != nil {
		return err
	}

	sessionKey := ""
	if browser != nil && browser.Session != nil {
		sessionKey = strings.TrimSpace(browser.Session.WebSocketURL)
		if sessionKey == "" && browser.Session.Port > 0 {
			sessionKey = "port:" + strconv.Itoa(browser.Session.Port)
		}
		if sessionKey == "" && browser.Session.PID > 0 {
			sessionKey = "pid:" + strconv.Itoa(browser.Session.PID)
		}
	}

	mobileViewportMu.Lock()
	if sessionKey != "" && mobileViewportSessionKey == sessionKey {
		mobileViewportMu.Unlock()
		return nil
	}
	mobileViewportMu.Unlock()

	if err := sc.RunBrowserWithTimeout(4*time.Second, chromedp.Tasks{
		// Hard-lock viewport size once for the session; avoid mobile/touch emulation
		// because it can trigger visual-viewport shifts between interactions.
		chromedp.EmulateViewport(393, 852),
	}); err != nil {
		return err
	}

	mobileViewportMu.Lock()
	mobileViewportSessionKey = sessionKey
	mobileViewportMu.Unlock()
	return nil
}

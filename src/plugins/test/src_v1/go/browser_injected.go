package test

import (
	"fmt"
	"strings"
	"sync"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"github.com/chromedp/chromedp"
	"github.com/nats-io/nats.go"
)

var injectedBrowserCheckBySuite sync.Map

func (sc *StepContext) runErrorPingCheckOnce() error {
	checkKey := strings.TrimSpace(sc.SuiteSubject)
	if checkKey == "" {
		checkKey = "__dialtone_error_ping_global__"
	}
	if _, loaded := injectedBrowserCheckBySuite.LoadOrStore(checkKey, true); loaded {
		return nil
	}
	if err := sc.runErrorPingCheck(); err != nil {
		injectedBrowserCheckBySuite.Delete(checkKey)
		return err
	}
	return nil
}

func (sc *StepContext) runErrorPingCheck() error {
	nc := sc.NATSConn()
	if nc == nil {
		sc.Warnf("ERROR_PING: skipped (no nats connection)")
		return nil
	}
	browserSubject := strings.TrimSpace(sc.BrowserSubject)
	errorSubject := strings.TrimSpace(sc.ErrorSubject)
	if browserSubject == "" || errorSubject == "" {
		sc.Warnf("ERROR_PING: skipped (subjects missing browser=%q error=%q)", browserSubject, errorSubject)
		return nil
	}
	b := sc.Session
	if b == nil {
		return fmt.Errorf("error-ping requires an initialized browser session")
	}
	if b.isServiceManaged() {
		sc.Warnf("ERROR_PING: skipped for chrome src_v3 NATS-managed browser session")
		return nil
	}
	if err := waitForBrowserSessionReady(b, 12*time.Second); err != nil {
		return fmt.Errorf("error-ping browser readiness: %w", err)
	}

	markerBrowser := fmt.Sprintf("__DIALTONE_ERROR_PING__:%d", time.Now().UnixNano())
	markerError := markerBrowser + ":error"
	sc.Infof("ERROR_PING: start browser_subject=%s error_subject=%s", browserSubject, errorSubject)

	browserCh := make(chan string, 32)
	errorCh := make(chan string, 32)
	subBrowser, err := nc.Subscribe(browserSubject, func(m *nats.Msg) {
		browserCh <- logs.FormatMessage(m.Subject, m.Data)
	})
	if err != nil {
		return fmt.Errorf("injected browser check subscribe browser topic: %w", err)
	}
	defer subBrowser.Unsubscribe()
	subError, err := nc.Subscribe(errorSubject, func(m *nats.Msg) {
		errorCh <- logs.FormatMessage(m.Subject, m.Data)
	})
	if err != nil {
		return fmt.Errorf("injected browser check subscribe error topic: %w", err)
	}
	defer subError.Unsubscribe()
	if err := nc.Flush(); err != nil {
		return fmt.Errorf("injected browser check flush: %w", err)
	}

	js := fmt.Sprintf(`(() => {
  console.log(%q);
  console.error(%q);
  return true;
})()`, markerBrowser, markerError)
	var ok bool
	var evalErr error
	for attempt := 1; attempt <= 4; attempt++ {
		evalErr = b.RunWithTimeout(7*time.Second, chromedp.Evaluate(js, &ok))
		if evalErr == nil {
			break
		}
		lower := strings.ToLower(evalErr.Error())
		recoverable := isNoBrowserOpenError(evalErr) ||
			isNoTargetIDError(evalErr) ||
			strings.Contains(lower, "context canceled") ||
			strings.Contains(lower, "context deadline exceeded")
		if !recoverable {
			return fmt.Errorf("error-ping evaluate: %w", evalErr)
		}
		sc.Warnf("ERROR_PING: recoverable evaluate error (attempt %d/4): %v", attempt, evalErr)
		if rerr := b.EnsureOpenPage(); rerr != nil {
			sc.Warnf("ERROR_PING: ensure page after recoverable error failed: %v", rerr)
		}
		time.Sleep(250 * time.Millisecond)
	}
	if evalErr != nil {
		return fmt.Errorf("error-ping evaluate failed after retries: %w", evalErr)
	}

	deadline := time.Now().Add(5 * time.Second)
	gotBrowser := false
	gotError := false
	for time.Now().Before(deadline) && (!gotBrowser || !gotError) {
		select {
		case line := <-browserCh:
			if strings.Contains(line, markerBrowser) {
				gotBrowser = true
				sc.Infof("ERROR_PING: browser-topic-ok marker=%s", markerBrowser)
			}
		case line := <-errorCh:
			if strings.Contains(line, markerError) {
				gotError = true
				sc.Infof("ERROR_PING: error-topic-ok marker=%s", markerError)
			}
		case <-time.After(120 * time.Millisecond):
		}
	}
	if !gotBrowser || !gotError {
		return fmt.Errorf("error-ping failed: browser_topic=%t error_topic=%t", gotBrowser, gotError)
	}
	sc.Infof("ERROR_PING: pass browser_topic=%t error_topic=%t", gotBrowser, gotError)
	return nil
}

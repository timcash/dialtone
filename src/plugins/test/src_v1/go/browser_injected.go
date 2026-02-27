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

func (sc *StepContext) runInjectedBrowserErrorCheckOnce() error {
	suiteKey := strings.TrimSpace(sc.SuiteSubject)
	if suiteKey == "" {
		return nil
	}
	if _, loaded := injectedBrowserCheckBySuite.LoadOrStore(suiteKey, true); loaded {
		return nil
	}
	if err := sc.runInjectedBrowserErrorCheck(); err != nil {
		injectedBrowserCheckBySuite.Delete(suiteKey)
		return err
	}
	return nil
}

func (sc *StepContext) runInjectedBrowserErrorCheck() error {
	nc := sc.NATSConn()
	if nc == nil {
		sc.Warnf("INJECTED_BROWSER_CHECK: skipped (no nats connection)")
		return nil
	}
	browserSubject := strings.TrimSpace(sc.BrowserSubject)
	errorSubject := strings.TrimSpace(sc.ErrorSubject)
	if browserSubject == "" || errorSubject == "" {
		sc.Warnf("INJECTED_BROWSER_CHECK: skipped (subjects missing browser=%q error=%q)", browserSubject, errorSubject)
		return nil
	}
	b, err := sc.Browser()
	if err != nil {
		return err
	}

	markerBrowser := fmt.Sprintf("__DIALTONE_INJECTED_BROWSER_TOPIC__:%d", time.Now().UnixNano())
	markerError := markerBrowser + ":error"
	sc.Infof("INJECTED_BROWSER_CHECK: start browser_subject=%s error_subject=%s", browserSubject, errorSubject)

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
  setTimeout(() => { throw new Error(%q); }, 0);
  return true;
})()`, markerBrowser, markerError)
	var ok bool
	if err := b.RunWithTimeout(5*time.Second, chromedp.Evaluate(js, &ok)); err != nil {
		return fmt.Errorf("injected browser check evaluate: %w", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	gotBrowser := false
	gotError := false
	for time.Now().Before(deadline) && (!gotBrowser || !gotError) {
		select {
		case line := <-browserCh:
			if strings.Contains(line, markerBrowser) {
				gotBrowser = true
				sc.Infof("INJECTED_BROWSER_CHECK: browser-topic-ok marker=%s", markerBrowser)
			}
		case line := <-errorCh:
			if strings.Contains(line, markerError) {
				gotError = true
				sc.Infof("INJECTED_BROWSER_CHECK: error-topic-ok marker=%s", markerError)
			}
		case <-time.After(120 * time.Millisecond):
		}
	}
	if !gotBrowser || !gotError {
		return fmt.Errorf("injected browser check failed: browser_topic=%t error_topic=%t", gotBrowser, gotError)
	}
	sc.Infof("INJECTED_BROWSER_CHECK: pass browser_topic=%t error_topic=%t", gotBrowser, gotError)
	return nil
}

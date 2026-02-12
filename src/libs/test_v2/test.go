package test_v2

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"dialtone/cli/src/libs/dialtest"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

type ConsoleEntry = dialtest.ConsoleEntry

type BrowserOptions struct {
	Headless        bool
	Role            string
	ReuseExisting   bool
	URL             string
	LogWriter       io.Writer
	LogPrefix       string
	EmitProofOfLife bool
}

type BrowserSession struct {
	chrome *dialtest.ChromeSession

	mu      sync.Mutex
	entries []ConsoleEntry
}

func StartBrowser(opts BrowserOptions) (*BrowserSession, error) {
	s := &BrowserSession{}

	chrome, err := dialtest.StartChromeSession(dialtest.ChromeSessionOptions{
		Headless:        opts.Headless,
		Role:            opts.Role,
		ReuseExisting:   opts.ReuseExisting,
		URL:             opts.URL,
		LogWriter:       opts.LogWriter,
		LogPrefix:       opts.LogPrefix,
		EmitProofOfLife: opts.EmitProofOfLife,
		OnEntry: func(entry dialtest.ConsoleEntry) {
			s.mu.Lock()
			s.entries = append(s.entries, entry)
			s.mu.Unlock()
		},
	})
	if err != nil {
		return nil, err
	}
	s.chrome = chrome
	return s, nil
}

func (s *BrowserSession) Close() {
	if s == nil || s.chrome == nil {
		return
	}
	s.chrome.Close()
}

func (s *BrowserSession) Entries() []ConsoleEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ConsoleEntry, len(s.entries))
	copy(out, s.entries)
	return out
}

func (s *BrowserSession) HasConsoleMessage(substr string) bool {
	for _, e := range s.Entries() {
		if strings.Contains(e.Message, substr) {
			return true
		}
	}
	return false
}

func (s *BrowserSession) Run(actions chromedp.Action) error {
	if s == nil || s.chrome == nil {
		return fmt.Errorf("browser session is not initialized")
	}
	return chromedp.Run(s.chrome.Ctx, actions)
}

func (s *BrowserSession) CaptureScreenshot(path string) error {
	if s == nil || s.chrome == nil {
		return fmt.Errorf("browser session is not initialized")
	}

	var shot []byte
	if err := chromedp.Run(s.chrome.Ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		b, err := page.CaptureScreenshot().Do(ctx)
		if err != nil {
			return err
		}
		shot = b
		return nil
	})); err != nil {
		return err
	}
	if len(shot) == 0 {
		return fmt.Errorf("empty screenshot")
	}
	return os.WriteFile(path, shot, 0o644)
}

package dialtest

import (
	"context"
	"fmt"
	"io"

	chrome_app "dialtone/dev/plugins/chrome/app"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

type ChromeSessionOptions struct {
	RequestedPort   int
	Headless        bool
	Role            string
	ReuseExisting   bool
	URL             string
	LogWriter       io.Writer
	LogPrefix       string
	EmitProofOfLife bool
	OnEntry         func(ConsoleEntry)
}

type ChromeSession struct {
	Ctx          context.Context
	cancel       context.CancelFunc
	IsNewBrowser bool
	BrowserPID   int
	chrome       *chrome_app.Session
}

func StartChromeSession(opts ChromeSessionOptions) (*ChromeSession, error) {
	if opts.Role == "" {
		opts.Role = "default"
	}
	resolved, err := chrome_app.StartSession(chrome_app.SessionOptions{
		RequestedPort: opts.RequestedPort,
		GPU:           true,
		Headless:      opts.Headless,
		TargetURL:     "",
		Role:          opts.Role,
		ReuseExisting: opts.ReuseExisting,
	})
	if err != nil {
		return nil, err
	}

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), resolved.WebSocketURL)
	ctx, ctxCancel := chromedp.NewContext(allocCtx)
	cancel := func() {
		ctxCancel()
		allocCancel()
	}

	logPrefix := opts.LogPrefix
	if logPrefix == "" {
		logPrefix = "[BROWSER]"
	}

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		var entry *ConsoleEntry

		switch e := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			msg := formatConsoleArgs(e.Args)
			entry = &ConsoleEntry{Level: string(e.Type), Message: msg}
		case *runtime.EventExceptionThrown:
			msg := e.ExceptionDetails.Text
			if e.ExceptionDetails.Exception != nil && e.ExceptionDetails.Exception.Description != "" {
				msg = e.ExceptionDetails.Exception.Description
			}
			entry = &ConsoleEntry{Level: "exception", Message: msg}
		}

		if entry == nil {
			return
		}

		if opts.OnEntry != nil {
			opts.OnEntry(*entry)
		}
		if opts.LogWriter != nil {
			fmt.Fprintf(opts.LogWriter, "%s [%s] %s\n", logPrefix, entry.Level, entry.Message)
		}
	})

	tasks := chromedp.Tasks{}
	if opts.URL != "" {
		tasks = append(tasks,
			chromedp.EmulateViewport(1280, 800),
			chromedp.Navigate(opts.URL),
		)
	}
	if opts.EmitProofOfLife {
		tasks = append(tasks, chromedp.Evaluate(`console.error('[PROOFOFLIFE] Intentional Browser Test Error')`, nil))
	}
	if len(tasks) > 0 {
		if err := chromedp.Run(ctx, tasks); err != nil {
			cancel()
			return nil, err
		}
	}

	return &ChromeSession{
		Ctx:          ctx,
		cancel:       cancel,
		IsNewBrowser: resolved.IsNew,
		BrowserPID:   resolved.PID,
		chrome:       resolved,
	}, nil
}

func (s *ChromeSession) Close() {
	if s == nil {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	if s.IsNewBrowser {
		_ = chrome_app.CleanupSession(s.chrome)
	}
}

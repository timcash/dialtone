package test_v2

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type DevBrowserOptions struct {
	URL       string
	LogWriter io.Writer
	LogPrefix string
	Role      string
}

func StartDevBrowser(opts DevBrowserOptions) (*BrowserSession, error) {
	role := opts.Role
	if role == "" {
		role = "dev"
	}
	return StartBrowser(BrowserOptions{
		Headless:      false,
		Role:          role,
		ReuseExisting: true,
		URL:           opts.URL,
		LogWriter:     opts.LogWriter,
		LogPrefix:     opts.LogPrefix,
	})
}

type DevSessionOptions struct {
	VersionDirPath string
	Port           int
	URL            string
	ConsoleWriter  io.Writer
	BrowserRole    string
}

type DevSession struct {
	logFile     *os.File
	logWriter   io.Writer
	url         string
	port        int
	browserRole string

	mu      sync.Mutex
	browser *BrowserSession
}

func NewDevSession(opts DevSessionOptions) (*DevSession, error) {
	if opts.ConsoleWriter == nil {
		opts.ConsoleWriter = os.Stdout
	}
	devLogPath := filepath.Join(opts.VersionDirPath, "dev.log")
	logFile, err := os.Create(devLogPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open dev log: %w", err)
	}

	return &DevSession{
		logFile:   logFile,
		logWriter: io.MultiWriter(opts.ConsoleWriter, logFile),
		url:       opts.URL,
		port:      opts.Port,
		browserRole: func() string {
			if opts.BrowserRole != "" {
				return opts.BrowserRole
			}
			return "dev"
		}(),
	}, nil
}

func (d *DevSession) Writer() io.Writer {
	return d.logWriter
}

func (d *DevSession) StartBrowserAttach() {
	go func() {
		if err := WaitForPort(d.port, 30*time.Second); err != nil {
			fmt.Fprintf(d.logWriter, "   [DEV] Warning: vite server not ready on port %d: %v\n", d.port, err)
			return
		}

		fmt.Fprintf(d.logWriter, "   [DEV] Vite ready at %s\n", d.url)
		fmt.Fprintln(d.logWriter, "   [DEV] Launching debug browser (HEADED) with console capture...")

		s, err := StartDevBrowser(DevBrowserOptions{
			URL:       d.url,
			LogWriter: d.logWriter,
			LogPrefix: "   [BROWSER]",
			Role:      d.browserRole,
		})
		if err != nil {
			fmt.Fprintf(d.logWriter, "   [DEV] Warning: failed to attach debug browser: %v\n", err)
			return
		}

		d.mu.Lock()
		d.browser = s
		d.mu.Unlock()
	}()
}

func (d *DevSession) Close() {
	d.mu.Lock()
	if d.browser != nil {
		d.browser.Close()
	}
	d.mu.Unlock()
	if d.logFile != nil {
		_ = d.logFile.Close()
	}
}

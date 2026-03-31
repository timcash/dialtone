package test

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	uiv1 "dialtone/dev/plugins/ui/src_v1/go"
)

type TestContext struct {
	mu        sync.Mutex
	repoRoot  string
	pluginDir string
	appDir    string
	distDir   string
	server    *http.Server
	serverURL string
	built     bool
	stepCtx   *StepContext
}

var (
	sharedCtxOnce sync.Once
	sharedCtx     *TestContext
)

func SharedContext() *TestContext {
	sharedCtxOnce.Do(func() {
		sharedCtx = NewTestContext()
	})
	return sharedCtx
}

func NewTestContext() *TestContext {
	return &TestContext{}
}

func (t *TestContext) BeginStep(sc *StepContext) {
	t.stepCtx = sc
	t.logf("STEP> begin %s", sc.Name)
}

func (t *TestContext) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.server != nil {
		_ = t.server.Close()
		t.server = nil
	}
}

func (t *TestContext) AppURL(path string) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	base := strings.TrimRight(t.serverURL, "/")
	if base == "" {
		return path
	}
	if path == "" {
		return base
	}
	if strings.HasPrefix(path, "/") {
		return base + path
	}
	return base + "/" + path
}

func (t *TestContext) EnsureBuiltAndServed() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	opts := GetOptions()
	if strings.TrimSpace(opts.AttachNode) != "" {
		return t.ensureAttachDevServerLocked()
	}
	if err := t.ensurePathsLocked(); err != nil {
		return err
	}
	if !t.built {
		if err := t.buildFixtureLocked(); err != nil {
			return err
		}
		t.built = true
	}
	if t.server != nil {
		return nil
	}
	return t.startServerLocked()
}

func (t *TestContext) ensureAttachDevServerLocked() error {
	if err := t.ensurePathsLocked(); err != nil {
		return err
	}
	if !t.built {
		if err := t.buildFixtureLocked(); err != nil {
			return err
		}
		t.built = true
	}
	const attachURL = "http://127.0.0.1:5177"
	t.serverURL = attachURL

	if t.server != nil {
		return nil
	}
	if err := waitHTTP(attachURL+"/health", 1500*time.Millisecond); err == nil {
		t.logf("LOOKING FOR: persistent ui fixture server already running at %s", attachURL)
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/", http.FileServer(http.Dir(t.distDir)))

	ln, err := net.Listen("tcp", "0.0.0.0:5177")
	if err != nil {
		return fmt.Errorf("start attach ui fixture server on %s: %w", attachURL, err)
	}
	t.server = &http.Server{Handler: mux}
	t.logf("LOOKING FOR: starting persistent ui fixture server at %s", attachURL)
	go func() {
		_ = t.server.Serve(ln)
	}()
	if err := waitHTTP(attachURL+"/health", 8*time.Second); err != nil {
		_ = t.server.Close()
		t.server = nil
		return fmt.Errorf("persistent ui fixture server did not become ready at %s: %w", attachURL, err)
	}
	t.logf("LOOKING FOR: persistent ui fixture server ready at %s", attachURL)
	return nil
}

func (t *TestContext) ensurePathsLocked() error {
	if t.repoRoot != "" {
		return nil
	}
	paths, err := uiv1.ResolvePaths("")
	if err != nil {
		return err
	}
	t.repoRoot = paths.Runtime.RepoRoot
	t.pluginDir = paths.PluginVersionRoot
	t.appDir = paths.FixtureApp
	t.distDir = filepath.Join(t.appDir, "dist")
	return nil
}

func (t *TestContext) buildFixtureLocked() error {
	t.logf("LOOKING FOR: ui fixture build at %s", t.appDir)
	if err := t.runCmdLocked(t.appDir, bunBin(), "install", "--silent"); err != nil {
		return fmt.Errorf("bun install failed: %w", err)
	}
	if err := t.runCmdLocked(t.appDir, bunBin(), "run", "build"); err != nil {
		return fmt.Errorf("bun build failed: %w", err)
	}
	if _, err := os.Stat(filepath.Join(t.distDir, "index.html")); err != nil {
		return fmt.Errorf("fixture build output missing index.html: %w", err)
	}
	return nil
}

func (t *TestContext) startServerLocked() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/", http.FileServer(http.Dir(t.distDir)))

	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		return err
	}
	t.server = &http.Server{Handler: mux}
	port := ""
	if tcpAddr, ok := ln.Addr().(*net.TCPAddr); ok && tcpAddr.Port > 0 {
		port = fmt.Sprintf("%d", tcpAddr.Port)
	}
	if port == "" {
		parsed, parseErr := url.Parse("http://" + ln.Addr().String())
		if parseErr == nil {
			port = parsed.Port()
		}
	}
	t.serverURL = "http://127.0.0.1:" + port
	t.logf("LOOKING FOR: go ui backend at %s", t.serverURL)
	go func() {
		_ = t.server.Serve(ln)
	}()
	return waitHTTP(t.serverURL+"/health", 8*time.Second)
}

func (t *TestContext) runCmdLocked(dir string, name string, args ...string) error {
	t.logf("LOOKING FOR: [%s %s]", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (t *TestContext) logf(format string, args ...any) {
	if t.stepCtx != nil {
		t.stepCtx.Infof(format, args...)
		return
	}
}

func waitHTTP(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", url)
}

func bunBin() string {
	candidate := configv1.ManagedBunBinPath(configv1.LookupEnvString("DIALTONE_ENV"))
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return "bun"
}

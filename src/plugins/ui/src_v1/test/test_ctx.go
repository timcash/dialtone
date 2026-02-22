package test

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type TestContext struct {
	mu        sync.Mutex
	repoRoot  string
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

func (t *TestContext) ensurePathsLocked() error {
	if t.repoRoot != "" {
		return nil
	}
	root, err := findRepoRoot()
	if err != nil {
		return err
	}
	t.repoRoot = root
	t.appDir = filepath.Join(root, "src", "plugins", "ui", "src_v1", "test", "fixtures", "app")
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

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	t.server = &http.Server{Handler: mux}
	t.serverURL = "http://" + ln.Addr().String()
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
	envDir := strings.TrimSpace(os.Getenv("DIALTONE_ENV"))
	if envDir == "" {
		return "bun"
	}
	candidate := filepath.Join(envDir, "bun", "bin", "bun")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return "bun"
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", fmt.Errorf("repo root not found")
		}
		cwd = parent
	}
}

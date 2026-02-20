package test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	test_v2 "dialtone/dev/plugins/test/src_v1/go"
)

type testCtx struct {
	sharedBrowser *test_v2.BrowserSession
	baseURL       string
}

func newTestCtx() *testCtx {
	base := strings.TrimSpace(os.Getenv("SIMPLE_TEST_TEST_BASE_URL"))
	if base == "" {
		base = "http://127.0.0.1:3000"
	}
	return &testCtx{
		baseURL: base,
	}
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

func (t *testCtx) ensureDevServer() error {
	if test_v2.WaitForPort(3000, 100*time.Millisecond) == nil {
		return nil
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	uiDir := filepath.Join(repoRoot, "src", "plugins", "simple-test", "src_v1", "ui")
	bunBin := filepath.Join(os.Getenv("DIALTONE_ENV"), "bun", "bin", "bun")
	if _, err := os.Stat(bunBin); err != nil {
		bunBin = "bun"
	}

	cmd := exec.Command(bunBin, "run", "dev", "--host", "127.0.0.1", "--port", "3000", "--strictPort")
	cmd.Dir = uiDir
	if err := cmd.Start(); err != nil {
		return err
	}

	return test_v2.WaitForPort(3000, 30*time.Second)
}

func (t *testCtx) browser() (*test_v2.BrowserSession, error) {
	if t.sharedBrowser != nil {
		return t.sharedBrowser, nil
	}
	if err := t.ensureDevServer(); err != nil {
		return nil, err
	}
	session, err := test_v2.StartBrowser(test_v2.BrowserOptions{
		Headless: true,
		GPU:      true,
		Role:     "test",
		URL:      t.baseURL + "/?test=true",
	})
	if err != nil {
		return nil, err
	}
	t.sharedBrowser = session
	return session, nil
}

func (t *testCtx) teardown() {
	if t.sharedBrowser != nil {
		t.sharedBrowser.Close()
		t.sharedBrowser = nil
	}
}

func (t *testCtx) captureShot(file string) error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	shot := filepath.Join(repoRoot, "src", "plugins", "simple-test", "src_v1", "test", "screenshots", file)
	// Ensure screenshots dir exists
	_ = os.MkdirAll(filepath.Dir(shot), 0755)
	
	if t.sharedBrowser == nil {
		return fmt.Errorf("no browser session")
	}
	return t.sharedBrowser.CaptureScreenshot(shot)
}

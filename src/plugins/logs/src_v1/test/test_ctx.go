package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

type testCtx struct {
	repoRoot    string
	testDir     string
	reportPath  string
	testLogPath string
	errorLog    string

	broker    *logs.EmbeddedNATS
	listeners []func() error
}

func newTestCtx() (*testCtx, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	repoRoot := cwd
	for {
		if _, err := os.Stat(filepath.Join(repoRoot, "dialtone.sh")); err == nil {
			break
		}
		parent := filepath.Dir(repoRoot)
		if parent == repoRoot {
			return nil, fmt.Errorf("repo root not found from %s", cwd)
		}
		repoRoot = parent
	}

	testDir := filepath.Join(repoRoot, "src", "plugins", "logs", "src_v1", "test")
	ctx := &testCtx{
		repoRoot:    repoRoot,
		testDir:     testDir,
		reportPath:  filepath.Join(testDir, "TEST.md"),
		testLogPath: filepath.Join(testDir, "test.log"),
		errorLog:    filepath.Join(testDir, "error.log"),
	}

	for _, p := range []string{ctx.reportPath, ctx.testLogPath, ctx.errorLog} {
		_ = os.Remove(p)
	}
	return ctx, nil
}

func (t *testCtx) ensureBroker() error {
	if t.broker != nil {
		return nil
	}
	b, err := logs.StartEmbeddedNATS()
	if err != nil {
		return err
	}
	t.broker = b
	return nil
}

func (t *testCtx) addListener(stop func() error) {
	t.listeners = append(t.listeners, stop)
}

func (t *testCtx) cleanup() {
	for i := len(t.listeners) - 1; i >= 0; i-- {
		_ = t.listeners[i]()
	}
	t.listeners = nil
	if t.broker != nil {
		t.broker.Close()
		t.broker = nil
	}
}

func (t *testCtx) run(steps []step) error {
	f, err := os.OpenFile(t.reportPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, s := range steps {
		fmt.Printf("Running %s...\n", s.Name)
		report, runErr := s.Run(t)
		if runErr != nil {
			fmt.Printf("%s FAILED: %v\n", s.Name, runErr)
		} else {
			fmt.Printf("%s PASSED\n", s.Name)
		}

		fmt.Fprintf(f, "# %s\n\n", s.Name)
		fmt.Fprintf(f, "### Conditions\n%s\n\n", s.Conditions)
		fmt.Fprintf(f, "### Results\n```text\n")
		if report != "" {
			fmt.Fprintf(f, "%s\n", report)
		}
		if runErr != nil {
			fmt.Fprintf(f, "ERROR: %v\n", runErr)
		}
		fmt.Fprintf(f, "```\n\n")

		if runErr != nil {
			return runErr
		}
	}
	return nil
}

func waitForContains(path, pattern string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), pattern) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %q in %s", pattern, path)
}

func fileContains(path, pattern string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), pattern)
}

func lineCount(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 1 && strings.TrimSpace(lines[0]) == "" {
		return 0
	}
	return len(lines)
}

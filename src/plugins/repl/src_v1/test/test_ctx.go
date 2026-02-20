package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	test_v2 "dialtone/dev/plugins/test/src_v1/go"
)

type testCtx struct {
	repoRoot string
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   *bytes.Buffer
	mu       sync.Mutex
	timeout  time.Duration
}

func newTestCtx() *testCtx {
	cwd, _ := os.Getwd()
	root := cwd
	for {
		if _, err := os.Stat(filepath.Join(root, "dialtone.sh")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			root = cwd
			break
		}
		root = parent
	}
	return &testCtx{repoRoot: root, timeout: 30 * time.Second}
}

func (ctx *testCtx) SetTimeout(d time.Duration) {
	ctx.timeout = d
}

// Cleanup ensures REPL process is killed and subtones are waited for/killed.
func (ctx *testCtx) Cleanup() {
	ctx.Close()
	// Best effort cleanup of any lingering dialtone processes
	_ = test_v2.WaitForAllProcessesToComplete(ctx.repoRoot, 5*time.Second)
}

// runREPL runs the REPL in one-shot mode (legacy)
func (ctx *testCtx) runREPL(input string) (string, error) {
	cmd := exec.Command(filepath.Join(ctx.repoRoot, "dialtone.sh"))
	cmd.Dir = ctx.repoRoot
	
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout
	
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	
	if err := cmd.Start(); err != nil {
		return "", err
	}
	
	_, _ = io.WriteString(stdin, input)
	_ = stdin.Close()
	
	err = cmd.Wait()
	return stdout.String(), err
}

// StartREPL launches the REPL in background for interactive testing
func (ctx *testCtx) StartREPL() error {
	ctx.mu.Lock()
	ctx.stdout = new(bytes.Buffer)
	ctx.mu.Unlock()

	ctx.cmd = exec.Command(filepath.Join(ctx.repoRoot, "dialtone.sh"))
	ctx.cmd.Dir = ctx.repoRoot
	
	var err error
	ctx.stdin, err = ctx.cmd.StdinPipe()
	if err != nil {
		return err
	}
	
	// Capture stdout/stderr to buffer
	ctx.cmd.Stdout = ctx.stdout
	ctx.cmd.Stderr = ctx.stdout
	
	return ctx.cmd.Start()
}

func (ctx *testCtx) SendInput(input string) error {
	if ctx.stdin == nil {
		return fmt.Errorf("REPL not started")
	}
	_, err := io.WriteString(ctx.stdin, input+"\n")
	return err
}

func (ctx *testCtx) Close() error {
	if ctx.cmd != nil && ctx.cmd.Process != nil {
		_ = ctx.cmd.Process.Kill()
	}
	return nil
}

func (ctx *testCtx) WaitForOutput(pattern string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx.mu.Lock()
		output := ctx.stdout.String()
		ctx.mu.Unlock()
		
		if strings.Contains(output, pattern) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	
	// Helper for debug: print last 200 chars
	ctx.mu.Lock()
	out := ctx.stdout.String()
	ctx.mu.Unlock()
	if len(out) > 500 {
		out = "..." + out[len(out)-500:]
	}
	
	return fmt.Errorf("timeout waiting for %q. Output ends with: %q", pattern, out)
}

func (ctx *testCtx) WaitForLogEntry(logPattern, contentPattern string, timeout time.Duration) error {
	logsDir := filepath.Join(ctx.repoRoot, ".dialtone", "logs")
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(logsDir)
		if err == nil {
			for _, entry := range entries {
				if strings.Contains(entry.Name(), logPattern) {
					path := filepath.Join(logsDir, entry.Name())
					content, err := os.ReadFile(path)
					if err == nil && strings.Contains(string(content), contentPattern) {
						return nil
					}
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for log pattern %q containing %q", logPattern, contentPattern)
}

func (ctx *testCtx) runDirect(args ...string) (string, error) {
	cmd := exec.Command(filepath.Join(ctx.repoRoot, "dialtone.sh"), args...)
	cmd.Dir = ctx.repoRoot
	
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout
	
	err := cmd.Run()
	return stdout.String(), err
}

func (ctx *testCtx) WaitProcesses(timeout time.Duration) error {
	return test_v2.WaitForAllProcessesToComplete(ctx.repoRoot, timeout)
}

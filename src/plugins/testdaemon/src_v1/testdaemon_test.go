package testdaemon

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

var (
	buildBinaryOnce sync.Once
	binaryPath      string
	buildBinaryErr  error
)

func TestEmitProgressWritesSharedLog(t *testing.T) {
	home := t.TempDir()
	out, err := runFixture(t, home, "src_v1", "emit-progress", "--steps", "3", "--name", "demo")
	if err != nil {
		t.Fatalf("emit-progress failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "progress 1/3") || !strings.Contains(out, "progress complete") {
		t.Fatalf("emit-progress output missing progress lines:\n%s", out)
	}
	logPath := parseOutputValue(out, "log_path")
	if logPath == "" {
		t.Fatalf("emit-progress output missing log_path:\n%s", out)
	}
	logRaw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log failed: %v", err)
	}
	logText := string(logRaw)
	if !strings.Contains(logText, "progress 2/3") || !strings.Contains(logText, "progress complete") {
		t.Fatalf("shared log missing expected lines:\n%s", logText)
	}
}

func TestServiceStartPauseResumeAndStop(t *testing.T) {
	home := t.TempDir()
	name := "demo"

	startOut, err := runFixture(t, home, "src_v1", "service", "--mode", "start", "--name", name, "--heartbeat-interval", "200ms")
	if err != nil {
		t.Fatalf("service start failed: %v\n%s", err, startOut)
	}
	if !strings.Contains(startOut, "health=healthy") {
		t.Fatalf("service start output missing healthy status:\n%s", startOut)
	}

	statusOut, err := runFixture(t, home, "src_v1", "service", "--mode", "status", "--name", name)
	if err != nil {
		t.Fatalf("service status failed: %v\n%s", err, statusOut)
	}
	if !strings.Contains(statusOut, "running=true") {
		t.Fatalf("service status missing running=true:\n%s", statusOut)
	}

	pauseOut, err := runFixture(t, home, "src_v1", "heartbeat", "--name", name, "--mode", "stop", "--timeout", "3s")
	if err != nil {
		t.Fatalf("heartbeat stop failed: %v\n%s", err, pauseOut)
	}
	if !strings.Contains(pauseOut, "heartbeat_paused=true") {
		t.Fatalf("heartbeat stop missing paused state:\n%s", pauseOut)
	}

	resumeOut, err := runFixture(t, home, "src_v1", "heartbeat", "--name", name, "--mode", "resume", "--timeout", "3s")
	if err != nil {
		t.Fatalf("heartbeat resume failed: %v\n%s", err, resumeOut)
	}
	if !strings.Contains(resumeOut, "health=healthy") || strings.Contains(resumeOut, "heartbeat_paused=true") {
		t.Fatalf("heartbeat resume missing healthy state:\n%s", resumeOut)
	}

	stopOut, err := runFixture(t, home, "src_v1", "service", "--mode", "stop", "--name", name, "--timeout", "5s")
	if err != nil {
		t.Fatalf("service stop failed: %v\n%s", err, stopOut)
	}
	if !strings.Contains(stopOut, "running=false") {
		t.Fatalf("service stop missing stopped state:\n%s", stopOut)
	}
}

func TestExitCodeCommandReturnsRequestedCode(t *testing.T) {
	home := t.TempDir()
	out, err := runFixture(t, home, "src_v1", "exit-code", "--code", "17")
	if err == nil {
		t.Fatalf("exit-code unexpectedly succeeded:\n%s", out)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("exit-code returned non-exit error: %v", err)
	}
	if exitErr.ExitCode() != 17 {
		t.Fatalf("exit-code returned %d, want 17\n%s", exitErr.ExitCode(), out)
	}
}

func TestPanicAndHangCommandsFailAsExpected(t *testing.T) {
	home := t.TempDir()

	panicOut, panicErr := runFixture(t, home, "src_v1", "panic")
	if panicErr == nil {
		t.Fatalf("panic command unexpectedly succeeded:\n%s", panicOut)
	}
	if !strings.Contains(strings.ToLower(panicOut), "panic requested") {
		t.Fatalf("panic output missing panic marker:\n%s", panicOut)
	}

	cmd := exec.Command(fixtureBinary(t), "src_v1", "hang")
	cmd.Env = fixtureEnv(t, home)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		t.Fatalf("hang start failed: %v", err)
	}
	time.Sleep(750 * time.Millisecond)
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		t.Fatalf("hang command exited too early:\n%s", out.String())
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	if !strings.Contains(out.String(), "hang requested") {
		t.Fatalf("hang output missing hang marker:\n%s", out.String())
	}
}

func fixtureBinary(t *testing.T) string {
	t.Helper()
	buildBinaryOnce.Do(func() {
		repoRoot, err := configv1.FindRepoRoot("")
		if err != nil {
			buildBinaryErr = err
			return
		}
		tmpDir, err := os.MkdirTemp("", "testdaemon-binary-*")
		if err != nil {
			buildBinaryErr = err
			return
		}
		name := "testdaemon-fixture"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		binaryPath = filepath.Join(tmpDir, name)
		cmd := exec.Command("go", "build", "-o", binaryPath, "./plugins/testdaemon/scaffold")
		cmd.Dir = filepath.Join(repoRoot, "src")
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			buildBinaryErr = errors.New(out.String())
		}
	})
	if buildBinaryErr != nil {
		t.Fatalf("build fixture binary failed: %v", buildBinaryErr)
	}
	return binaryPath
}

func runFixture(t *testing.T, home string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(fixtureBinary(t), args...)
	cmd.Env = fixtureEnv(t, home)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func fixtureEnv(t *testing.T, home string) []string {
	t.Helper()
	repoRoot, err := configv1.FindRepoRoot("")
	if err != nil {
		t.Fatalf("resolve repo root failed: %v", err)
	}
	return append(os.Environ(),
		"DIALTONE_USE_NIX=0",
		"DIALTONE_REPO_ROOT="+repoRoot,
		"DIALTONE_SRC_ROOT="+filepath.Join(repoRoot, "src"),
		"DIALTONE_ENV_FILE="+filepath.Join(repoRoot, "env", "dialtone.json"),
		"DIALTONE_HOME="+filepath.Join(home, ".dialtone"),
		"DIALTONE_ENV="+filepath.Join(home, ".dialtone_env"),
	)
}

func parseOutputValue(output string, key string) string {
	prefix := "testdaemon> " + strings.TrimSpace(key) + "="
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

package selfcheck

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name: "go-cli-stdout-propagates",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			repoRoot, err := findRepoRoot()
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			marker := "GO_STDOUT_MARKER_42"
			out, err := runDialtone(repoRoot, "go", "src_v1", "exec", "run", "./plugins/go/test/fixtures/stdout/main.go", marker)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("expected success from go stdout command: %w\noutput:\n%s", err, out)
			}
			if !strings.Contains(out, marker) {
				return testv1.StepRunResult{}, fmt.Errorf("stdout marker missing from output: %s", marker)
			}
			if err := ctx.WaitForStepMessageAfterAction("go-stdout-ok", 3*time.Second, func() error {
				ctx.Infof("go-stdout-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "go stdout propagation verified"}, nil
		},
	})

	r.Add(testv1.Step{
		Name: "go-cli-stderr-exit-propagates",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			repoRoot, err := findRepoRoot()
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			stderrMarker := "GO_STDERR_MARKER_42"
			out, err := runDialtone(repoRoot, "go", "src_v1", "exec", "run", "./plugins/go/test/fixtures/stderr_exit/main.go", stderrMarker, "17")
			if err == nil {
				return testv1.StepRunResult{}, fmt.Errorf("expected non-zero exit from go stderr/exit command\noutput:\n%s", out)
			}
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				return testv1.StepRunResult{}, fmt.Errorf("expected ExitError, got %T: %v", err, err)
			}
			if exitErr.ExitCode() == 0 {
				return testv1.StepRunResult{}, fmt.Errorf("expected non-zero wrapper exit code")
			}
			if !strings.Contains(out, stderrMarker) {
				return testv1.StepRunResult{}, fmt.Errorf("stderr marker missing from output: %s", stderrMarker)
			}
			if !strings.Contains(out, "exit status 17") {
				return testv1.StepRunResult{}, fmt.Errorf("expected underlying go exit status in output")
			}
			if err := ctx.WaitForStepMessageAfterAction("go-stderr-exit-ok", 3*time.Second, func() error {
				ctx.Infof("go-stderr-exit-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "go stderr and non-zero exit propagation verified"}, nil
		},
	})
}

func runDialtone(repoRoot string, args ...string) (string, error) {
	dialtoneSh := filepath.Join(repoRoot, "dialtone.sh")
	cmd := exec.Command(dialtoneSh, args...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	return string(out), err
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

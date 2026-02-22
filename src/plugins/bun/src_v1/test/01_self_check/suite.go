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
		Name: "bun-cli-stdout-propagates",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			repoRoot, err := findRepoRoot()
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			marker := "BUN_STDOUT_MARKER_42"
			out, err := runDialtone(repoRoot, "bun", "src_v1", "exec", "--eval", fmt.Sprintf("console.log(%q)", marker))
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("expected success from bun stdout command: %w\noutput:\n%s", err, out)
			}
			if !strings.Contains(out, marker) {
				return testv1.StepRunResult{}, fmt.Errorf("stdout marker missing from output: %s", marker)
			}
			if err := ctx.WaitForStepMessageAfterAction("bun-stdout-ok", 3*time.Second, func() error {
				ctx.Infof("bun-stdout-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "bun stdout propagation verified"}, nil
		},
	})

	r.Add(testv1.Step{
		Name: "bun-cli-stderr-exit-propagates",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			repoRoot, err := findRepoRoot()
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			stderrMarker := "BUN_STDERR_MARKER_42"
			out, err := runDialtone(repoRoot, "bun", "src_v1", "exec", "--eval", fmt.Sprintf("console.error(%q); process.exit(23)", stderrMarker))
			if err == nil {
				return testv1.StepRunResult{}, fmt.Errorf("expected non-zero exit from bun stderr/exit command\noutput:\n%s", out)
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
			if err := ctx.WaitForStepMessageAfterAction("bun-stderr-exit-ok", 3*time.Second, func() error {
				ctx.Infof("bun-stderr-exit-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "bun stderr and non-zero exit propagation verified"}, nil
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

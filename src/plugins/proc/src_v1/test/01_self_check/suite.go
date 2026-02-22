package selfcheck

import (
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
		Name: "proc-emit-src-v1",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			repoRoot, err := findRepoRoot()
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			marker := "PROC_EMIT_MARKER_42"
			out, err := runDialtone(repoRoot, "proc", "src_v1", "emit", marker)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("proc emit failed: %w\noutput:\n%s", err, out)
			}
			if !strings.Contains(out, marker) {
				return testv1.StepRunResult{}, fmt.Errorf("missing marker in proc emit output")
			}
			if err := ctx.WaitForStepMessageAfterAction("proc-emit-src-v1-ok", 3*time.Second, func() error {
				ctx.Infof("proc-emit-src-v1-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "proc emit src_v1 command verified"}, nil
		},
	})

	r.Add(testv1.Step{
		Name: "proc-old-order-warns",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			repoRoot, err := findRepoRoot()
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			out, err := runDialtone(repoRoot, "proc", "emit", "src_v1", "OLD_ORDER_MARKER")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("proc old-order command failed: %w\noutput:\n%s", err, out)
			}
			if !strings.Contains(out, "old proc CLI order is deprecated") {
				return testv1.StepRunResult{}, fmt.Errorf("expected old-order warning in output")
			}
			if !strings.Contains(out, "OLD_ORDER_MARKER") {
				return testv1.StepRunResult{}, fmt.Errorf("expected emitted marker in old-order output")
			}
			if err := ctx.WaitForStepMessageAfterAction("proc-old-order-warns-ok", 3*time.Second, func() error {
				ctx.Infof("proc-old-order-warns-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "proc old-order compatibility warning verified"}, nil
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

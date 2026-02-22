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
		Name: "worktree-help-src-v1",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			repoRoot, err := findRepoRoot()
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			out, err := runDialtone(repoRoot, "worktree", "src_v1", "help")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("worktree help failed: %w\noutput:\n%s", err, out)
			}
			needles := []string{
				"Usage: ./dialtone.sh worktree src_v1 <command> [args]",
				"add <name> [--task <file>] [--branch <branch>]",
				"test",
			}
			for _, needle := range needles {
				if !strings.Contains(out, needle) {
					return testv1.StepRunResult{}, fmt.Errorf("missing help output line: %q", needle)
				}
			}
			if err := ctx.WaitForStepMessageAfterAction("worktree-help-src-v1-ok", 3*time.Second, func() error {
				ctx.Infof("worktree-help-src-v1-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "worktree src_v1 help output verified"}, nil
		},
	})

	r.Add(testv1.Step{
		Name: "worktree-old-order-warning",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			repoRoot, err := findRepoRoot()
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			out, err := runDialtone(repoRoot, "worktree", "list", "src_v1")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("worktree old-order list failed: %w\noutput:\n%s", err, out)
			}
			if !strings.Contains(out, "old worktree CLI order is deprecated") {
				return testv1.StepRunResult{}, fmt.Errorf("expected old-order warning, got output:\n%s", out)
			}
			if err := ctx.WaitForStepMessageAfterAction("worktree-old-order-warning-ok", 3*time.Second, func() error {
				ctx.Infof("worktree-old-order-warning-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "worktree old-order warning verified"}, nil
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

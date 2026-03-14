package tmpworkspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "tmp-bootstrap-workspace",
		Timeout: 30 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			repoRoot := strings.TrimSpace(ctx.RepoRoot())
			if strings.HasSuffix(filepath.ToSlash(repoRoot), "/src") {
				repoRoot = filepath.Dir(repoRoot)
			}
			repoRootClean := filepath.Clean(repoRoot)
			repoRootReal := repoRootClean
			if resolved, err := filepath.EvalSymlinks(repoRootClean); err == nil && strings.TrimSpace(resolved) != "" {
				repoRootReal = filepath.Clean(resolved)
			}
			tmpRoot := filepath.Clean(os.TempDir())
			tmpRootReal := tmpRoot
			if resolved, err := filepath.EvalSymlinks(tmpRoot); err == nil && strings.TrimSpace(resolved) != "" {
				tmpRootReal = filepath.Clean(resolved)
			}
			tmpAliases := []string{
				filepath.Clean("/tmp"),
				filepath.Clean("/private/tmp"),
			}
			isUnderTmp := strings.HasPrefix(repoRootClean, tmpRoot+string(os.PathSeparator)) ||
				strings.HasPrefix(repoRootReal, tmpRootReal+string(os.PathSeparator))
			if !isUnderTmp {
				for _, alias := range tmpAliases {
					if strings.HasPrefix(repoRootClean, alias+string(os.PathSeparator)) ||
						strings.HasPrefix(repoRootReal, alias+string(os.PathSeparator)) {
						isUnderTmp = true
						break
					}
				}
			}
			if !isUnderTmp {
				return testv1.StepRunResult{}, fmt.Errorf("expected tmp workspace under %s (resolved %s) or /tmp aliases, got %s (resolved %s)", tmpRoot, tmpRootReal, repoRootClean, repoRootReal)
			}
			required := []string{
				filepath.Join(repoRoot, "dialtone.sh"),
				filepath.Join(repoRoot, "src", "dev.go"),
				filepath.Join(repoRoot, "env", "dialtone.json"),
			}
			for _, p := range required {
				if _, err := os.Stat(p); err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("missing required bootstrap file %s: %w", p, err)
				}
			}
			ctx.TestPassf("tmp workspace is active at %s", repoRoot)
			return testv1.StepRunResult{
				Report: "Verified ./dialtone.sh repl src_v3 test runs from a bootstrap workspace under /tmp with dialtone.sh, src, and env configuration materialized.",
			}, nil
		},
	})
}

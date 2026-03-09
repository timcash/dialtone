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
			tmpRoot := filepath.Clean(os.TempDir())
			if !strings.HasPrefix(filepath.Clean(repoRoot), tmpRoot+string(os.PathSeparator)) {
				return testv1.StepRunResult{}, fmt.Errorf("expected tmp workspace under %s, got %s", tmpRoot, repoRoot)
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
				Report: "Verified ./dialtone.sh --test runs from a bootstrap workspace under /tmp with dialtone.sh, src, and env configuration materialized.",
			}, nil
		},
	})
}

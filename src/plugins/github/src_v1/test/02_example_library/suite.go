package examplelibrary

import (
	"fmt"
	"strings"
	"time"

	githubv1 "dialtone/dev/plugins/github/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name: "example-library-render-smoke",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			doc := githubv1.RenderIssueTaskMarkdown(githubv1.Issue{
				Number: 42,
				Title:  "Example issue for task conversion",
				Body:   "Implement minimal sync command",
				State:  "open",
				URL:    "https://github.com/example/repo/issues/42",
				Labels: []githubv1.GHLabel{{Name: "automation"}},
			}, githubv1.RenderOptions{})
			if !strings.Contains(doc, "- status: wait") {
				return testv1.StepRunResult{}, fmt.Errorf("missing wait status")
			}
			if err := ctx.WaitForStepMessageAfterAction("example-library-ok", 3*time.Second, func() error {
				ctx.Infof("example-library-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "example library render verified"}, nil
		},
	})
}

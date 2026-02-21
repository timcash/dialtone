package selfcheck

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	githubv1 "dialtone/dev/plugins/github/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name: "render-markdown-has-wait-status",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			doc := githubv1.RenderIssueTaskMarkdown(githubv1.Issue{
				Number: 7,
				Title:  "Fix CI flake",
				Body:   "CI flakes on linux",
				State:  "open",
			}, githubv1.RenderOptions{})
			if !strings.Contains(doc, "- status: wait") {
				return testv1.StepRunResult{}, fmt.Errorf("task markdown missing wait status")
			}
			if err := ctx.WaitForStepMessageAfterAction("wait-status-ok", 3*time.Second, func() error {
				ctx.Infof("wait-status-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "wait status verified"}, nil
		},
	})

	r.Add(testv1.Step{
		Name: "write-task-file-shape",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			repoRoot, err := findRepoRoot()
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			tmpDir := filepath.Join(repoRoot, "src", "plugins", "github", "src_v1", "test", "tmp")
			_ = os.MkdirAll(tmpDir, 0o755)
			path := filepath.Join(tmpDir, "999.md")

			doc := githubv1.RenderIssueTaskMarkdown(githubv1.Issue{
				Number: 999,
				Title:  "Downloaded issue",
				Body:   "Task body",
				State:  "open",
			}, githubv1.RenderOptions{})
			if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
				return testv1.StepRunResult{}, err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			text := string(data)
			if !strings.Contains(text, "### signature:") || !strings.Contains(text, "- status: wait") {
				return testv1.StepRunResult{}, fmt.Errorf("missing signature/wait in file")
			}
			if err := ctx.WaitForStepMessageAfterAction("task-file-shape-ok", 3*time.Second, func() error {
				ctx.Infof("task-file-shape-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "task file shape verified"}, nil
		},
	})
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

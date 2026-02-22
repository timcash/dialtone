package smoke

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
	var tmpTasksDir string
	var tmpIssuesDir string
	var repoRoot string
	var srcDir string
	var taskToolRel string

	r.Add(testv1.Step{
		Name: "setup-test-env",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			var err error
			tmpTasksDir, err = os.MkdirTemp("", "dialtone-tasks-test-*")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			tmpIssuesDir, err = os.MkdirTemp("", "dialtone-issues-test-*")
			if err != nil {
				return testv1.StepRunResult{}, err
			}

			cwd, _ := os.Getwd()
			repoRoot = cwd
			if filepath.Base(cwd) == "src" {
				repoRoot = filepath.Dir(cwd)
			}
			srcDir = filepath.Join(repoRoot, "src")
			taskToolRel = "./plugins/task/src_v1/go/main.go"

			if err := ctx.WaitForStepMessageAfterAction("setup-test-env-ready", 3*time.Second, func() error {
				ctx.Infof("setup-test-env-ready")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "Test environment initialized"}, nil
		},
	})

	r.Add(testv1.Step{
		Name: "sync-issue-to-root-and-input-tree",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			issueMD := `# mock-issue-title
### signature:
- status: wait
- issue: 777
- source: github
- url: https://github.com/example/repo/issues/777
- synced-at: 2026-02-21T00:00:00Z
### sync:
- github-updated-at: 2026-02-21T00:00:00Z
- last-pulled-at: 2026-02-21T00:00:00Z
- last-pushed-at:
- github-labels-hash:
### description:
- root task from issue
### tags:
- task
### comments-github:
- none
### comments-outbound:
- TODO: add a bullet comment here to post to GitHub
### task-dependencies:
- dep-a
- dep-b
### documentation:
- none
### test-condition-1:
- root test passes
### test-command:
- go test ./...
### reviewed:
- none
### tested:
- none
### last-error-types:
- none
### last-error-times:
- none
### log-stream-command:
- TODO
### last-error-loglines:
- none
### notes:
- none
`
			if err := os.WriteFile(filepath.Join(tmpIssuesDir, "777.md"), []byte(issueMD), 0o644); err != nil {
				return testv1.StepRunResult{}, err
			}

			cmd := exec.Command("go", "run", taskToolRel, "--tasks-dir", tmpTasksDir, "--issues-dir", tmpIssuesDir, "sync", "777")
			cmd.Dir = srcDir
			if out, err := cmd.CombinedOutput(); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("sync failed: %v, output: %s", err, string(out))
			}

			rootPath := filepath.Join(tmpTasksDir, "777", "v2", "root.md")
			rootContent, _ := os.ReadFile(rootPath)
			rootStr := string(rootContent)
			if !strings.Contains(rootStr, "### issue:\n- [#777](https://github.com/example/repo/issues/777)") {
				return testv1.StepRunResult{}, fmt.Errorf("root missing issue link")
			}
			if !strings.Contains(rootStr, "### pr:\n- none") {
				return testv1.StepRunResult{}, fmt.Errorf("root missing pr placeholder")
			}
			if !strings.Contains(rootStr, "### signatures:\n- none") {
				return testv1.StepRunResult{}, fmt.Errorf("root missing signatures placeholder")
			}
			if !strings.Contains(rootStr, "### outputs:\n- none") {
				return testv1.StepRunResult{}, fmt.Errorf("root outputs must be none")
			}
			if !strings.Contains(rootStr, "- [dep-a](../../dep-a/v2/root.md)") || !strings.Contains(rootStr, "- [dep-b](../../dep-b/v2/root.md)") {
				return testv1.StepRunResult{}, fmt.Errorf("root missing dependency input links")
			}

			for _, dep := range []string{"dep-a", "dep-b"} {
				depPath := filepath.Join(tmpTasksDir, dep, "v2", "root.md")
				depContent, _ := os.ReadFile(depPath)
				if !strings.Contains(string(depContent), "- [777](../../777/v2/root.md)") {
					return testv1.StepRunResult{}, fmt.Errorf("%s missing output link to root", dep)
				}
			}

			if err := ctx.WaitForStepMessageAfterAction("task-sync-ok", 3*time.Second, func() error {
				ctx.Infof("task-sync-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "Issue sync created root links and dependency tree"}, nil
		},
	})

	r.Add(testv1.Step{
		Name: "resolve-root-after-inputs-done",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			for _, task := range []string{"dep-a", "dep-b", "777"} {
				cmd := exec.Command("go", "run", taskToolRel, "--tasks-dir", tmpTasksDir, "sign", task, "--role", "TEST")
				cmd.Dir = srcDir
				if out, err := cmd.CombinedOutput(); err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("sign TEST failed for %s: %v, output: %s", task, err, string(out))
				}
				cmd = exec.Command("go", "run", taskToolRel, "--tasks-dir", tmpTasksDir, "sign", task, "--role", "REVIEW")
				cmd.Dir = srcDir
				if out, err := cmd.CombinedOutput(); err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("sign REVIEW failed for %s: %v, output: %s", task, err, string(out))
				}
			}

			cmd := exec.Command("go", "run", taskToolRel, "--tasks-dir", tmpTasksDir, "--issues-dir", tmpIssuesDir, "resolve", "777", "--pr-url", "https://github.com/example/repo/pull/999")
			cmd.Dir = srcDir
			if out, err := cmd.CombinedOutput(); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("resolve failed: %v, output: %s", err, string(out))
			}

			rootPath := filepath.Join(tmpTasksDir, "777", "v2", "root.md")
			rootContent, _ := os.ReadFile(rootPath)
			rootStr := string(rootContent)
			if !strings.Contains(rootStr, "- status: done") {
				return testv1.StepRunResult{}, fmt.Errorf("root signature status not set to done")
			}
			if !strings.Contains(rootStr, "### pr:\n- [PR](https://github.com/example/repo/pull/999)") {
				return testv1.StepRunResult{}, fmt.Errorf("root pr link not set")
			}

			issuePath := filepath.Join(tmpIssuesDir, "777.md")
			issueContent, _ := os.ReadFile(issuePath)
			issueStr := string(issueContent)
			if !strings.Contains(issueStr, "- status: done") {
				return testv1.StepRunResult{}, fmt.Errorf("issue status not updated to done")
			}
			if !strings.Contains(issueStr, "task 777 resolved via task plugin") {
				return testv1.StepRunResult{}, fmt.Errorf("issue outbound completion comment not written")
			}

			if err := ctx.WaitForStepMessageAfterAction("task-resolve-ok", 3*time.Second, func() error {
				ctx.Infof("task-resolve-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "Resolve flow completed and synced back to issue markdown"}, nil
		},
	})

	r.Add(testv1.Step{
		Name: "multi-link-syntax-chain-and-list",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			for _, name := range []string{"n1", "n2", "n3", "n4"} {
				cmd := exec.Command("go", "run", taskToolRel, "--tasks-dir", tmpTasksDir, "create", name)
				cmd.Dir = srcDir
				if out, err := cmd.CombinedOutput(); err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("create %s failed: %v, output: %s", name, err, string(out))
				}
			}

			cmd := exec.Command("go", "run", taskToolRel, "--tasks-dir", tmpTasksDir, "link", "n1-->n2-->n3,n3-->n4")
			cmd.Dir = srcDir
			if out, err := cmd.CombinedOutput(); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("multi-link failed: %v, output: %s", err, string(out))
			}

			n3Path := filepath.Join(tmpTasksDir, "n3", "v2", "root.md")
			n3Content, _ := os.ReadFile(n3Path)
			n3Str := string(n3Content)
			if !strings.Contains(n3Str, "- [n2](../../n2/v2/root.md)") {
				return testv1.StepRunResult{}, fmt.Errorf("n3 missing input from n2")
			}
			if !strings.Contains(n3Str, "- [n4](../../n4/v2/root.md)") {
				return testv1.StepRunResult{}, fmt.Errorf("n3 missing output to n4")
			}
			if err := ctx.WaitForStepMessageAfterAction("task-multilink-ok", 3*time.Second, func() error {
				ctx.Infof("task-multilink-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "Multi-link syntax works for chain/list"}, nil
		},
	})

	r.Add(testv1.Step{
		Name: "link-and-unlink-roundtrip",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			for _, name := range []string{"u1", "u2"} {
				cmd := exec.Command("go", "run", taskToolRel, "--tasks-dir", tmpTasksDir, "create", name)
				cmd.Dir = srcDir
				if out, err := cmd.CombinedOutput(); err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("create %s failed: %v, output: %s", name, err, string(out))
				}
			}

			if err := ctx.WaitForStepMessageAfterAction("task-link-unlink-linked", 3*time.Second, func() error {
				cmd := exec.Command("go", "run", taskToolRel, "--tasks-dir", tmpTasksDir, "link", "u1<--u2")
				cmd.Dir = srcDir
				if out, err := cmd.CombinedOutput(); err != nil {
					return fmt.Errorf("link failed: %v, output: %s", err, string(out))
				}
				ctx.Infof("task-link-unlink-linked")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			u1Path := filepath.Join(tmpTasksDir, "u1", "v2", "root.md")
			u2Path := filepath.Join(tmpTasksDir, "u2", "v2", "root.md")
			u1Text, _ := os.ReadFile(u1Path)
			u2Text, _ := os.ReadFile(u2Path)
			if !strings.Contains(string(u1Text), "- [u2](../../u2/v2/root.md)") {
				return testv1.StepRunResult{}, fmt.Errorf("u1 missing input link to u2")
			}
			if !strings.Contains(string(u2Text), "- [u1](../../u1/v2/root.md)") {
				return testv1.StepRunResult{}, fmt.Errorf("u2 missing output link to u1")
			}

			if err := ctx.WaitForStepMessageAfterAction("task-link-unlink-unlinked", 3*time.Second, func() error {
				cmd := exec.Command("go", "run", taskToolRel, "--tasks-dir", tmpTasksDir, "unlink", "u1", "u2")
				cmd.Dir = srcDir
				if out, err := cmd.CombinedOutput(); err != nil {
					return fmt.Errorf("unlink failed: %v, output: %s", err, string(out))
				}
				ctx.Infof("task-link-unlink-unlinked")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			u1Text, _ = os.ReadFile(u1Path)
			u2Text, _ = os.ReadFile(u2Path)
			if !strings.Contains(string(u1Text), "### inputs:\n- none") {
				return testv1.StepRunResult{}, fmt.Errorf("u1 inputs did not reset to none after unlink")
			}
			if !strings.Contains(string(u2Text), "### outputs:\n- none") {
				return testv1.StepRunResult{}, fmt.Errorf("u2 outputs did not reset to none after unlink")
			}
			return testv1.StepRunResult{Report: "link/unlink roundtrip verified"}, nil
		},
	})

	r.Add(testv1.Step{
		Name: "signing-roles-review-test-docs",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			roleTask := "role-demo"
			cmd := exec.Command("go", "run", taskToolRel, "--tasks-dir", tmpTasksDir, "create", roleTask)
			cmd.Dir = srcDir
			if out, err := cmd.CombinedOutput(); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("create %s failed: %v, output: %s", roleTask, err, string(out))
			}

			for _, role := range []string{"TEST", "REVIEW", "DOCS"} {
				role := role
				if err := ctx.WaitForStepMessageAfterAction("task-sign-role-"+strings.ToLower(role), 3*time.Second, func() error {
					cmd := exec.Command("go", "run", taskToolRel, "--tasks-dir", tmpTasksDir, "sign", roleTask, "--role", role)
					cmd.Dir = srcDir
					if out, err := cmd.CombinedOutput(); err != nil {
						return fmt.Errorf("sign role %s failed: %v, output: %s", role, err, string(out))
					}
					ctx.Infof("task-sign-role-%s", strings.ToLower(role))
					return nil
				}); err != nil {
					return testv1.StepRunResult{}, err
				}
			}

			rolePath := filepath.Join(tmpTasksDir, roleTask, "v2", "root.md")
			roleMD, _ := os.ReadFile(rolePath)
			text := string(roleMD)
			if !strings.Contains(text, "### tested:") || !strings.Contains(text, "TEST>") {
				return testv1.StepRunResult{}, fmt.Errorf("TEST signature missing from tested section")
			}
			if !strings.Contains(text, "### reviewed:") || !strings.Contains(text, "REVIEW>") {
				return testv1.StepRunResult{}, fmt.Errorf("REVIEW signature missing from reviewed section")
			}
			if !strings.Contains(text, "### signatures:") || !strings.Contains(text, "TEST>") || !strings.Contains(text, "REVIEW>") || !strings.Contains(text, "DOCS>") {
				return testv1.StepRunResult{}, fmt.Errorf("signatures section missing required role signatures")
			}
			return testv1.StepRunResult{Report: "REVIEW/TEST/DOCS signing behavior verified"}, nil
		},
	})

	r.Add(testv1.Step{
		Name: "cleanup",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if tmpTasksDir != "" {
				_ = os.RemoveAll(tmpTasksDir)
			}
			if tmpIssuesDir != "" {
				_ = os.RemoveAll(tmpIssuesDir)
			}
			return testv1.StepRunResult{Report: "Cleaned up temp directories"}, nil
		},
	})
}

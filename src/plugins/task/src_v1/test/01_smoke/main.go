package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	var tmpTasksDir string
	var tmpIssuesDir string
	var repoRoot string
	var srcDir string
	var taskToolRel string

	steps := []testv1.Step{
		{
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
				ctx.Logf("Created temp tasks dir: %s", tmpTasksDir)
				ctx.Logf("Created temp issues dir: %s", tmpIssuesDir)

				cwd, _ := os.Getwd()
				repoRoot = cwd
				if filepath.Base(cwd) == "src" {
					repoRoot = filepath.Dir(cwd)
				}
				srcDir = filepath.Join(repoRoot, "src")
				taskToolRel = "./plugins/task/src_v1/go/main.go"

				return testv1.StepRunResult{Report: "Test environment initialized"}, nil
			},
		},
		{
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
					return testv1.StepRunResult{}, logs.Errorf("sync failed: %v, output: %s", err, string(out))
				}

				rootPath := filepath.Join(tmpTasksDir, "777", "v2", "root.md")
				rootContent, _ := os.ReadFile(rootPath)
				rootStr := string(rootContent)
				if !strings.Contains(rootStr, "### issue:\n- [#777](https://github.com/example/repo/issues/777)") {
					return testv1.StepRunResult{}, logs.Errorf("root missing issue link: %s", rootStr)
				}
				if !strings.Contains(rootStr, "### pr:\n- none") {
					return testv1.StepRunResult{}, logs.Errorf("root missing pr placeholder: %s", rootStr)
				}
				if !strings.Contains(rootStr, "### outputs:\n- none") {
					return testv1.StepRunResult{}, logs.Errorf("root outputs must be none: %s", rootStr)
				}
				if !strings.Contains(rootStr, "- [dep-a](../../dep-a/v2/root.md)") || !strings.Contains(rootStr, "- [dep-b](../../dep-b/v2/root.md)") {
					return testv1.StepRunResult{}, logs.Errorf("root missing dependency input links: %s", rootStr)
				}

				for _, dep := range []string{"dep-a", "dep-b"} {
					depPath := filepath.Join(tmpTasksDir, dep, "v2", "root.md")
					depContent, _ := os.ReadFile(depPath)
					if !strings.Contains(string(depContent), "- [777](../../777/v2/root.md)") {
						return testv1.StepRunResult{}, logs.Errorf("%s missing output link to root: %s", dep, string(depContent))
					}
				}

				return testv1.StepRunResult{Report: "Issue sync created root links and dependency tree"}, nil
			},
		},
		{
			Name: "resolve-root-after-inputs-done",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				for _, task := range []string{"dep-a", "dep-b", "777"} {
					cmd := exec.Command("go", "run", taskToolRel, "--tasks-dir", tmpTasksDir, "sign", task, "--role", "TEST")
					cmd.Dir = srcDir
					if out, err := cmd.CombinedOutput(); err != nil {
						return testv1.StepRunResult{}, logs.Errorf("sign TEST failed for %s: %v, output: %s", task, err, string(out))
					}
					cmd = exec.Command("go", "run", taskToolRel, "--tasks-dir", tmpTasksDir, "sign", task, "--role", "REVIEW")
					cmd.Dir = srcDir
					if out, err := cmd.CombinedOutput(); err != nil {
						return testv1.StepRunResult{}, logs.Errorf("sign REVIEW failed for %s: %v, output: %s", task, err, string(out))
					}
				}

				cmd := exec.Command("go", "run", taskToolRel, "--tasks-dir", tmpTasksDir, "--issues-dir", tmpIssuesDir, "resolve", "777", "--pr-url", "https://github.com/example/repo/pull/999")
				cmd.Dir = srcDir
				if out, err := cmd.CombinedOutput(); err != nil {
					return testv1.StepRunResult{}, logs.Errorf("resolve failed: %v, output: %s", err, string(out))
				}

				rootPath := filepath.Join(tmpTasksDir, "777", "v2", "root.md")
				rootContent, _ := os.ReadFile(rootPath)
				rootStr := string(rootContent)
				if !strings.Contains(rootStr, "- status: done") {
					return testv1.StepRunResult{}, logs.Errorf("root signature status not set to done: %s", rootStr)
				}
				if !strings.Contains(rootStr, "### pr:\n- [PR](https://github.com/example/repo/pull/999)") {
					return testv1.StepRunResult{}, logs.Errorf("root pr link not set: %s", rootStr)
				}

				issuePath := filepath.Join(tmpIssuesDir, "777.md")
				issueContent, _ := os.ReadFile(issuePath)
				issueStr := string(issueContent)
				if !strings.Contains(issueStr, "- status: done") {
					return testv1.StepRunResult{}, logs.Errorf("issue status not updated to done: %s", issueStr)
				}
				if !strings.Contains(issueStr, "task 777 resolved via task plugin") {
					return testv1.StepRunResult{}, logs.Errorf("issue outbound completion comment not written: %s", issueStr)
				}

				return testv1.StepRunResult{Report: "Resolve flow completed and synced back to issue markdown"}, nil
			},
		},
		{
			Name: "multi-link-syntax-chain-and-list",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				for _, name := range []string{"n1", "n2", "n3", "n4"} {
					cmd := exec.Command("go", "run", taskToolRel, "--tasks-dir", tmpTasksDir, "create", name)
					cmd.Dir = srcDir
					if out, err := cmd.CombinedOutput(); err != nil {
						return testv1.StepRunResult{}, logs.Errorf("create %s failed: %v, output: %s", name, err, string(out))
					}
				}

				cmd := exec.Command("go", "run", taskToolRel, "--tasks-dir", tmpTasksDir, "link", "n1-->n2-->n3,n3-->n4")
				cmd.Dir = srcDir
				if out, err := cmd.CombinedOutput(); err != nil {
					return testv1.StepRunResult{}, logs.Errorf("multi-link failed: %v, output: %s", err, string(out))
				}

				n3Path := filepath.Join(tmpTasksDir, "n3", "v2", "root.md")
				n3Content, _ := os.ReadFile(n3Path)
				n3Str := string(n3Content)
				if !strings.Contains(n3Str, "- [n2](../../n2/v2/root.md)") {
					return testv1.StepRunResult{}, logs.Errorf("n3 missing input from n2: %s", n3Str)
				}
				if !strings.Contains(n3Str, "- [n4](../../n4/v2/root.md)") {
					return testv1.StepRunResult{}, logs.Errorf("n3 missing output to n4: %s", n3Str)
				}
				return testv1.StepRunResult{Report: "Multi-link syntax works for chain/list"}, nil
			},
		},
		{
			Name: "cleanup",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				if tmpTasksDir != "" {
					os.RemoveAll(tmpTasksDir)
				}
				if tmpIssuesDir != "" {
					os.RemoveAll(tmpIssuesDir)
				}
				return testv1.StepRunResult{Report: "Cleaned up temp directories"}, nil
			},
		},
	}

	err := testv1.RunSuite(testv1.SuiteOptions{
		Version: "task-io-linking-v1",
	}, steps)
	if err != nil {
		logs.Error("Suite failed: %v", err)
		os.Exit(1)
	}
}

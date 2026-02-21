package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	githubv1 "dialtone/dev/plugins/github/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}

	tmpDir := filepath.Join(repoRoot, "src", "plugins", "github", "src_v1", "test", "tmp")
	_ = os.RemoveAll(tmpDir)
	_ = os.MkdirAll(tmpDir, 0o755)
	defer os.RemoveAll(tmpDir)

	steps := []testv1.Step{
		{
			Name: "render-markdown-has-wait-status",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				doc := githubv1.RenderIssueTaskMarkdown(githubv1.Issue{
					Number: 7,
					Title:  "Fix CI flake",
					Body:   "CI flakes on linux",
					State:  "open",
				})
				if !strings.Contains(doc, "- status: wait") {
					return testv1.StepRunResult{}, fmt.Errorf("task markdown missing wait status")
				}
				ctx.Logf("wait status found in rendered markdown")
				return testv1.StepRunResult{Report: "wait status verified"}, nil
			},
		},
		{
			Name: "example-library-runs",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				cmd := exec.Command("go", "run", "./plugins/github/src_v1/test/02_example_library/main.go")
				cmd.Dir = filepath.Join(repoRoot, "src")
				var out bytes.Buffer
				cmd.Stdout = &out
				cmd.Stderr = &out
				if err := cmd.Run(); err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("example library failed: %v\n%s", err, out.String())
				}
				if !strings.Contains(out.String(), "GITHUB_LIBRARY_EXAMPLE_PASS") {
					return testv1.StepRunResult{}, fmt.Errorf("missing pass marker:\n%s", out.String())
				}
				ctx.Logf("example output: %s", strings.TrimSpace(out.String()))
				return testv1.StepRunResult{Report: "example library pass marker found"}, nil
			},
		},
		{
			Name: "write-task-file-shape",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				path := filepath.Join(tmpDir, "999.md")
				doc := githubv1.RenderIssueTaskMarkdown(githubv1.Issue{
					Number: 999,
					Title:  "Downloaded issue",
					Body:   "Task body",
					State:  "open",
				})
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
				ctx.Logf("created %s", path)
				return testv1.StepRunResult{Report: "task file shape verified"}, nil
			},
		},
	}

	if err := testv1.RunSuite(testv1.SuiteOptions{
		Version: "src_v1",
	}, steps); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("PASS: github src_v1 self-check")
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

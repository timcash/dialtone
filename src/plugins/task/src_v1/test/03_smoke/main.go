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
	var tmpTasksDir string
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
				ctx.Logf("Created temp tasks dir: %s", tmpTasksDir)

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
			Name: "scaffold-dependency-tree",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				tasks := []string{"env-config-update", "auth-middleware-v2"}
				
				for _, name := range tasks {
					cmd := exec.Command("go", "run", taskToolRel, "--tasks-dir", tmpTasksDir, "create", name)
					cmd.Dir = srcDir
					out, err := cmd.CombinedOutput()
					if err != nil {
						return testv1.StepRunResult{}, logs.Errorf("failed to create task %s: %v, output: %s", name, err, string(out))
					}
				}

				v2Path := filepath.Join(tmpTasksDir, "auth-middleware-v2", "v2", "root.md")
				content, err := os.ReadFile(v2Path)
				if err != nil {
					return testv1.StepRunResult{}, err
				}
				
				updatedContent := strings.Replace(string(content), "### task-dependencies:\n- none", "### task-dependencies:\n- env-config-update", 1)
				if err := os.WriteFile(v2Path, []byte(updatedContent), 0644); err != nil {
					return testv1.StepRunResult{}, err
				}

				return testv1.StepRunResult{Report: "Scaffolded 2 tasks with 1 dependency"}, nil
			},
		},
		{
			Name: "verify-v1-v2-diff",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				v1Path := filepath.Join(tmpTasksDir, "auth-middleware-v2", "v1", "root.md")
				v2Path := filepath.Join(tmpTasksDir, "auth-middleware-v2", "v2", "root.md")

				v1, _ := os.ReadFile(v1Path)
				v2, _ := os.ReadFile(v2Path)

				if string(v1) == string(v2) {
					return testv1.StepRunResult{}, logs.Errorf("v1 and v2 should be different for auth-middleware-v2")
				}

				return testv1.StepRunResult{Report: "Verified baseline vs WIP separation"}, nil
			},
		},
		{
			Name: "sign-and-validate",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				taskName := "env-config-update"
				
				cmd := exec.Command("go", "run", taskToolRel, "--tasks-dir", tmpTasksDir, "sign", taskName, "--role", "LLM-CODE")
				cmd.Dir = srcDir
				if out, err := cmd.CombinedOutput(); err != nil {
					return testv1.StepRunResult{}, logs.Errorf("sign failed: %v, output: %s", taskName, err, string(out))
				}

				cmd = exec.Command("go", "run", taskToolRel, "--tasks-dir", tmpTasksDir, "validate", taskName)
				cmd.Dir = srcDir
				if out, err := cmd.CombinedOutput(); err != nil {
					return testv1.StepRunResult{}, logs.Errorf("validate failed: %v, output: %s", taskName, err, string(out))
				}

				return testv1.StepRunResult{Report: "Successfully signed and validated task"}, nil
			},
		},
		{
			Name: "archive-and-verify-match",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				taskName := "auth-middleware-v2"
				
				cmd := exec.Command("go", "run", taskToolRel, "--tasks-dir", tmpTasksDir, "archive", taskName)
				cmd.Dir = srcDir
				if out, err := cmd.CombinedOutput(); err != nil {
					return testv1.StepRunResult{}, logs.Errorf("archive failed: %v, output: %s", taskName, err, string(out))
				}

				v1Path := filepath.Join(tmpTasksDir, "auth-middleware-v2", "v1", "root.md")
				v2Path := filepath.Join(tmpTasksDir, "auth-middleware-v2", "v2", "root.md")

				v1, _ := os.ReadFile(v1Path)
				v2, _ := os.ReadFile(v2Path)

				if string(v1) != string(v2) {
					return testv1.StepRunResult{}, logs.Errorf("v1 and v2 should match after archive")
				}

				if !strings.Contains(string(v1), "- env-config-update") {
					return testv1.StepRunResult{}, logs.Errorf("v1 missing dependency after archive")
				}

				return testv1.StepRunResult{Report: "Verified v2 promoted to v1 and v2 reset to match"}, nil
			},
		},
		{
			Name: "cleanup",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				if tmpTasksDir != "" {
					os.RemoveAll(tmpTasksDir)
				}
				return testv1.StepRunResult{Report: "Cleaned up temp directories"}, nil
			},
		},
	}

	err := testv1.RunSuite(testv1.SuiteOptions{
		Version: "task-workflow-v1",
	}, steps)
	if err != nil {
		logs.Error("Suite failed: %v", err)
		os.Exit(1)
	}
}

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	steps := []testv1.Step{
		{
			Name: "create-task",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				taskName := "smoke-test-task"
				ctx.Logf("Creating task: %s", taskName)
				
				cwd, _ := os.Getwd()
				repoRoot := cwd
				if filepath.Base(cwd) == "src" {
					repoRoot = filepath.Dir(cwd)
				}
				
				taskTool := filepath.Join(repoRoot, "src", "plugins", "task", "src_v1", "go", "main.go")
				
				cmd := exec.Command("go", "run", taskTool, "create", taskName)
				cmd.Dir = repoRoot
				out, err := cmd.CombinedOutput()
				if err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("create failed: %v, output: %s", err, string(out))
				}

				v1Path := filepath.Join(repoRoot, "src", "plugins", "task", "database", taskName, "v1", taskName+".md")
				v2Path := filepath.Join(repoRoot, "src", "plugins", "task", "database", taskName, "v2", taskName+".md")

				if _, err := os.Stat(v1Path); err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("v1 file missing at %s: %v", v1Path, err)
				}
				if _, err := os.Stat(v2Path); err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("v2 file missing at %s: %v", v2Path, err)
				}

				return testv1.StepRunResult{Report: "Task files created successfully"}, nil
			},
		},
		{
			Name: "sign-task",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				taskName := "smoke-test-task"
				ctx.Logf("Signing task: %s", taskName)

				cwd, _ := os.Getwd()
				repoRoot := cwd
				if filepath.Base(cwd) == "src" {
					repoRoot = filepath.Dir(cwd)
				}
				
				taskTool := filepath.Join(repoRoot, "src", "plugins", "task", "src_v1", "go", "main.go")

				cmd := exec.Command("go", "run", taskTool, "sign", taskName, "--role", "LLM-CODE")
				cmd.Dir = repoRoot
				out, err := cmd.CombinedOutput()
				if err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("sign failed: %v, output: %s", err, string(out))
				}

				v2Path := filepath.Join(repoRoot, "src", "plugins", "task", "database", taskName, "v2", taskName+".md")
				content, err := os.ReadFile(v2Path)
				if err != nil {
					return testv1.StepRunResult{}, err
				}

				if !strings.Contains(string(content), "LLM-CODE>") {
					return testv1.StepRunResult{}, fmt.Errorf("v2 content does not contain signature")
				}

				return testv1.StepRunResult{Report: "Task signed successfully"}, nil
			},
		},
		{
			Name: "validate-task",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				taskName := "smoke-test-task"
				ctx.Logf("Validating task: %s", taskName)

				cwd, _ := os.Getwd()
				repoRoot := cwd
				if filepath.Base(cwd) == "src" {
					repoRoot = filepath.Dir(cwd)
				}
				
				taskTool := filepath.Join(repoRoot, "src", "plugins", "task", "src_v1", "go", "main.go")

				cmd := exec.Command("go", "run", taskTool, "validate", taskName)
				cmd.Dir = repoRoot
				out, err := cmd.CombinedOutput()
				if err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("validate failed: %v, output: %s", err, string(out))
				}

				if !strings.Contains(string(out), "Validation PASSED") {
					return testv1.StepRunResult{}, fmt.Errorf("unexpected output: %s", string(out))
				}

				return testv1.StepRunResult{Report: "Task validated successfully"}, nil
			},
		},
		{
			Name: "archive-task",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				taskName := "smoke-test-task"
				ctx.Logf("Archiving task: %s", taskName)

				cwd, _ := os.Getwd()
				repoRoot := cwd
				if filepath.Base(cwd) == "src" {
					repoRoot = filepath.Dir(cwd)
				}
				
				taskTool := filepath.Join(repoRoot, "src", "plugins", "task", "src_v1", "go", "main.go")

				cmd := exec.Command("go", "run", taskTool, "archive", taskName)
				cmd.Dir = repoRoot
				out, err := cmd.CombinedOutput()
				if err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("archive failed: %v, output: %s", err, string(out))
				}

				v1Path := filepath.Join(repoRoot, "src", "plugins", "task", "database", taskName, "v1", taskName+".md")
				v1Content, err := os.ReadFile(v1Path)
				if err != nil {
					return testv1.StepRunResult{}, err
				}

				if !strings.Contains(string(v1Content), "LLM-CODE>") {
					return testv1.StepRunResult{}, fmt.Errorf("v1 content does not match v2 after archive")
				}

				return testv1.StepRunResult{Report: "Task archived successfully"}, nil
			},
		},
	}

	err := testv1.RunSuite(testv1.SuiteOptions{
		Version: "task-smoke-v1",
	}, steps)
	if err != nil {
		fmt.Printf("Suite failed: %v\n", err)
		os.Exit(1)
	}
}

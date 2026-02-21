package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	tsnetv1 "dialtone/dev/plugins/tsnet/src_v1/go"
)

func main() {
	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}

	steps := []testv1.Step{
		{
			Name: "normalize-hostname",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				got := tsnetv1.NormalizeHostname("Robot_1 Dev")
				if got != "robot-1-dev" {
					return testv1.StepRunResult{}, fmt.Errorf("expected robot-1-dev, got %s", got)
				}
				ctx.Logf("normalized hostname=%s", got)
				return testv1.StepRunResult{Report: "hostname normalization pass"}, nil
			},
		},
		{
			Name: "resolve-config-defaults",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				cfg, err := tsnetv1.ResolveConfig("", "")
				if err != nil {
					return testv1.StepRunResult{}, err
				}
				if cfg.Hostname == "" || cfg.StateDir == "" {
					return testv1.StepRunResult{}, fmt.Errorf("empty config fields")
				}
				ctx.Logf("config hostname=%s state_dir=%s", cfg.Hostname, cfg.StateDir)
				return testv1.StepRunResult{Report: "config defaults pass"}, nil
			},
		},
		{
			Name: "example-library-runs",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				cmd := exec.Command("go", "run", "./plugins/tsnet/src_v1/test/02_example_library/main.go")
				cmd.Dir = filepath.Join(repoRoot, "src")
				var out bytes.Buffer
				cmd.Stdout = &out
				cmd.Stderr = &out
				if err := cmd.Run(); err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("example failed: %v\n%s", err, out.String())
				}
				if !strings.Contains(out.String(), "TSNET_LIBRARY_EXAMPLE_PASS") {
					return testv1.StepRunResult{}, fmt.Errorf("pass marker missing:\n%s", out.String())
				}
				ctx.Logf(strings.TrimSpace(out.String()))
				return testv1.StepRunResult{Report: "example library pass marker found"}, nil
			},
		},
		{
			Name: "build-create-key-request",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				req := tsnetv1.BuildCreateKeyRequest(tsnetv1.ProvisionOptions{
					Description:   "demo-key",
					Tags:          []string{"robot", "tag:ops"},
					Reusable:      true,
					Ephemeral:     false,
					Preauthorized: true,
					ExpiryHours:   12,
				})
				gotDesc, _ := req["description"].(string)
				if gotDesc != "demo-key" {
					return testv1.StepRunResult{}, fmt.Errorf("description mismatch: %s", gotDesc)
				}
				gotExp, _ := req["expirySeconds"].(int)
				if gotExp != 43200 {
					return testv1.StepRunResult{}, fmt.Errorf("expirySeconds mismatch: %d", gotExp)
				}
				create := req["capabilities"].(map[string]any)["devices"].(map[string]any)["create"].(map[string]any)
				tagsAny, _ := create["tags"].([]string)
				wantTags := []string{"tag:robot", "tag:ops"}
				if !reflect.DeepEqual(tagsAny, wantTags) {
					return testv1.StepRunResult{}, fmt.Errorf("tags mismatch: got=%v want=%v", tagsAny, wantTags)
				}
				ctx.Logf("build key request pass")
				return testv1.StepRunResult{Report: "create key request pass"}, nil
			},
		},
		{
			Name: "upsert-env-var",
			RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
				tmp := filepath.Join(repoRoot, "src", "plugins", "tsnet", "src_v1", "test", "tmp.env")
				defer os.Remove(tmp)
				if err := os.WriteFile(tmp, []byte("A=1\nTS_AUTHKEY=old\n"), 0o644); err != nil {
					return testv1.StepRunResult{}, err
				}
				if err := tsnetv1.UpsertEnvVar(tmp, "TS_AUTHKEY", "new-value"); err != nil {
					return testv1.StepRunResult{}, err
				}
				raw, err := os.ReadFile(tmp)
				if err != nil {
					return testv1.StepRunResult{}, err
				}
				text := string(raw)
				if !strings.Contains(text, "TS_AUTHKEY=new-value") {
					return testv1.StepRunResult{}, fmt.Errorf("upsert failed: %s", text)
				}
				ctx.Logf("env upsert pass")
				return testv1.StepRunResult{Report: "env upsert pass"}, nil
			},
		},
	}

	if err := testv1.RunSuite(testv1.SuiteOptions{
		Version: "src_v1",
	}, steps); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("PASS: tsnet src_v1 self-check")
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

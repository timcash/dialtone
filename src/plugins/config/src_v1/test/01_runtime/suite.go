package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name: "runtime-resolves-repo-and-env",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := configv1.ResolveRuntime("")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if !strings.HasSuffix(rt.EnvFile, "env/dialtone.json") {
				return testv1.StepRunResult{}, fmt.Errorf("env file must default to env/dialtone.json: %s", rt.EnvFile)
			}
			if strings.Contains(strings.ReplaceAll(rt.EnvFile, "\\", "/"), "/src/env/") {
				return testv1.StepRunResult{}, fmt.Errorf("env file must not resolve under src/env: %s", rt.EnvFile)
			}
			return testv1.StepRunResult{Report: "resolved runtime with env/dialtone.json"}, nil
		},
	})

	reg.Add(testv1.Step{
		Name: "plugin-preset-plugin-version-root",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := configv1.ResolveRuntime("")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			preset := configv1.NewPluginPreset(rt, "logs", "src_v1")
			if !strings.HasSuffix(preset.PluginVersionRoot, "src/plugins/logs/src_v1") {
				return testv1.StepRunResult{}, fmt.Errorf("unexpected PluginVersionRoot: %s", preset.PluginVersionRoot)
			}
			if preset.UI != preset.Join("ui") {
				return testv1.StepRunResult{}, fmt.Errorf("preset UI path mismatch")
			}
			if preset.TestCmd != preset.Join("test", "cmd") {
				return testv1.StepRunResult{}, fmt.Errorf("preset TestCmd path mismatch")
			}
			return testv1.StepRunResult{Report: "plugin preset paths resolved from PluginVersionRoot"}, nil
		},
	})

	reg.Add(testv1.Step{
		Name: "runtime-respects-explicit-env-file",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			tempRoot, err := os.MkdirTemp("", "dialtone-config-runtime-*")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer os.RemoveAll(tempRoot)

			envFile := filepath.Join(tempRoot, "custom-runtime.json")
			restoreKeys := []string{
				"DIALTONE_ENV_FILE",
				"DIALTONE_HOME",
				"DIALTONE_ENV",
				"DIALTONE_GO_CACHE_DIR",
				"DIALTONE_BUN_CACHE_DIR",
				"DIALTONE_TOOL_CACHE_DIR",
				"DIALTONE_CONTAINER_CACHE_DIR",
			}
			savedValues := map[string]string{}
			savedSet := map[string]bool{}
			for _, key := range restoreKeys {
				value, ok := os.LookupEnv(key)
				savedValues[key] = value
				savedSet[key] = ok
				_ = os.Unsetenv(key)
			}
			defer func() {
				for _, key := range restoreKeys {
					if savedSet[key] {
						_ = os.Setenv(key, savedValues[key])
						continue
					}
					_ = os.Unsetenv(key)
				}
			}()

			_ = os.Setenv("DIALTONE_ENV_FILE", envFile)

			rt, err := configv1.ResolveRuntime("")
			if err != nil {
				return testv1.StepRunResult{}, err
			}

			wantHome := filepath.Join(tempRoot, ".dialtone")
			wantEnv := filepath.Join(tempRoot, ".dialtone_env")
			if rt.EnvFile != envFile {
				return testv1.StepRunResult{}, fmt.Errorf("env file mismatch: got=%q want=%q", rt.EnvFile, envFile)
			}
			if rt.DialtoneHome != wantHome {
				return testv1.StepRunResult{}, fmt.Errorf("dialtone home mismatch: got=%q want=%q", rt.DialtoneHome, wantHome)
			}
			if rt.DialtoneEnv != wantEnv {
				return testv1.StepRunResult{}, fmt.Errorf("dialtone env mismatch: got=%q want=%q", rt.DialtoneEnv, wantEnv)
			}
			if rt.GoCacheDir != filepath.Join(wantEnv, "cache", "go") {
				return testv1.StepRunResult{}, fmt.Errorf("go cache mismatch: %s", rt.GoCacheDir)
			}
			if rt.BunCacheDir != filepath.Join(wantEnv, "cache", "bun") {
				return testv1.StepRunResult{}, fmt.Errorf("bun cache mismatch: %s", rt.BunCacheDir)
			}
			return testv1.StepRunResult{Report: "explicit env file keeps folder-scoped runtime defaults"}, nil
		},
	})
}

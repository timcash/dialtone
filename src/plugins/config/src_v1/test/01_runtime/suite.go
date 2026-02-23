package runtime

import (
	"fmt"
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
			if !strings.HasSuffix(rt.EnvFile, "env/.env") {
				return testv1.StepRunResult{}, fmt.Errorf("env file must default to env/.env: %s", rt.EnvFile)
			}
			if strings.Contains(rt.EnvFile, "/src/env/.env") {
				return testv1.StepRunResult{}, fmt.Errorf("env file must not resolve to src/env/.env: %s", rt.EnvFile)
			}
			return testv1.StepRunResult{Report: "resolved runtime with env/.env"}, nil
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
}

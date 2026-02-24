package ops

import (
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	test_plugin "dialtone/dev/plugins/test/src_v1/go"
	"path/filepath"
)

func Dev(repoRoot string, args []string) error {
	rt, err := configv1.ResolveRuntime(repoRoot)
	if err != nil {
		return err
	}
	preset := configv1.NewPluginPreset(rt, "wsl", "src_v3")
	pluginDir := preset.PluginVersionRoot
	uiDir := preset.UI

	opts := test_plugin.DevOptions{
		RepoRoot:          repoRoot,
		PluginDir:         pluginDir,
		UIDir:             uiDir,
		DevPort:           3000,
		Role:              "wsl-dev",
		BrowserMetaPath:   filepath.Join(pluginDir, "dev.browser.json"),
		BrowserModeEnvVar: "WSL_DEV_BROWSER_MODE",
	}
	return test_plugin.RunDev(opts)
}

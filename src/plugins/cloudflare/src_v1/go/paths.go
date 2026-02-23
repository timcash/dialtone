package cloudflare

import (
	"path/filepath"
	"strings"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

type Paths struct {
	Runtime           configv1.Runtime
	Preset            configv1.PluginPreset
	PluginVersionRoot string
	TestMain          string
	TestReport        string
	TestLog           string
	TestErrorLog      string
	DevBrowserMeta    string
	DevLog            string
}

func ResolvePaths(start, version string) (Paths, error) {
	rt, err := configv1.ResolveRuntime(start)
	if err != nil {
		return Paths{}, err
	}
	version = strings.TrimSpace(version)
	if version == "" {
		version = "src_v1"
	}
	preset := configv1.NewPluginPreset(rt, "cloudflare", version)
	return Paths{
		Runtime:           rt,
		Preset:            preset,
		PluginVersionRoot: preset.PluginVersionRoot,
		TestMain:          filepath.Join(preset.Test, "main.go"),
		TestReport:        filepath.Join(preset.Test, "TEST.md"),
		TestLog:           filepath.Join(preset.Test, "test.log"),
		TestErrorLog:      filepath.Join(preset.Test, "error.log"),
		DevBrowserMeta:    filepath.Join(preset.PluginVersionRoot, "dev.browser.json"),
		DevLog:            filepath.Join(preset.PluginVersionRoot, "dev.log"),
	}, nil
}

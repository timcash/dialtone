package robotv1

import (
	"path/filepath"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

type Paths struct {
	Runtime           configv1.Runtime
	Preset            configv1.PluginPreset
	PluginVersionRoot string
	DiagnosticShots   string
	TestShots         string
	ServerMain        string
	SleepMain         string
	DevLog            string
	DevBrowserMeta    string
}

func ResolvePaths(start string) (Paths, error) {
	rt, err := configv1.ResolveRuntime(start)
	if err != nil {
		return Paths{}, err
	}
	preset := configv1.NewPluginPreset(rt, "robot", "src_v1")
	return Paths{
		Runtime:           rt,
		Preset:            preset,
		PluginVersionRoot: preset.PluginVersionRoot,
		DiagnosticShots:   filepath.Join(preset.PluginVersionRoot, "diagnostic_screenshots"),
		TestShots:         filepath.Join(preset.Test, "screenshots"),
		ServerMain:        filepath.Join(preset.Cmd, "server", "main.go"),
		SleepMain:         filepath.Join(preset.Cmd, "sleep", "main.go"),
		DevLog:            filepath.Join(preset.PluginVersionRoot, "dev.log"),
		DevBrowserMeta:    filepath.Join(preset.PluginVersionRoot, "dev.browser.json"),
	}, nil
}

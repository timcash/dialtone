package ui

import (
	"path/filepath"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

type Paths struct {
	Runtime           configv1.Runtime
	Preset            configv1.PluginPreset
	PluginVersionRoot string
	FixtureApp        string
	FixtureDist       string
	TestCmdMain       string
}

func ResolvePaths(start string) (Paths, error) {
	rt, err := configv1.ResolveRuntime(start)
	if err != nil {
		return Paths{}, err
	}
	preset := configv1.NewPluginPreset(rt, "ui", "src_v1")
	fixtureApp := filepath.Join(preset.Test, "fixtures", "app")
	return Paths{
		Runtime:           rt,
		Preset:            preset,
		PluginVersionRoot: preset.PluginVersionRoot,
		FixtureApp:        fixtureApp,
		FixtureDist:       filepath.Join(fixtureApp, "dist"),
		TestCmdMain:       filepath.Join(preset.TestCmd, "main.go"),
	}, nil
}

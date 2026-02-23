package repl

import (
	"path/filepath"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

type Paths struct {
	Runtime           configv1.Runtime
	Preset            configv1.PluginPreset
	PluginVersionRoot string
	StandaloneBinDir  string
	StandaloneBin     string
	TestCmdMain       string
}

func ResolvePaths(start string) (Paths, error) {
	rt, err := configv1.ResolveRuntime(start)
	if err != nil {
		return Paths{}, err
	}
	preset := configv1.NewPluginPreset(rt, "repl", "src_v1")
	return Paths{
		Runtime:           rt,
		Preset:            preset,
		PluginVersionRoot: preset.PluginVersionRoot,
		StandaloneBinDir:  configv1.RepoPath(rt, ".dialtone", "bin"),
		StandaloneBin:     configv1.RepoPath(rt, ".dialtone", "bin", "repl-src_v1"),
		TestCmdMain:       filepath.Join(preset.TestCmd, "main.go"),
	}, nil
}

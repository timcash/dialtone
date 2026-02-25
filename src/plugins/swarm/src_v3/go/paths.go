package swarmv3

import (
	"path/filepath"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

type Paths struct {
	Runtime    configv1.Runtime
	Preset     configv1.PluginPreset
	VersionDir string
	SourceFile string
	LibudxDir  string
	BinAMD64   string
	BinARM64   string
}

func ResolvePaths(start string) (Paths, error) {
	rt, err := configv1.ResolveRuntime(start)
	if err != nil {
		return Paths{}, err
	}
	preset := configv1.NewPluginPreset(rt, "swarm", "src_v3")
	return Paths{
		Runtime:    rt,
		Preset:     preset,
		VersionDir: preset.PluginVersionRoot,
		SourceFile: filepath.Join(preset.PluginVersionRoot, "dialtone_swarm_v3.c"),
		LibudxDir:  filepath.Join(preset.PluginVersionRoot, "libudx"),
		BinAMD64:   filepath.Join(preset.PluginVersionRoot, "dialtone_swarm_v3_x86_64"),
		BinARM64:   filepath.Join(preset.PluginVersionRoot, "dialtone_swarm_v3_arm64"),
	}, nil
}

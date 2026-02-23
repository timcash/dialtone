package github

import (
	"path/filepath"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

type Paths struct {
	Preset    configv1.PluginPreset
	IssuesDir string
	PRsDir    string
}

func ResolvePaths() (Paths, error) {
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return Paths{}, err
	}
	preset := configv1.NewPluginPreset(rt, "github", "src_v1")
	return Paths{
		Preset:    preset,
		IssuesDir: filepath.Join(preset.PluginVersionRoot, "issues"),
		PRsDir:    filepath.Join(preset.PluginVersionRoot, "prs"),
	}, nil
}

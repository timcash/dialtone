package gacad

import (
	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

type Paths struct {
	Runtime    configv1.Runtime
	Preset     configv1.PluginPreset
	UIDir      string
	UIDist     string
	StateDir   string
	ServerPID  string
	ServerMeta string
}

func ResolvePaths(start, version string) (Paths, error) {
	if version == "" {
		version = "src_v1"
	}
	rt, err := configv1.ResolveRuntime(start)
	if err != nil {
		return Paths{}, err
	}
	preset := configv1.NewPluginPreset(rt, "ga_cad", version)
	return Paths{
		Runtime:    rt,
		Preset:     preset,
		UIDir:      preset.UI,
		UIDist:     preset.UIDist,
		StateDir:   configv1.RepoPath(rt, ".dialtone", "ga_cad", version),
		ServerPID:  configv1.RepoPath(rt, ".dialtone", "ga_cad", version, "server.pid"),
		ServerMeta: configv1.RepoPath(rt, ".dialtone", "ga_cad", version, "server.json"),
	}, nil
}

package cad

import (
	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

type Paths struct {
	Runtime     configv1.Runtime
	Preset      configv1.PluginPreset
	BackendDir  string
	BackendMain string
	UIDir       string
	UIDist      string
	StateDir    string
	ServerPID   string
	ServerMeta  string
}

func ResolvePaths(start, version string) (Paths, error) {
	if version == "" {
		version = "src_v1"
	}
	rt, err := configv1.ResolveRuntime(start)
	if err != nil {
		return Paths{}, err
	}
	preset := configv1.NewPluginPreset(rt, "cad", version)
	return Paths{
		Runtime:     rt,
		Preset:      preset,
		BackendDir:  preset.Join("backend"),
		BackendMain: preset.Join("backend", "main.py"),
		UIDir:       preset.UI,
		UIDist:      preset.UIDist,
		StateDir:    configv1.RepoPath(rt, ".dialtone", "cad", version),
		ServerPID:   configv1.RepoPath(rt, ".dialtone", "cad", version, "server.pid"),
		ServerMeta:  configv1.RepoPath(rt, ".dialtone", "cad", version, "server.json"),
	}, nil
}

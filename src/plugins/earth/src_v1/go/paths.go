package earth

import (
	"strings"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

type Paths struct {
	Runtime configv1.Runtime
	Preset  configv1.PluginPreset
}

func ResolvePaths(version string) (Paths, error) {
	ver := strings.TrimSpace(version)
	if ver == "" {
		ver = "src_v1"
	}
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return Paths{}, err
	}
	return Paths{Runtime: rt, Preset: configv1.NewPluginPreset(rt, "earth", ver)}, nil
}

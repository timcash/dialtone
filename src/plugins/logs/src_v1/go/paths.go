package logs

import (
	"path/filepath"
	"strings"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

type Paths struct {
	Runtime           configv1.Runtime
	Preset            configv1.PluginPreset
	PluginVersionRoot string
	DevLog            string
	DevBrowserMeta    string
	DevPreviewLog     string
	NATSDaemonPID     string
	NATSDaemonLog     string
	TestReport        string
	TestLog           string
	TestErrorLog      string
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
	preset := configv1.NewPluginPreset(rt, "logs", version)
	return Paths{
		Runtime:           rt,
		Preset:            preset,
		PluginVersionRoot: preset.PluginVersionRoot,
		DevLog:            filepath.Join(preset.PluginVersionRoot, "dev.log"),
		DevBrowserMeta:    filepath.Join(preset.PluginVersionRoot, "dev.browser.json"),
		DevPreviewLog:     filepath.Join(preset.Test, "dev_preview.log"),
		NATSDaemonPID:     configv1.RepoPath(rt, ".dialtone", "logs", "logs-nats-daemon.pid"),
		NATSDaemonLog:     configv1.RepoPath(rt, ".dialtone", "logs", "logs-nats-daemon.log"),
		TestReport:        filepath.Join(preset.PluginVersionRoot, "TEST.md"),
		TestLog:           filepath.Join(preset.PluginVersionRoot, "test.log"),
		TestErrorLog:      filepath.Join(preset.PluginVersionRoot, "error.log"),
	}, nil
}

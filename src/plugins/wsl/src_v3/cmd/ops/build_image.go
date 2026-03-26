package ops

import (
	"flag"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	wslv3 "dialtone/dev/plugins/wsl/src_v3/go"
)

func BuildImage(args ...string) error {
	fs := flag.NewFlagSet("wsl-build-image", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := wslv3.EnsureBuildImage("")
	if err != nil {
		return err
	}
	logs.Info(">> [WSL] build image ready: %s", cfg.ImageName)
	logs.Info(">> [WSL] build image cache: %s", cfg.CacheDir)
	return nil
}

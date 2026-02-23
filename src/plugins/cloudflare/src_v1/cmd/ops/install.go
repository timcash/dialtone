package ops

import (
	"fmt"
	"os"
	"path/filepath"
)

func Install() error {
	paths, err := resolveCloudflarePaths()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(paths.Preset.UI, "package.json")); err != nil {
		return fmt.Errorf("missing src_v1 ui package.json: %w", err)
	}
	cmd := runDialtone(paths.Runtime.RepoRoot, "bun", "src_v1", "exec", "--cwd", paths.Preset.UI, "install", "--force")
	return cmd.Run()
}

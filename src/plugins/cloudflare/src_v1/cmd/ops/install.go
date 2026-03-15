package ops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func Install() error {
	paths, err := resolveCloudflarePaths()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(paths.Preset.UI, "package.json")); err != nil {
		return fmt.Errorf("missing src_v1 ui package.json: %w", err)
	}
	binPath, err := ensureCloudflaredInstalled(paths.Runtime, false)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "installed cloudflared at %s\n", binPath)
	if err := ensureBunToolchain(paths.Runtime.RepoRoot); err != nil {
		return err
	}
	cmd := runDialtone(paths.Runtime.RepoRoot, "bun", "src_v1", "exec", "--cwd", paths.Preset.UI, "install", "--force")
	return cmd.Run()
}

func ensureBunToolchain(repoRoot string) error {
	envDir := logs.GetDialtoneEnv()
	bunBin := filepath.Join(envDir, "bun", "bin", "bun")
	if _, err := os.Stat(bunBin); err == nil {
		return nil
	}

	logs.Warn("[CLOUDFLARE INSTALL] managed bun runtime missing at %s; installing into DIALTONE_ENV", bunBin)
	if err := os.MkdirAll(filepath.Join(envDir, "bun"), 0o755); err != nil {
		return err
	}
	installCmd := "curl -fsSL https://bun.sh/install | BUN_INSTALL='" + filepath.Join(envDir, "bun") + "' bash"
	cmd := exec.Command("bash", "-lc", installCmd)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install bun runtime into %s: %w", bunBin, err)
	}
	if _, err := os.Stat(bunBin); err != nil {
		return fmt.Errorf("bun runtime still missing after install at %s", bunBin)
	}
	return nil
}

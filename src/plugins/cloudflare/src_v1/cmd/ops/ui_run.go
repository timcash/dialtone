package ops

import "strconv"

func UIRun(port int) error {
	paths, err := resolveCloudflarePaths()
	if err != nil {
		return err
	}
	if port == 0 {
		port = 3000
	}
	cmd := runDialtone(paths.Runtime.RepoRoot, "bun", "src_v1", "exec", "--cwd", paths.Preset.UI, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(port))
	return cmd.Run()
}

package ops

func Lint() error {
	paths, err := resolveCloudflarePaths()
	if err != nil {
		return err
	}
	cmd := runDialtone(paths.Runtime.RepoRoot, "bun", "src_v1", "exec", "--cwd", paths.Preset.UI, "run", "lint")
	return cmd.Run()
}

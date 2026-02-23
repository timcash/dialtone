package ops

func GoBuild() error {
	paths, err := resolveCloudflarePaths()
	if err != nil {
		return err
	}
	cmd := runDialtone(paths.Runtime.RepoRoot, "go", "src_v1", "exec", "build", "./plugins/cloudflare/src_v1/...")
	cmd.Dir = paths.Runtime.SrcRoot
	return cmd.Run()
}

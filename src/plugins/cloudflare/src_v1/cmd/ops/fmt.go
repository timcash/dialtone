package ops

func Fmt() error {
	paths, err := resolveCloudflarePaths()
	if err != nil {
		return err
	}
	cmd := runDialtone(paths.Runtime.RepoRoot, "go", "src_v1", "exec", "fmt", "./plugins/cloudflare/src_v1/...")
	cmd.Dir = paths.Runtime.SrcRoot
	return cmd.Run()
}

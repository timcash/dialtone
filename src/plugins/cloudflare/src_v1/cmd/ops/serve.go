package ops

import "path/filepath"

func Serve() error {
	paths, err := resolveCloudflarePaths()
	if err != nil {
		return err
	}
	cmd := runDialtone(paths.Runtime.RepoRoot, "go", "src_v1", "exec", "run", filepath.ToSlash(filepath.Join("plugins", "cloudflare", "src_v1", "cmd", "main.go")))
	cmd.Dir = paths.Runtime.SrcRoot
	return cmd.Run()
}

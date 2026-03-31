package ops

import "path/filepath"

func Serve(args []string) error {
	paths, err := resolveCloudflarePaths()
	if err != nil {
		return err
	}
	cmdArgs := []string{
		"go", "src_v1", "exec", "run",
		filepath.ToSlash(filepath.Join("plugins", "cloudflare", "src_v1", "cmd", "main.go")),
	}
	cmdArgs = append(cmdArgs, args...)
	cmd := runDialtone(paths.Runtime.RepoRoot, cmdArgs...)
	cmd.Dir = paths.Runtime.SrcRoot
	return cmd.Run()
}

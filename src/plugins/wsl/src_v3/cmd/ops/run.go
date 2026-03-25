package ops

import (
	"os"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

func Run(repoRoot string, args []string) error {
	rt, err := configv1.ResolveRuntime(repoRoot)
	if err != nil {
		return err
	}
	cmd := getDialtoneCmd(rt.RepoRoot)
	cmd.Args = append(cmd.Args, "go", "src_v1", "exec", "run", "./plugins/wsl/src_v3/cmd/server/main.go")
	cmd.Args = append(cmd.Args, args...)
	cmd.Dir = rt.RepoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

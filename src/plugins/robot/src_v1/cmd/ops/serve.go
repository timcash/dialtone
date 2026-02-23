package ops

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Serve(repoRoot string, flags ...string) error {
	remoteOpts, _, err := parseRemoteOptions("robot-serve", flags)
	if err != nil {
		return err
	}
	if remoteOpts.Remote {
		return runRemoteServe(remoteOpts)
	}

	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "go", "src_v1", "exec", "run", "plugins/robot/src_v1/cmd/server/main.go")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

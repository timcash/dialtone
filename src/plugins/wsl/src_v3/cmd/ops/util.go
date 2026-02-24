package ops

import (
	"os/exec"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

func getDialtoneCmd(repoRoot string) *exec.Cmd {
	return configv1.DialtoneCommand(repoRoot)
}

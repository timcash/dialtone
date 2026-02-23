package ops

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Sleep starts the lightweight sleep server (local process) for src_v1.
// It runs in the foreground and serves on local :8080 and tsnet :80.
func Sleep(repoRoot string, args []string) error {
	cmdArgs := []string{"go", "src_v1", "exec", "run", "./plugins/robot/src_v1/cmd/sleep/main.go"}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), cmdArgs...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

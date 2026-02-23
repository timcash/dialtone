package ops

import (
	"os"
	"os/exec"
	"path/filepath"

	cloudflarev1 "dialtone/dev/plugins/cloudflare/src_v1/go"
)

func resolveCloudflarePaths() (cloudflarev1.Paths, error) {
	return cloudflarev1.ResolvePaths("", "src_v1")
}

func runDialtone(repoRoot string, args ...string) *exec.Cmd {
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = repoRoot
	return cmd
}

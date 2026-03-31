package test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// WaitForAllProcessesToComplete waits until 'dialtone.sh ps' returns "No active task workers."
// or the timeout is reached.
func WaitForAllProcessesToComplete(repoRoot string, timeout time.Duration) error {
	dialtoneSh := filepath.Join(repoRoot, "dialtone.sh")

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		cmd := exec.Command(dialtoneSh, "ps")
		cmd.Dir = repoRoot
		out, err := cmd.CombinedOutput()
		output := string(out)

		if err == nil {
			// Check for "No active task workers" or equivalent empty state
			// Current implementation prints "DIALTONE> No active task workers." or just headers if none?
			// dev.go: "DIALTONE> No active task workers." if len==0
			if strings.Contains(output, "No active task workers") {
				return nil
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timed out waiting for processes to complete after %v", timeout)
}

package test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// WaitForAllProcessesToComplete waits until 'dialtone.sh ps' returns "No active subtones."
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
			// Check for "No active subtones" or equivalent empty state
			// Current implementation prints "DIALTONE> No active subtones." or just headers if none?
			// dev.go: "DIALTONE> No active subtones." if len==0
			if strings.Contains(output, "No active subtones") {
				return nil
			}
		}
		
		time.Sleep(500 * time.Millisecond)
	}
	
	return fmt.Errorf("timed out waiting for processes to complete after %v", timeout)
}

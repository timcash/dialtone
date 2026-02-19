package test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// RunInstallWorkflow executes a deterministic REPL script and validates
// request -> subtone stream behavior for robot install.
func RunInstallWorkflow(timeoutSec int) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Assuming cwd is src/plugins/repl, repo root is 3 levels up
	repoRoot := filepath.Join(cwd, "..", "..", "..")
	dialtoneSh := filepath.Join(repoRoot, "dialtone.sh")
	if _, err := os.Stat(dialtoneSh); err != nil {
		return fmt.Errorf("dialtone.sh not found at expected path: %s (cwd=%s)", dialtoneSh, cwd)
	}

	script := strings.Join([]string{
		"@DIALTONE robot install src_v1",
		"exit",
		"",
	}, "\n")

	cmd := exec.Command(dialtoneSh)
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(script)
	// cmd.Env no longer needs DIALTONE_TEST_TASK_ID, inherit system env
	cmd.Env = os.Environ()

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start REPL process: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		output := out.String()
		if err != nil {
			return fmt.Errorf("repl workflow failed: %w\noutput:\n%s", err, output)
		}
		return validateOutput(output)
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		_ = cmd.Process.Kill()
		<-done
		return errors.New("repl workflow timed out")
	}
}

func validateOutput(output string) error {
	required := []string{
		"DIALTONE> Virtual Librarian online.",
		"DIALTONE> Request received. Spawning subtone for robot install...",
		"DIALTONE> Streaming stdout/stderr from subtone PID",
		"DIALTONE> Process ",
		"DIALTONE> Goodbye.",
	}

	for _, marker := range required {
		if !strings.Contains(output, marker) {
			return fmt.Errorf("missing output marker: %q\nfull output:\n%s", marker, output)
		}
	}

	pidStreamPattern := regexp.MustCompile(`DIALTONE:[0-9]+:>`)
	if !pidStreamPattern.MatchString(output) {
		return fmt.Errorf("missing subtone stream output marker DIALTONE:PID:>\nfull output:\n%s", output)
	}

	return nil
}

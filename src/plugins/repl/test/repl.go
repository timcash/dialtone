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
// request -> sign -> subtone stream behavior for robot install.
func RunInstallWorkflow(timeoutSec int) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	dialtoneSh := filepath.Join(repoRoot, "dialtone.sh")
	testTaskID := "robot-install-testid"
	script := strings.Join([]string{
		"@DIALTONE robot install src_v1",
		"@DIALTONE task --sign " + testTaskID,
		"exit",
		"",
	}, "\n")

	cmd := exec.Command(dialtoneSh)
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(script)
	cmd.Env = append(os.Environ(), "DIALTONE_TEST_TASK_ID="+testTaskID)

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
		return validateOutput(output, testTaskID)
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		_ = cmd.Process.Kill()
		<-done
		return errors.New("repl workflow timed out")
	}
}

func validateOutput(output, taskID string) error {
	required := []string{
		"DIALTONE> Virtual Librarian online.",
		"DIALTONE> Request received. Task created: `" + taskID + "`.",
		"DIALTONE> Sign with `@DIALTONE task --sign " + taskID + "` to run.",
		"DIALTONE> Signatures verified. Spawning subtone subprocess via PID",
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

package test

import (
	"dialtone/dev/core/logger"
	"dialtone/dev/core/test"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() {
	test.Register("bun-cli-stdout-propagates", "bun", []string{"plugin", "bun", "integration"}, RunStdoutPropagation)
	test.Register("bun-cli-stderr-exit-propagates", "bun", []string{"plugin", "bun", "integration"}, RunStderrAndExitPropagation)
}

func RunAll() error {
	logger.LogInfo("Running bun plugin suite...")
	return test.RunPlugin("bun")
}

func RunStdoutPropagation() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	dialtoneSh := filepath.Join(repoRoot, "dialtone.sh")
	marker := "BUN_STDOUT_MARKER_42"

	cmd := exec.Command(dialtoneSh, "bun", "exec", "--eval", fmt.Sprintf("console.log(%q)", marker))
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		return fmt.Errorf("expected success from bun stdout propagation command, got error: %w\noutput:\n%s", err, output)
	}
	if !strings.Contains(output, marker) {
		return fmt.Errorf("stdout marker missing from dialtone output\nexpected marker: %s\noutput:\n%s", marker, output)
	}
	return nil
}

func RunStderrAndExitPropagation() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	dialtoneSh := filepath.Join(repoRoot, "dialtone.sh")
	stderrMarker := "BUN_STDERR_MARKER_42"

	cmd := exec.Command(dialtoneSh, "bun", "exec", "--eval", fmt.Sprintf("console.error(%q); process.exit(23)", stderrMarker))
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	output := string(out)
	if err == nil {
		return fmt.Errorf("expected non-zero exit from bun stderr/exit command, got success\noutput:\n%s", output)
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return fmt.Errorf("expected ExitError, got %T: %v\noutput:\n%s", err, err, output)
	}
	if exitErr.ExitCode() != 1 {
		return fmt.Errorf("expected wrapper exit code 1 from go run launcher, got %d\noutput:\n%s", exitErr.ExitCode(), output)
	}
	if !strings.Contains(output, stderrMarker) {
		return fmt.Errorf("stderr marker missing from dialtone output\nexpected marker: %s\noutput:\n%s", stderrMarker, output)
	}
	if !strings.Contains(output, "exit status 23") {
		return fmt.Errorf("underlying program exit status not propagated in output\nexpected to find: exit status 23\noutput:\n%s", output)
	}
	return nil
}

package test

import (
	"dialtone/dev/logger"
	"dialtone/dev/test_core"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() {
	test.Register("go-cli-stdout-propagates", "go", []string{"plugin", "go", "integration"}, RunStdoutPropagation)
	test.Register("go-cli-stderr-exit-propagates", "go", []string{"plugin", "go", "integration"}, RunStderrAndExitPropagation)
}

// RunAll is the standard entry point required by project rules.
// It uses the registry to find and run all tests for this plugin.
func RunAll() error {
	logger.LogInfo("Running go plugin suite...")
	return test.RunPlugin("go")
}

func RunStdoutPropagation() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	dialtoneSh := filepath.Join(repoRoot, "dialtone.sh")
	marker := "GO_STDOUT_MARKER_42"

	cmd := exec.Command(dialtoneSh, "go", "exec", "run", "./src/plugins/go/test/fixtures/stdout/main.go", marker)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		return fmt.Errorf("expected success from go stdout propagation command, got error: %w\noutput:\n%s", err, output)
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
	stderrMarker := "GO_STDERR_MARKER_42"
	expectedCode := "17"

	cmd := exec.Command(dialtoneSh, "go", "exec", "run", "./src/plugins/go/test/fixtures/stderr_exit/main.go", stderrMarker, expectedCode)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	output := string(out)
	if err == nil {
		return fmt.Errorf("expected non-zero exit from go stderr/exit command, got success\noutput:\n%s", output)
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
	if !strings.Contains(output, "exit status 17") {
		return fmt.Errorf("underlying program exit status not propagated in output\nexpected to find: exit status 17\noutput:\n%s", output)
	}
	return nil
}

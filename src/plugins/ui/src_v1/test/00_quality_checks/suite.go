package qualitychecks

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name:    "ui-quality-fmt-lint-build",
		Timeout: 2 * time.Minute,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			dialtoneScript := resolveDialtoneScript(sc.RepoRoot())
			commands := [][]string{
				{dialtoneScript, "ui", "src_v1", "install"},
				{dialtoneScript, "ui", "src_v1", "fmt-check"},
				{dialtoneScript, "ui", "src_v1", "lint"},
				{dialtoneScript, "ui", "src_v1", "build"},
			}
			for _, cmdArgs := range commands {
				if err := runAndLog(sc, cmdArgs); err != nil {
					return testv1.StepRunResult{}, err
				}
			}
			return testv1.StepRunResult{Report: "fmt-check, lint, and build passed"}, nil
		},
	})
}

func resolveDialtoneScript(start string) string {
	cur := strings.TrimSpace(start)
	if cur == "" {
		return "./dialtone.sh"
	}
	for {
		candidate := filepath.Join(cur, "dialtone.sh")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return "./dialtone.sh"
}

func runAndLog(sc *testv1.StepContext, cmdArgs []string) error {
	if len(cmdArgs) == 0 {
		return fmt.Errorf("empty command")
	}
	repoRoot := strings.TrimSpace(sc.RepoRoot())
	if repoRoot == "" {
		return fmt.Errorf("repo root unavailable in test context")
	}
	sc.Infof("running command: %s", strings.Join(cmdArgs, " "))
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = repoRoot
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		logOutput(sc, "stdout", stdout.String(), false)
		logOutput(sc, "stderr", stderr.String(), true)
		return fmt.Errorf("%s failed: %w", strings.Join(cmdArgs, " "), err)
	}
	logOutput(sc, "stdout", stdout.String(), false)
	logOutput(sc, "stderr", stderr.String(), false)
	return nil
}

func logOutput(sc *testv1.StepContext, stream string, output string, asError bool) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		sc.Infof("%s: <empty>", stream)
		return
	}
	scanner := bufio.NewScanner(strings.NewReader(trimmed))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if asError {
			sc.Errorf("%s: %s", stream, line)
			continue
		}
		sc.Infof("%s: %s", stream, line)
	}
}

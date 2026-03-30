package qualitychecks

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	uipaths "dialtone/dev/plugins/ui/src_v1/go"
)

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name:    "ui-quality-fmt-lint-build",
		Timeout: 2 * time.Minute,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			paths, err := uipaths.ResolvePaths(sc.RepoRoot())
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			goBin := strings.TrimSpace(paths.Runtime.GoBin)
			if goBin == "" {
				goBin = "go"
			}
			commands := [][]string{
				{goBin, "run", "./plugins/ui/scaffold/main.go", "src_v1", "install"},
				{goBin, "run", "./plugins/ui/scaffold/main.go", "src_v1", "fmt-check"},
				{goBin, "run", "./plugins/ui/scaffold/main.go", "src_v1", "lint"},
				{goBin, "run", "./plugins/ui/scaffold/main.go", "src_v1", "build"},
			}
			for _, cmdArgs := range commands {
				if err := runAndLog(sc, paths.Runtime.SrcRoot, cmdArgs); err != nil {
					return testv1.StepRunResult{}, err
				}
			}
			return testv1.StepRunResult{Report: "fmt-check, lint, and build passed"}, nil
		},
	})
}

func runAndLog(sc *testv1.StepContext, workdir string, cmdArgs []string) error {
	if len(cmdArgs) == 0 {
		return fmt.Errorf("empty command")
	}
	workdir = strings.TrimSpace(workdir)
	if workdir == "" {
		return fmt.Errorf("workdir unavailable in test context")
	}
	sc.Infof("running command: %s", strings.Join(cmdArgs, " "))
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = workdir
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

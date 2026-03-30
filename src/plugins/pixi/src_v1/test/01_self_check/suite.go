package selfcheck

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name: "pixi-cli-managed-bin-propagates-stdout",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if runtime.GOOS == "windows" {
				return testv1.StepRunResult{Report: "skipped on windows"}, nil
			}
			repoRoot, err := findRepoRoot()
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			tempEnv, fakePixi, err := fakeManagedPixi("pixi 0.test")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer os.RemoveAll(tempEnv)

			out, err := runDialtoneWithEnv(repoRoot, map[string]string{
				"DIALTONE_ENV":      tempEnv,
				"DIALTONE_PIXI_BIN": "",
			}, "pixi", "src_v1", "exec", "--version")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("expected success from pixi version command via %s: %w\noutput:\n%s", fakePixi, err, out)
			}
			if !strings.Contains(out, "pixi 0.test") {
				return testv1.StepRunResult{}, fmt.Errorf("stdout marker missing from pixi output")
			}
			if err := ctx.WaitForStepMessageAfterAction("pixi-stdout-ok", 3*time.Second, func() error {
				ctx.Infof("pixi-stdout-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "pixi stdout propagation verified from managed runtime"}, nil
		},
	})

	r.Add(testv1.Step{
		Name: "pixi-cli-stderr-exit-propagates",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if runtime.GOOS == "windows" {
				return testv1.StepRunResult{Report: "skipped on windows"}, nil
			}
			repoRoot, err := findRepoRoot()
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			tempEnv, _, err := fakeManagedPixi("")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer os.RemoveAll(tempEnv)

			out, err := runDialtoneWithEnv(repoRoot, map[string]string{
				"DIALTONE_ENV":      tempEnv,
				"DIALTONE_PIXI_BIN": "",
			}, "pixi", "src_v1", "exec", "fail")
			if err == nil {
				return testv1.StepRunResult{}, fmt.Errorf("expected non-zero exit from pixi fail command\noutput:\n%s", out)
			}
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				return testv1.StepRunResult{}, fmt.Errorf("expected ExitError, got %T: %v", err, err)
			}
			if exitErr.ExitCode() == 0 {
				return testv1.StepRunResult{}, fmt.Errorf("expected non-zero wrapper exit code")
			}
			if !strings.Contains(out, "PIXI_STDERR_MARKER_42") {
				return testv1.StepRunResult{}, fmt.Errorf("stderr marker missing from output")
			}
			if err := ctx.WaitForStepMessageAfterAction("pixi-stderr-exit-ok", 3*time.Second, func() error {
				ctx.Infof("pixi-stderr-exit-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "pixi stderr and non-zero exit propagation verified"}, nil
		},
	})
}

func fakeManagedPixi(versionOutput string) (string, string, error) {
	tempEnv, err := os.MkdirTemp("", "dialtone-pixi-env-*")
	if err != nil {
		return "", "", err
	}
	fakePixi := filepath.Join(tempEnv, "pixi", "bin", "pixi")
	if err := os.MkdirAll(filepath.Dir(fakePixi), 0o755); err != nil {
		_ = os.RemoveAll(tempEnv)
		return "", "", err
	}
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then\n" +
		fmt.Sprintf("  echo %q\n", versionOutput) +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"fail\" ]; then\n" +
		"  echo PIXI_STDERR_MARKER_42 1>&2\n" +
		"  exit 17\n" +
		"fi\n" +
		"echo PIXI_ARGS:$*\n"
	if err := os.WriteFile(fakePixi, []byte(script), 0o755); err != nil {
		_ = os.RemoveAll(tempEnv)
		return "", "", err
	}
	return tempEnv, fakePixi, nil
}

func runDialtoneWithEnv(repoRoot string, env map[string]string, args ...string) (string, error) {
	dialtoneSh := filepath.Join(repoRoot, "dialtone.sh")
	cmd := exec.Command(dialtoneSh, args...)
	cmd.Dir = repoRoot
	cmd.Env = applyEnv(os.Environ(), env)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func applyEnv(base []string, updates map[string]string) []string {
	keys := map[string]struct{}{}
	for key := range updates {
		keys[key] = struct{}{}
	}
	filtered := make([]string, 0, len(base)+len(updates))
	for _, entry := range base {
		key := entry
		if idx := strings.Index(entry, "="); idx >= 0 {
			key = entry[:idx]
		}
		if _, ok := keys[key]; ok {
			continue
		}
		filtered = append(filtered, entry)
	}
	for key, value := range updates {
		if strings.TrimSpace(value) == "" {
			continue
		}
		filtered = append(filtered, key+"="+value)
	}
	return filtered
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", fmt.Errorf("repo root not found")
		}
		cwd = parent
	}
}

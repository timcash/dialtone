package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	fmt.Println("Running test plugin suite (src_v1)...")

	broker, err := logs.StartEmbeddedNATS()
	if err != nil {
		fmt.Printf("FAIL: embedded nats start failed: %v\n", err)
		os.Exit(1)
	}
	defer broker.Close()

	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}
	logPath := filepath.Join(repoRoot, "src", "plugins", "test", "src_v1", "test", "test.log")
	_ = os.Remove(logPath)

	baseSubject := "logs.test.src_v1_ctx"
	stop, err := logs.ListenToFile(broker.Conn(), baseSubject+".>", logPath)
	if err != nil {
		fmt.Printf("FAIL: listen to file failed: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = stop() }()

	steps := []testv1.Step{
		{
			Name: "ctx-log-smoke",
			RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
				sc.Logf("ctx log info message")
				sc.Errorf("ctx log error message")
				return testv1.StepRunResult{Report: "ctx.Logf + ctx.Errorf emitted"}, nil
			},
		},
	}
	if err := testv1.RunSuite(testv1.SuiteOptions{
		Version:     "src_v1",
		NATSURL:     broker.URL(),
		NATSSubject: baseSubject,
	}, steps); err != nil {
		fmt.Printf("FAIL: RunSuite failed: %v\n", err)
		os.Exit(1)
	}

	if err := waitForContains(logPath, "ctx log info message", 4*time.Second); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}
	if err := waitForContains(logPath, "ctx log error message", 4*time.Second); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}
	if err := runTemplateExample(repoRoot, broker.URL(), baseSubject, logPath); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("PASS: test StepContext logging verified via NATS (subject_prefix=%s, file=%s)\n", baseSubject, logPath)
}

func runTemplateExample(repoRoot, natsURL, baseSubject, logPath string) error {
	subject := baseSubject + ".template"
	binPath := filepath.Join(repoRoot, "src", "plugins", "test", "src_v1", "test", "template_example_bin")
	_ = os.Remove(binPath)

	build := exec.Command("go", "build", "-o", binPath, "./plugins/test/src_v1/test/02_example_plugin_template")
	build.Dir = filepath.Join(repoRoot, "src")
	var buildOut bytes.Buffer
	build.Stdout = &buildOut
	build.Stderr = &buildOut
	if err := build.Run(); err != nil {
		return fmt.Errorf("template example build failed: %v\n%s", err, buildOut.String())
	}
	defer os.Remove(binPath)

	run := exec.Command(binPath, "--nats-url", natsURL, "--subject", subject)
	run.Dir = repoRoot
	var runOut bytes.Buffer
	run.Stdout = &runOut
	run.Stderr = &runOut
	if err := run.Run(); err != nil {
		return fmt.Errorf("template example run failed: %v\n%s", err, runOut.String())
	}
	if !strings.Contains(runOut.String(), "TEMPLATE_PLUGIN_PASS") {
		return fmt.Errorf("template pass marker missing:\n%s", runOut.String())
	}
	if err := waitForContains(logPath, "template plugin info", 4*time.Second); err != nil {
		return err
	}
	if err := waitForContains(logPath, "template plugin error", 4*time.Second); err != nil {
		return err
	}
	return nil
}

func waitForContains(path, pattern string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), pattern) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %q in %s", pattern, path)
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

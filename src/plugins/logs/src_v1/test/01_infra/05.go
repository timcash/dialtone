package infra

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Run05ExamplePluginImport(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return testv1.StepRunResult{}, err
	}
	testDir := filepath.Join(repoRoot, "src", "plugins", "logs", "src_v1", "test")

	topic := "logs.example.plugin"
	outPath := filepath.Join(testDir, "example_plugin.log")
	binPath := filepath.Join(testDir, "example_plugin_bin")

	_ = os.Remove(outPath)
	_ = os.Remove(binPath)

	build := exec.Command("go", "build", "-o", binPath, "./plugins/logs/src_v1/test/02_example")
	build.Dir = filepath.Join(repoRoot, "src")
	if out, err := build.CombinedOutput(); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("build example plugin failed: %v\n%s", err, string(out))
	}
	usedNatsURL := sc.NATSURL()
	if usedNatsURL == "" {
		usedNatsURL = "nats://127.0.0.1:4222"
	}
	nc := sc.NATSConn()
	if nc == nil {
		return testv1.StepRunResult{}, fmt.Errorf("NATS not available in test step context")
	}
	stopFile, err := logs.ListenToFile(nc, topic, outPath)
	if err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("listen to file failed: %w", err)
	}
	defer func() { _ = stopFile() }()

	run := exec.Command(binPath,
		"--nats-url", usedNatsURL,
		"--topic", topic,
		"--count", "4",
		"--out", outPath,
	)
	run.Dir = repoRoot
	if err := run.Start(); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("run example plugin failed: %v", err)
	}

	// Verify via "act then wait" on expected log messages.
	for i := 1; i <= 4; i++ {
		needle := fmt.Sprintf("example plugin message %d", i)
		if err := sc.WaitForMessage(topic, needle, 10*time.Second); err != nil {
			return testv1.StepRunResult{}, fmt.Errorf("missing message %d in NATS: %v", i, err)
		}
	}
	_ = run.Wait()

	// Verify the listener still worked (writing from topic to file)
	if err := waitForContains(outPath, "example plugin message 4", 4*time.Second); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("listener failed to write to file: %v", err)
	}

	return testv1.StepRunResult{Report: "Verified example plugin binary imports logs library and publishes expected messages."}, nil
}

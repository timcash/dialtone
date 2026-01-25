package test

import (
	"os"
	"os/exec"

	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/test"
)

func init() {
	test.Register("test-demo-ticket", "test", []string{"test", "demo", "ticket"}, RunDemoTicket)
	test.Register("test-demo-tags", "test", []string{"test", "demo", "tags"}, RunDemoTags)
	test.Register("test-demo-overlapping", "test", []string{"test", "demo", "overlapping"}, RunDemoOverlapping)
}

// RunAll runs self-tests for the test plugin.
func RunAll() error {
	logger.LogInfo("Running Test Plugin self-tests...")
	return test.RunTicket("test")
}

func RunDemoTicket() error {
	logger.LogInfo("Demo 2: Running ticket tests via './dialtone.sh test ticket test-test-tags'")
	return runDemoCommand("./dialtone.sh", "test", "ticket", "test-test-tags", "--list")
}

func RunDemoTags() error {
	logger.LogInfo("Demo 3: Running multiple tags via './dialtone.sh test tags metadata camera-filters'")
	return runDemoCommand("./dialtone.sh", "test", "tags", "metadata", "camera-filters")
}

func RunDemoOverlapping() error {
	logger.LogInfo("Demo 4: Running overlapping tags via './dialtone.sh test tags red-team'")
	return runDemoCommand("./dialtone.sh", "test", "tags", "red-team")
}

func runDemoCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

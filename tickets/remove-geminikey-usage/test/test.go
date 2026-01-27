package test

import (
	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/test"
	"fmt"
	"os/exec"
)

func init() {
	test.Register("verify-removal", "remove-geminikey-usage", []string{"cleanup"}, RunVerifyRemoval)
}

// RunAll is the standard entry point required by project rules.
func RunAll() error {
	logger.LogInfo("Running remove-geminikey-usage suite...")
	return RunVerifyRemoval()
}

func RunVerifyRemoval() error {
	logger.LogInfo("Checking for 'geminiKey' in src/plugins/ai/cli/...")

	// We'll use grep to find any occurrences
	out, err := exec.Command("grep", "-r", "geminiKey", "src/plugins/ai/cli/").CombinedOutput()
	if err == nil {
		// If grep returns 0, it found something, which is a failure
		return fmt.Errorf("FAIL: Found 'geminiKey' in source files:\n%s", string(out))
	}

	logger.LogInfo("PASS: No 'geminiKey' occurrences found.")
	return nil
}

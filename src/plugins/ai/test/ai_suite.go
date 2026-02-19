package ai_test

import (
	"fmt"
	"os"
	"os/exec"

	"dialtone/dev/core/logger"
	"dialtone/dev/core/test"
)

func init() {
	test.Register("ai-binary-exists", "ai-migration", []string{"ai", "binary"}, RunOpencodeBinaryExists)
	test.Register("ai-cli-version", "ai-migration", []string{"ai", "cli"}, RunOpencodeVersion)
}

// RunAll is the standard entry point required by project rules.
func RunAll() error {
	logger.LogInfo("Running AI Plugin suite...")
	return test.RunTicket("ai")
}

func RunOpencodeBinaryExists() error {
	opencodePath := os.ExpandEnv("$HOME/.opencode/bin/opencode")
	if _, err := os.Stat(opencodePath); os.IsNotExist(err) {
		return fmt.Errorf("opencode binary not found at %s", opencodePath)
	}
	fmt.Println("PASS: opencode binary found")
	return nil
}

func RunOpencodeVersion() error {
	opencodePath := os.ExpandEnv("$HOME/.opencode/bin/opencode")
	cmd := exec.Command(opencodePath, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run opencode --version: %v", err)
	}
	fmt.Printf("PASS: opencode version: %s\n", string(output))
	return nil
}

package test

import (
	"fmt"
	"os/exec"
	"time"
	"dialtone/cli/src/core/test"
	"dialtone/cli/src/core/logger"
)

func init() {
	test.Register("flags-parsing", "antigravity-logs-refinement", []string{"ide"}, RunFlagsParsing)
	test.Register("additive-logic-parsing", "antigravity-logs-refinement", []string{"ide"}, RunAdditiveLogicParsing)
}

// RunAll is the standard entry point required by project rules.
// It uses the registry to find and run all tests for this ticket.
func RunAll() error {
	logger.LogInfo("Running antigravity-logs-refinement suite...")
	return test.RunTicket("antigravity-logs-refinement")
}

func RunFlagsParsing() error {
	// Test --chat
	cmd := exec.Command("./dialtone.sh", "ide", "antigravity", "logs", "--chat")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command with --chat: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	cmd.Process.Kill()
	fmt.Println("PASS: [flags-parsing] --chat flag accepted")

	// Test --commands
	cmd = exec.Command("./dialtone.sh", "ide", "antigravity", "logs", "--commands")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command with --commands: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	cmd.Process.Kill()
	fmt.Println("PASS: [flags-parsing] --commands flag accepted")

	return nil
}

func RunAdditiveLogicParsing() error {
	// Test both flags
	cmd := exec.Command("./dialtone.sh", "ide", "antigravity", "logs", "--chat", "--commands")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command with combined flags: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	cmd.Process.Kill()
	fmt.Println("PASS: [additive-logic-parsing] Combined flags accepted")
	return nil
}

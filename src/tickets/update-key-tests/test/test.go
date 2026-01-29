package test

import (
	"dialtone/cli/src/dialtest"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func init() {
	dialtest.RegisterTicket("update-key-tests")
	dialtest.AddSubtaskTest("init", func() error { return nil }, nil)
	dialtest.AddSubtaskTest("promote-key-cmd", func() error {
		output, _ := exec.Command("./dialtone.sh", "key").CombinedOutput()
		if !strings.Contains(string(output), "Usage: ./dialtone.sh ticket key") {
			return fmt.Errorf("usage not found in output: %s", string(output))
		}
		return nil
	}, nil)
	dialtest.AddSubtaskTest("update-integration-tests", func() error {
		content, err := os.ReadFile("src/plugins/ticket/test/integration.go")
		if err != nil {
			return err
		}
		if !strings.Contains(string(content), "TestKeyWorkflow") {
			return fmt.Errorf("TestKeyWorkflow not found in integration.go")
		}
		return nil
	}, nil)
	dialtest.AddSubtaskTest("update-docs", func() error {
		content, err := os.ReadFile("src/plugins/ticket/README.md")
		if err != nil {
			return err
		}
		if !strings.Contains(string(content), "./dialtone.sh key add") {
			return fmt.Errorf("key add docs not found in README.md")
		}
		return nil
	}, nil)
	dialtest.AddSubtaskTest("verify-workflow", func() error {
		// Use a temporary DB for integration tests to avoid wiping the main one
		os.Setenv("TICKET_DB_PATH", "src/tickets/test_tickets.duckdb")
		defer os.Unsetenv("TICKET_DB_PATH")

		cmd := exec.Command("./dialtone.sh", "plugin", "test", "ticket")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("integration tests failed: %v, output: %s", err, string(output))
		}
		if !strings.Contains(string(output), "Key Management Workflow") || !strings.Contains(string(output), "PASS") {
			return fmt.Errorf("Key workflow test not passed in output: %s", string(output))
		}
		return nil
	}, nil)
}

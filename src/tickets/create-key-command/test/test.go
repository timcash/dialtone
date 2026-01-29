package test

import (
	"dialtone/cli/src/dialtest"
	"fmt"
	"os/exec"
	"strings"
)

func init() {
	dialtest.RegisterTicket("create-key-command")
	dialtest.AddSubtaskTest("init", func() error { return nil }, nil)
	dialtest.AddSubtaskTest("storage-schema", func() error { return nil }, nil)
	dialtest.AddSubtaskTest("crypto-utils", func() error { return nil }, nil)

	dialtest.AddSubtaskTest("cli-routing", func() error {
		cmd := exec.Command("./dialtone.sh", "ticket", "key")
		output, _ := cmd.CombinedOutput()
		if !strings.Contains(string(output), "Usage: ./dialtone.sh ticket key") {
			return fmt.Errorf("usage not found in output: %s", string(output))
		}
		return nil
	}, nil)

	dialtest.AddSubtaskTest("cmd-add", func() error {
		cmd := exec.Command("./dialtone.sh", "ticket", "key", "add", "testkey", "secretvalue", "pass123")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("cmd-add failed: %v, output: %s", err, string(output))
		}
		if !strings.Contains(string(output), "stored securely") {
			return fmt.Errorf("unexpected output: %s", string(output))
		}
		return nil
	}, nil)

	dialtest.AddSubtaskTest("cmd-lease", func() error {
		cmd := exec.Command("./dialtone.sh", "ticket", "key", "testkey", "pass123")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("cmd-lease failed: %v, output: %s", err, string(output))
		}
		if string(output) != "secretvalue" {
			return fmt.Errorf("expected 'secretvalue', got '%s'", string(output))
		}
		return nil
	}, nil)

	dialtest.AddSubtaskTest("cmd-list", func() error {
		cmd := exec.Command("./dialtone.sh", "ticket", "key", "list")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("cmd-list failed: %v, output: %s", err, string(output))
		}
		if !strings.Contains(string(output), "testkey") {
			return fmt.Errorf("key not found in list output: %s", string(output))
		}
		return nil
	}, nil)

	dialtest.AddSubtaskTest("cmd-rm", func() error {
		cmd := exec.Command("./dialtone.sh", "ticket", "key", "rm", "testkey")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("cmd-rm failed: %v, output: %s", err, string(output))
		}
		if !strings.Contains(string(output), "removed") {
			return fmt.Errorf("unexpected output: %s", string(output))
		}
		return nil
	}, nil)
}

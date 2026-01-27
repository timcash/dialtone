package test

import (
	"fmt"
	"dialtone/cli/src/core/test"
	"dialtone/cli/src/core/logger"
)

func init() {
	// Register subtask tests here: test.Register("<subtask-name>", "<ticket-name>", []string{"<tag1>"}, Run<SubtaskName>)
	test.Register("example-subtask", "replace-progress-with-ticket-next", []string{"example"}, RunExample)
}

// RunAll is the standard entry point required by project rules.
// It uses the registry to find and run all tests for this ticket.
func RunAll() error {
	logger.LogInfo("Running replace-progress-with-ticket-next suite...")
	return test.RunTicket("replace-progress-with-ticket-next")
}

func RunExample() error {
	fmt.Println("PASS: [example] Subtask logic verified")
	return nil
}

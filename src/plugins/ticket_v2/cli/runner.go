package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// runDynamicTest executes a subtask test (or all tests) for a ticket by generating
// a temporary Go file that imports the ticket's test package.
func runDynamicTest(ticketID, subtaskName string) error {
	tmpDir, err := os.MkdirTemp("", "dialtest-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "main.go")
	
	// Create the temporary main.go
	// We import the ticket's test package to trigger its init() function.
	content := fmt.Sprintf(`package main

import (
	"dialtone/cli/src/dialtest"
	_ "dialtone/cli/src/tickets_v2/%s/test"
	"os"
	"flag"
)

func main() {
	ticket := "%s"
	subtask := flag.String("subtask", "", "Subtask name")
	flag.Parse()

	if *subtask != "" {
		if err := dialtest.RunSubtask(ticket, *subtask); err != nil {
			os.Exit(1)
		}
	} else {
		tests := dialtest.GetAllTests(ticket)
		failed := false
		for _, t := range tests {
			if err := dialtest.RunSubtask(ticket, t.Name); err != nil {
				failed = true
			}
		}
		if failed {
			os.Exit(1)
		}
	}
}
`, ticketID, ticketID)

	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write temp main.go: %v", err)
	}

	args := []string{"run", tmpFile}
	if subtaskName != "" {
		args = append(args, "-subtask", subtaskName)
	}

	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Use the same environment as dialtone.sh if needed, but 'go run' should work if .env is set up correctly in the shell.
	
	return cmd.Run()
}

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runGoSubtaskTest(ticketName, subtaskName string) error {
	tmpDir, err := os.MkdirTemp("", "dialtest-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	mainGoPath := filepath.Join(tmpDir, "main.go")

	// We use blank import to trigger init() in the ticket's test package
	mainContent := fmt.Sprintf(`package main
import (
	"dialtone/cli/src/dialtest"
	_ "dialtone/cli/src/tickets/%s/test"
	"os"
	"fmt"
)
func main() {
	err := dialtest.RunSubtask("%s")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%%v\n", err)
		os.Exit(1)
	}
}
`, ticketName, subtaskName)

	err = os.WriteFile(mainGoPath, []byte(mainContent), 0644)
	if err != nil {
		return err
	}

	cmd := exec.Command("go", "run", mainGoPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runDynamicTest(ticketName, subtaskName string) error {
	if subtaskName != "" {
		return runSubtaskCommandTest(ticketName, subtaskName)
	}
	tmpDir, err := os.MkdirTemp("", "dialtest-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	mainGoPath := filepath.Join(tmpDir, "main.go")

	// We use blank import to trigger init() in the ticket's test package
	mainContent := fmt.Sprintf(`package main
import (
	"dialtone/cli/src/dialtest"
	_ "dialtone/cli/src/tickets/%s/test"
	"os"
	"fmt"
)
func main() {
	err := dialtest.RunSubtask("%s")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%%v\n", err)
		os.Exit(1)
	}
}
`, ticketName, subtaskName)

	if subtaskName == "" {
		// Run all if subtask is empty
		mainContent = fmt.Sprintf(`package main
import (
	"dialtone/cli/src/dialtest"
	_ "dialtone/cli/src/tickets/%s/test"
	"os"
	"fmt"
)
func main() {
	registry := dialtest.GetRegistry()
	hasFail := false
	for _, t := range registry {
		fmt.Printf("[dialtest] Running test for subtask: %%s\n", t.Name)
		if err := t.Fn(); err != nil {
			fmt.Printf("[dialtest] FAIL: %%s - %%v\n", t.Name, err)
			hasFail = true
		} else {
			fmt.Printf("[dialtest] PASS: %%s\n", t.Name)
		}
	}
	if hasFail {
		os.Exit(1)
	}
}
`, ticketName)
	}

	err = os.WriteFile(mainGoPath, []byte(mainContent), 0644)
	if err != nil {
		return err
	}

	cmd := exec.Command("go", "run", mainGoPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runSubtaskCommandTest(ticketName, subtaskName string) error {
	ticket, err := GetTicket(ticketName)
	if err != nil {
		return err
	}
	for _, st := range ticket.Subtasks {
		if st.Name == subtaskName {
			// Prefer explicit shell test command, but fall back to ticket-local Go test
			// (the scaffolded src/tickets/<ticket>/test/test.go) if no command is provided.
			if strings.TrimSpace(st.TestCommand) == "" {
				return runGoSubtaskTest(ticketName, subtaskName)
			}
			return runShellCommand(st.TestCommand)
		}
	}
	return fmt.Errorf("subtask not found: %s", subtaskName)
}

func runShellCommand(command string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

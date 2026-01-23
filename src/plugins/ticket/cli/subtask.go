package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Subtask represents a single subtask parsed from ticket.md
type Subtask struct {
	Name            string // kebab-case name
	Description     string // Description of the task
	TestDescription string // Description of how to test
	TestCommand     string // Command to run the test
	Status          string // todo, progress, done
	LineNumber      int    // Line number in the file where "status: ..." is defined
}

// RunSubtask is the entry point for 'ticket subtask ...'
func RunSubtask(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ticket subtask <list|next|test|done> <ticket-name> [subtask-name]")
	}

	subcmd := args[0]
	subArgs := args[1:]

	if len(subArgs) < 1 {
		logFatal("Usage: ticket subtask %s <ticket-name>", subcmd)
	}

	ticketName := GetTicketName(subArgs[0])

	switch subcmd {
	case "list":
		RunSubtaskList(ticketName)
	case "next":
		RunSubtaskNext(ticketName)
	case "test":
		if len(subArgs) < 2 {
			logFatal("Usage: ticket subtask test <ticket-name> <subtask-name>")
		}
		RunSubtaskTest(ticketName, subArgs[1])
	case "done":
		if len(subArgs) < 2 {
			logFatal("Usage: ticket subtask done <ticket-name> <subtask-name>")
		}
		RunSubtaskDone(ticketName, subArgs[1])
	default:
		logFatal("Unknown subtask command: %s", subcmd)
	}

	logReminder(ticketName)
}

// RunSubtaskList parses ticket.md and lists all subtasks
func RunSubtaskList(ticketName string) {
	subtasks, err := parseSubtasks(ticketName)
	if err != nil {
		logFatal("Failed to parse subtasks: %v", err)
	}

	if len(subtasks) == 0 {
		logInfo("No subtasks found for ticket %s", ticketName)
		return
	}

	fmt.Printf("\nSubtasks for %s:\n", ticketName)
	fmt.Println("---------------------------------------------------")
	for _, st := range subtasks {
		icon := "[ ]"
		if st.Status == "done" {
			icon = "[x]"
		} else if st.Status == "progress" {
			icon = "[/]"
		}
		fmt.Printf("%s %s (%s)\n", icon, st.Name, st.Status)
	}
	fmt.Println("---------------------------------------------------")
}

// RunSubtaskNext finds the first non-done subtask and prints it
func RunSubtaskNext(ticketName string) {
	subtasks, err := parseSubtasks(ticketName)
	if err != nil {
		logFatal("Failed to parse subtasks: %v", err)
	}

	for _, st := range subtasks {
		if st.Status == "todo" || st.Status == "progress" {
			fmt.Printf("\nNext Subtask:\n")
			fmt.Printf("Name:        %s\n", st.Name)
			fmt.Printf("Description: %s\n", st.Description)
			fmt.Printf("Test:        %s\n", st.TestCommand)
			fmt.Printf("Status:      %s\n", st.Status)
			return
		}
	}

	logInfo("No pending subtasks found. All done!")
}

// RunSubtaskTest runs the test command for a specific subtask
func RunSubtaskTest(ticketName, subtaskName string) {
	subtasks, err := parseSubtasks(ticketName)
	if err != nil {
		logFatal("Failed to parse subtasks: %v", err)
	}

	var target *Subtask
	for _, st := range subtasks {
		if st.Name == subtaskName {
			target = &st
			break
		}
	}

	if target == nil {
		logFatal("Subtask '%s' not found in ticket %s", subtaskName, ticketName)
	}

	logInfo("Running test for subtask: %s", subtaskName)
	logInfo("Command: %s", target.TestCommand)

	// Execute the test command
	// The command is likely "dialtone.sh ticket test ..." or similar
	parts := strings.Fields(target.TestCommand)
	if len(parts) == 0 {
		logFatal("Empty test command for subtask %s", subtaskName)
	}

	// If it starts with ./dialtone.sh, we might want to just run it directly
	cmdName := parts[0]
	if cmdName == "dialtone.sh" {
		cmdName = "./dialtone.sh"
	}

	cmd := exec.Command(cmdName, parts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		logFatal("Test failed: %v", err)
	}

	logInfo("Test passed for subtask: %s", subtaskName)
}

// RunSubtaskDone marks a subtask as done
func RunSubtaskDone(ticketName, subtaskName string) {
	subtasks, err := parseSubtasks(ticketName)
	if err != nil {
		logFatal("Failed to parse subtasks: %v", err)
	}

	var target *Subtask
	for _, st := range subtasks {
		if st.Name == subtaskName {
			target = &st
			break
		}
	}

	if target == nil {
		logFatal("Subtask '%s' not found in ticket %s", subtaskName, ticketName)
	}

	if target.Status == "done" {
		logInfo("Subtask '%s' is already marked as done.", subtaskName)
		return
	}

	// Update the file
	ticketMdPath := filepath.Join("tickets", ticketName, "ticket.md")
	content, err := os.ReadFile(ticketMdPath)
	if err != nil {
		logFatal("Failed to read ticket.md: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	
	// Just strict replace on the specific line we found earlier
	// Note: LineNumber is 1-based, array is 0-based
	if target.LineNumber > 0 && target.LineNumber <= len(lines) {
		lines[target.LineNumber-1] = strings.Replace(lines[target.LineNumber-1], target.Status, "done", 1)
	} else {
		logFatal("Could not find line %d in ticket.md", target.LineNumber)
	}

	if err := os.WriteFile(ticketMdPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		logFatal("Failed to update ticket.md: %v", err)
	}

	logInfo("Marked subtask '%s' as done.", subtaskName)
}

// parseSubtasks reads ticket.md and extracts subtasks
func parseSubtasks(ticketName string) ([]Subtask, error) {
	ticketMdPath := filepath.Join("tickets", ticketName, "ticket.md")
	if _, err := os.Stat(ticketMdPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("ticket file not found: %s", ticketMdPath)
	}

	content, err := os.ReadFile(ticketMdPath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	var subtasks []Subtask
	var currentSubtask *Subtask

	// Simple state machine parser
	// Looks for "## SUBTASK: Title" start
	// Then parses bullet points
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if strings.HasPrefix(trimmed, "## SUBTASK:") {
			// Save previous if exists
			if currentSubtask != nil {
				subtasks = append(subtasks, *currentSubtask)
			}
			// Start new
			currentSubtask = &Subtask{}
		} else if currentSubtask != nil {
			if strings.HasPrefix(trimmed, "- name:") {
				currentSubtask.Name = strings.TrimSpace(strings.TrimPrefix(trimmed, "- name:"))
			} else if strings.HasPrefix(trimmed, "- description:") {
				currentSubtask.Description = strings.TrimSpace(strings.TrimPrefix(trimmed, "- description:"))
			} else if strings.HasPrefix(trimmed, "- test-description:") {
				currentSubtask.TestDescription = strings.TrimSpace(strings.TrimPrefix(trimmed, "- test-description:"))
			} else if strings.HasPrefix(trimmed, "- test-command:") {
				// Handle code blocks `...` if present
				cmd := strings.TrimSpace(strings.TrimPrefix(trimmed, "- test-command:"))
				cmd = strings.Trim(cmd, "`")
				currentSubtask.TestCommand = cmd
			} else if strings.HasPrefix(trimmed, "- status:") {
				currentSubtask.Status = strings.TrimSpace(strings.TrimPrefix(trimmed, "- status:"))
				currentSubtask.LineNumber = i + 1
			}
		}
	}
	// Append last one
	if currentSubtask != nil {
		subtasks = append(subtasks, *currentSubtask)
	}

	return subtasks, nil
}

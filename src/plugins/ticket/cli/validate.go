package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RunValidate checks the format of a ticket
func RunValidate(args []string) {
	if len(args) < 1 {
		ticketName := GetCurrentBranch()
		if ticketName == "" {
			logFatal("Usage: ticket validate <ticket-name> (or run from a feature branch)")
		}
		args = []string{ticketName}
	}
	ticketName := args[0]

	logInfo("Validating ticket: %s", ticketName)

	ticketMdPath := filepath.Join("tickets", ticketName, "ticket.md")
	if _, err := os.Stat(ticketMdPath); os.IsNotExist(err) {
		logFatal("Ticket file not found: %s", ticketMdPath)
	}

	contentBytes, err := os.ReadFile(ticketMdPath)
	if err != nil {
		logFatal("Failed to read ticket file: %v", err)
	}
	content := string(contentBytes)
	lines := strings.Split(content, "\n")

	errors := []string{}
	subtaskCount := 0

	// State machine
	var currentSubtaskName string
	var statusLineFound bool
	var subtaskHeaderFound bool

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "## SUBTASK:") {
			// Check previous subtask
			if subtaskHeaderFound {
				if !statusLineFound {
					errors = append(errors, fmt.Sprintf("Subtask '%s' is missing a '- status:' line", currentSubtaskName))
				}
			}

			// Start new
			subtaskHeaderFound = true
			subtaskCount++
			currentSubtaskName = strings.TrimSpace(strings.TrimPrefix(trimmed, "## SUBTASK:"))
			statusLineFound = false

			if currentSubtaskName == "" {
				errors = append(errors, fmt.Sprintf("Line %d: Empty subtask name", i+1))
			}

		} else if strings.HasPrefix(trimmed, "- status:") {
			if !subtaskHeaderFound {
				errors = append(errors, fmt.Sprintf("Line %d: Status line found outside of a subtask", i+1))
			}

			if statusLineFound {
				errors = append(errors, fmt.Sprintf("Subtask '%s' has multiple status lines (see line %d)", currentSubtaskName, i+1))
			}
			statusLineFound = true

			status := strings.TrimSpace(strings.TrimPrefix(trimmed, "- status:"))
			if status != "todo" && status != "progress" && status != "done" {
				errors = append(errors, fmt.Sprintf("Line %d: Invalid status '%s'. Must be todo, progress, or done.", i+1, status))
			}
		}
	}

	// Check last subtask
	if subtaskHeaderFound {
		if !statusLineFound {
			errors = append(errors, fmt.Sprintf("Subtask '%s' is missing a '- status:' line", currentSubtaskName))
		}
	} else {
		errors = append(errors, "No subtasks found in ticket.md")
	}

	if len(errors) > 0 {
		fmt.Println("\nValidation Failed:")
		for _, e := range errors {
			fmt.Printf("- %s\n", e)
		}
		os.Exit(1)
	}

	logInfo("Validation successful! (%d subtasks)", subtaskCount)
}

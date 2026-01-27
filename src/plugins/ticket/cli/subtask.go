package cli

import (
	"bytes"
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
		logFatal("Usage: ticket subtask <list|next|test|done|failed> [<ticket-name>] [<subtask-name>]")
	}

	subcmd := args[0]
	subArgs := args[1:]

	ticketName := ""
	subtaskName := ""

	if len(subArgs) >= 1 {
		// Might be <ticket-name> or <subtask-name>
		// Use a simple heuristic: if it contains a ticket name, use it.
		// For now, let's assume if it exists in tickets/, it's a ticket.
		val := GetTicketName(subArgs[0])
		if _, err := os.Stat(filepath.Join("tickets", val)); err == nil {
			ticketName = val
			if len(subArgs) >= 2 {
				subtaskName = subArgs[1]
			}
		} else {
			// Try as subtask name with current branch
			ticketName = GetCurrentBranch()
			subtaskName = val
		}
	} else {
		ticketName = GetCurrentBranch()
	}

	if ticketName == "" {
		logFatal("Usage: ticket subtask <list|next|test|done|failed> [<ticket-name>] [<subtask-name>]\n(You must specify a ticket name or be on a feature branch)")
	}

	switch subcmd {
	case "list":
		RunSubtaskList(ticketName)
	case "next":
		RunSubtaskNext(ticketName)
	case "test":
		if subtaskName == "" {
			logFatal("Usage: ticket subtask test [<ticket-name>] <subtask-name>")
		}
		RunSubtaskTest(ticketName, subtaskName)
	case "done":
		if subtaskName == "" {
			logFatal("Usage: ticket subtask done [<ticket-name>] <subtask-name>")
		}
		RunSubtaskDone(ticketName, subtaskName)
	case "failed":
		if subtaskName == "" {
			logFatal("Usage: ticket subtask failed [<ticket-name>] <subtask-name>")
		}
		RunSubtaskFailed(ticketName, subtaskName)
	default:
		logFatal("Unknown subtask command: %s", subcmd)
	}

	// logReminder is removed
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
		} else if st.Status == "failed" {
			icon = "[!]"
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
	if err := runSubtaskTestInternal(ticketName, subtaskName); err != nil {
		logFatal("Test failed: %v", err)
	}
}

func runSubtaskTestInternal(ticketName, subtaskName string) error {
	subtasks, err := parseSubtasks(ticketName)
	if err != nil {
		return fmt.Errorf("failed to parse subtasks: %v", err)
	}

	var target *Subtask
	for _, st := range subtasks {
		if st.Name == subtaskName {
			target = &st
			break
		}
	}

	if target == nil {
		return fmt.Errorf("subtask '%s' not found in ticket %s", subtaskName, ticketName)
	}

	logInfo("Running test for subtask: %s", subtaskName)
	logInfo("Command: %s", target.TestCommand)

	parts := strings.Fields(target.TestCommand)
	if len(parts) == 0 {
		return fmt.Errorf("empty test command for subtask %s", subtaskName)
	}

	cmdName := parts[0]
	var cmd *exec.Cmd
	if cmdName == "./dialtone.sh" || cmdName == "dialtone.sh" {
		cmd = exec.Command("./dialtone.sh", parts[1:]...)
	} else {
		cmd = exec.Command("sh", "-c", target.TestCommand)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return err
	}

	logInfo("Test passed for subtask: %s", subtaskName)
	return nil
}

// RunSubtaskDone marks a subtask as done
func RunSubtaskDone(ticketName, subtaskName string) {
	validateGitState(ticketName)
	updateSubtaskStatus(ticketName, subtaskName, "done")
	logInfo("Marked subtask '%s' as done.", subtaskName)
}

// RunSubtaskFailed marks a subtask as failed
func RunSubtaskFailed(ticketName, subtaskName string) {
	// For failed, we still enforce Git checks to ensure consistency and history.
	validateGitState(ticketName)
	updateSubtaskStatus(ticketName, subtaskName, "failed")
	logInfo("Marked subtask '%s' as failed.", subtaskName)
}

func updateSubtaskStatus(ticketName, subtaskName, newStatus string) {
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

	if target.Status == newStatus {
		return
	}

	// Update the file
	ticketMdPath := filepath.Join("tickets", ticketName, "ticket.md")
	content, err := os.ReadFile(ticketMdPath)
	if err != nil {
		logFatal("Failed to read ticket.md: %v", err)
	}

	lines := strings.Split(string(content), "\n")

	if target.LineNumber > 0 && target.LineNumber <= len(lines) {
		lines[target.LineNumber-1] = strings.Replace(lines[target.LineNumber-1], target.Status, newStatus, 1)
	} else {
		logFatal("Could not find line %d in ticket.md", target.LineNumber)
	}

	if err := os.WriteFile(ticketMdPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		logFatal("Failed to update ticket.md: %v", err)
	}
}

func validateGitState(ticketName string) {
	if os.Getenv("DIALTONE_DISABLE_GIT_CHECKS") == "1" {
		return
	}

	// Check Git Cleanliness
	if !isGitClean() {
		logFatal("Error: Git repository is not clean.\nPlease commit all your changes before proceeding.")
	}
}

// isProgressUpdated is deprecated

func isGitClean() bool {
	cmd := exec.Command("git", "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		// If git fails, assume not clean or weird state
		return false
	}
	return len(bytes.TrimSpace(out)) == 0
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

// RunTicketNext handles the automated TDD loop
func RunTicketNext(args []string) {
	ticketName := ""
	if len(args) > 0 {
		ticketName = GetTicketName(args[0])
	} else {
		ticketName = GetCurrentBranch()
	}

	if ticketName == "" {
		fmt.Println("Error: 'ticket next' cannot be run from the main/master branch.")
		fmt.Println("Please ask the USER for the next ticket to work on or create a new one.")
		os.Exit(1)
	}

	// 1. Validate the ticket format first
	if err := validateTicketInternal(ticketName); err != nil {
		logFatal("Ticket validation failed: %v", err)
	}

	subtasks, err := parseSubtasks(ticketName)
	if err != nil {
		logFatal("Failed to parse subtasks: %v", err)
	}

	// 2. Find subtask in progress
	var progressSubtask *Subtask
	for _, st := range subtasks {
		if st.Status == "progress" {
			progressSubtask = &st
			break
		}
	}

	// 3. If in progress, run its test
	if progressSubtask != nil {
		logInfo("Checking progress of subtask: %s", progressSubtask.Name)
		if err := runSubtaskTestInternal(ticketName, progressSubtask.Name); err != nil {
			logInfo("Subtask '%s' is still in progress (test failed).", progressSubtask.Name)
			RunSubtaskList(ticketName)
			fmt.Printf("\nNext Subtask (CONTINUE):\n")
			fmt.Printf("Name:        %s\n", progressSubtask.Name)
			fmt.Printf("Description: %s\n", progressSubtask.Description)
			fmt.Printf("Test:        %s\n", progressSubtask.TestCommand)
			return
		}

		// Test passed, mark as done
		updateSubtaskStatus(ticketName, progressSubtask.Name, "done")
		logInfo("Subtask '%s' PASSED and marked as DONE.", progressSubtask.Name)

		// Re-parse to get updated state
		subtasks, _ = parseSubtasks(ticketName)
	}

	// 4. Find next subtask
	var nextSubtask *Subtask
	for _, st := range subtasks {
		if st.Status == "todo" || st.Status == "progress" {
			nextSubtask = &st
			break
		}
	}

	if nextSubtask == nil {
		logInfo("All subtasks are COMPLETE! You are ready to run 'dialtone.sh ticket done'.")
		RunSubtaskList(ticketName)
		return
	}

	// 5. If it was todo, mark as progress
	if nextSubtask.Status == "todo" {
		updateSubtaskStatus(ticketName, nextSubtask.Name, "progress")
		logInfo("Starting next subtask: %s", nextSubtask.Name)
	}

	// 6. Final Status List and Next Info
	RunSubtaskList(ticketName)
	fmt.Printf("\nNext Subtask:\n")
	fmt.Printf("Name:        %s\n", nextSubtask.Name)
	fmt.Printf("Description: %s\n", nextSubtask.Description)
	fmt.Printf("Test:        %s\n", nextSubtask.TestCommand)
	fmt.Printf("Status:      progress\n")
}

func validateTicketInternal(ticketName string) error {
	// Re-use logic from RunValidate but return error instead of exit
	subtasks, err := parseSubtasks(ticketName)
	if err != nil {
		return err
	}
	if len(subtasks) == 0 {
		return fmt.Errorf("no subtasks found in ticket.md")
	}
	return nil
}

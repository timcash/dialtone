package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func RunSubtask(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: ./dialtone.sh ticket subtask <command> [args]")
		return
	}

	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "list":
		logSubtaskCommand(command, cmdArgs)
		RunSubtaskList(cmdArgs)
	case "test":
		logSubtaskCommand(command, cmdArgs)
		RunSubtaskTestCmd(cmdArgs)
	case "done":
		logSubtaskCommand(command, cmdArgs)
		RunSubtaskDone(cmdArgs)
	case "failed":
		logSubtaskCommand(command, cmdArgs)
		RunSubtaskFailed(cmdArgs)
	case "":
		// If no command provided, print next incomplete
		logSubtaskCommand(command, cmdArgs)
		ticket, err := GetCurrentTicket()
		if err == nil {
			st := FindNextSubtask(ticket)
			if st != nil {
				fmt.Printf("Next Subtask: %s (%s)\n", st.Name, st.Status)
			}
		}
	default:
		logInfo("Unknown subtask command: %s", command)
	}
}

func RunSubtaskList(args []string) {
	var ticket *Ticket
	var err error
	if len(args) > 0 {
		path := filepath.Join("src", "tickets", args[0], "ticket.md")
		ticket, err = ParseTicketMd(path)
	} else {
		ticket, err = GetCurrentTicket()
	}

	if err != nil {
		logFatal("Error: %v", err)
	}
	PrintTicketReport(ticket)
}

func RunNext(args []string) {
	var ticket *Ticket
	var err error
	var ticketID string

	if len(args) > 0 {
		ticketID = args[0]
		path := filepath.Join("src", "tickets", ticketID, "ticket.md")
		ticket, err = ParseTicketMd(path)
	} else {
		ticket, err = GetCurrentTicket()
		if ticket != nil {
			ticketID = ticket.ID
		}
	}

	if err != nil {
		logFatal("Error: %v", err)
	}

	logTicketCommand(ticketID, "next", args)

	st := FindNextSubtask(ticket)
	if st == nil {
		logInfo("No eligible subtasks found. Ticket might be complete.")
		PrintTicketReport(ticket)
		return
	}

	if st.Status == "todo" {
		st.Status = "progress"
		logInfo("Promoting subtask %s to progress", st.Name)
		WriteTicketMd(filepath.Join("src", "tickets", ticketID, "ticket.md"), ticket)
	}

	logInfo("Executing test for subtask: %s", st.Name)
	err = runDynamicTest(ticketID, st.Name)
	
	// Reload
	path := filepath.Join("src", "tickets", ticketID, "ticket.md")
	ticket, _ = ParseTicketMd(path)
	st = nil
	for i := range ticket.Subtasks {
		if ticket.Subtasks[i].Status == "progress" {
			st = &ticket.Subtasks[i]
		}
	}

	if err == nil {
		logInfo("Subtask %s passed!", st.Name)
		st.Status = "done"
		st.PassTimestamp = time.Now().Format(time.RFC3339)
		WriteTicketMd(filepath.Join("src", "tickets", ticketID, "ticket.md"), ticket)
		
		// Auto-commit on Pass
		commitMsg := fmt.Sprintf("docs: subtask %s passed", st.Name)
		addCmd := exec.Command("git", "add", filepath.Join("src", "tickets", ticketID, "ticket.md"))
		if addOutput, err := addCmd.CombinedOutput(); err != nil {
			logFatal("Git add failed: %v\nOutput: %s", err, string(addOutput))
		}
		commitCmd := exec.Command("git", "commit", "-m", commitMsg)
		if commitOutput, err := commitCmd.CombinedOutput(); err != nil {
			logFatal("Git commit failed: %v\nOutput: %s", err, string(commitOutput))
		}

		// Recurse to next task with same ID
		RunNext([]string{ticketID})
	} else {
		logInfo("Subtask %s failed.", st.Name)
		st.FailTimestamp = time.Now().Format(time.RFC3339)
		st.AgentNotes = err.Error()
		WriteTicketMd(filepath.Join("src", "tickets", ticketID, "ticket.md"), ticket)
		PrintTicketReport(ticket)
	}
}

func RunSubtaskTestCmd(args []string) {
	if len(args) < 2 {
		logFatal("Usage: ./dialtone.sh ticket subtask test <ticket-name> <subtask-name>")
	}
	ticketName := args[0]
	subtaskName := args[1]
	err := runDynamicTest(ticketName, subtaskName)
	if err != nil {
		logFatal("Test failed: %v", err)
	}
	logInfo("Test passed!")
}

func RunTest(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ./dialtone.sh ticket test <ticket-name>")
	}
	name := args[0]
	logTicketCommand(name, "test", args)
	logInfo("Testing all subtasks for %s...", name)
	err := runDynamicTest(name, "")
	if err != nil {
		logFatal("Tests failed: %v", err)
	}
	logInfo("All tests passed!")
}

func RunSubtaskDone(args []string) {
	if len(args) < 2 {
		logFatal("Usage: ./dialtone.sh ticket subtask done <ticket-name> <subtask-name>")
	}
	name := args[0]
	subtask := args[1]

	// Check git hygiene
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusOutput, _ := statusCmd.Output()
	if len(strings.TrimSpace(string(statusOutput))) > 0 {
		logFatal("Git status is not clean. Please commit or stash changes before running 'subtask done'.")
	}

	ticket, _ := ParseTicketMd(filepath.Join("src", "tickets", name, "ticket.md"))
	for i := range ticket.Subtasks {
		if ticket.Subtasks[i].Name == subtask {
			ticket.Subtasks[i].Status = "done"
			ticket.Subtasks[i].PassTimestamp = time.Now().Format(time.RFC3339)
		}
	}
	ticketPath := filepath.Join("src", "tickets", name, "ticket.md")
	WriteTicketMd(ticketPath, ticket)

	addCmd := exec.Command("git", "add", ticketPath)
	if addOutput, err := addCmd.CombinedOutput(); err != nil {
		logFatal("Git add failed: %v\nOutput: %s", err, string(addOutput))
	}
	commitCmd := exec.Command("git", "commit", "-m", fmt.Sprintf("docs: subtask %s done", subtask))
	if commitOutput, err := commitCmd.CombinedOutput(); err != nil {
		logFatal("Git commit failed: %v\nOutput: %s", err, string(commitOutput))
	}
}

func RunSubtaskFailed(args []string) {
	if len(args) < 2 {
		logFatal("Usage: ./dialtone.sh ticket subtask failed <ticket-name> <subtask-name>")
	}
	name := args[0]
	subtask := args[1]

	// Check git hygiene
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusOutput, _ := statusCmd.Output()
	if len(strings.TrimSpace(string(statusOutput))) > 0 {
		logFatal("Git status is not clean. Please commit or stash changes before running 'subtask failed'.")
	}

	ticket, _ := ParseTicketMd(filepath.Join("src", "tickets", name, "ticket.md"))
	for i := range ticket.Subtasks {
		if ticket.Subtasks[i].Name == subtask {
			ticket.Subtasks[i].Status = "failed"
			ticket.Subtasks[i].FailTimestamp = time.Now().Format(time.RFC3339)
		}
	}
	ticketPath := filepath.Join("src", "tickets", name, "ticket.md")
	WriteTicketMd(ticketPath, ticket)

	addCmd := exec.Command("git", "add", ticketPath)
	if addOutput, err := addCmd.CombinedOutput(); err != nil {
		logFatal("Git add failed: %v\nOutput: %s", err, string(addOutput))
	}
	commitCmd := exec.Command("git", "commit", "-m", fmt.Sprintf("docs: subtask %s failed", subtask))
	if commitOutput, err := commitCmd.CombinedOutput(); err != nil {
		logFatal("Git commit failed: %v\nOutput: %s", err, string(commitOutput))
	}
}

func logSubtaskCommand(command string, args []string) {
	ticketID := ""
	if len(args) > 0 {
		ticketID = args[0]
	} else if ticket, err := GetCurrentTicket(); err == nil {
		ticketID = ticket.ID
	}
	if ticketID == "" {
		return
	}

	subArgs := args
	if command != "" {
		subArgs = append([]string{command}, args...)
	}
	logTicketCommand(ticketID, "subtask", subArgs)
}

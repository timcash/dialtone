package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

func RunNext(args []string) {
	name := GetCurrentBranch()
	if name == "" {
		logFatal("Must be on a feature branch to run 'next'")
	}

	path := filepath.Join("src", "tickets_v2", name, "ticket.md")
	ticket, err := ParseTicketMd(path)
	if err != nil {
		logFatal("Failed to parse ticket: %v", err)
	}

	st := FindNextSubtask(ticket)
	if st == nil {
		logInfo("No eligible subtasks found. Ticket might be complete.")
		PrintTicketReport(ticket)
		return
	}

	if st.Status == "todo" {
		logInfo("Promoting subtask %s to progress", st.Name)
		st.Status = "progress"
		WriteTicketMd(path, ticket)
	}

	logInfo("Executing test for subtask: %s", st.Name)
	err = runDynamicTest(ticket.ID, st.Name)
	if err == nil {
		logInfo("Subtask %s passed!", st.Name)
		st.Status = "done"
		st.PassTimestamp = time.Now().Format(time.RFC3339)
		WriteTicketMd(path, ticket)
		
		// Auto-commit the progress
		exec.Command("git", "add", ".").Run()
		exec.Command("git", "commit", "-m", fmt.Sprintf("feat: subtask %s passed", st.Name)).Run()
		
		// Recurse to 'next' if possible, or show report
		RunNext(args)
	} else {
		logInfo("Subtask %s failed.", st.Name)
		st.FailTimestamp = time.Now().Format(time.RFC3339)
		st.AgentNotes = err.Error()
		WriteTicketMd(path, ticket)
		PrintTicketReport(ticket)
	}
}

func RunSubtask(args []string) {
	if len(args) == 0 {
		name := GetCurrentBranch()
		if name == "" {
			logFatal("Usage: ticket_v2 subtask <subcmd> [options]")
		}
		path := filepath.Join("src", "tickets_v2", name, "ticket.md")
		ticket, _ := ParseTicketMd(path)
		st := FindNextSubtask(ticket)
		if st != nil {
			fmt.Printf("Next Subtask: %s\n", st.Name)
		}
		return
	}

	subcmd := args[0]
	subArgs := args[1:]

	switch subcmd {
	case "list":
		RunSubtaskList(subArgs)
	case "test":
		RunSubtaskTest(subArgs)
	case "done":
		RunSubtaskDone(subArgs)
	case "failed":
		RunSubtaskFailed(subArgs)
	default:
		fmt.Printf("Unknown subtask command: %s\n", subcmd)
	}
}

func RunSubtaskList(args []string) {
	name := GetCurrentBranch()
	if name == "" {
		logFatal("Must be on a feature branch")
	}
	path := filepath.Join("src", "tickets_v2", name, "ticket.md")
	ticket, _ := ParseTicketMd(path)
	PrintTicketReport(ticket)
}

func RunSubtaskTest(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ticket_v2 subtask test <name>")
	}
	subtaskName := args[0]
	ticketName := GetCurrentBranch()
	
	runDynamicTest(ticketName, subtaskName)
}

func RunSubtaskDone(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ticket_v2 subtask done <name>")
	}
	subtaskName := args[0]
	ticketName := GetCurrentBranch()
	
	path := filepath.Join("src", "tickets_v2", ticketName, "ticket.md")
	ticket, _ := ParseTicketMd(path)
	
	for i := range ticket.Subtasks {
		if ticket.Subtasks[i].Name == subtaskName {
			ticket.Subtasks[i].Status = "done"
			ticket.Subtasks[i].PassTimestamp = time.Now().Format(time.RFC3339)
			break
		}
	}
	WriteTicketMd(path, ticket)
	logInfo("Marked %s as done", subtaskName)
}

func RunSubtaskFailed(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ticket_v2 subtask failed <name>")
	}
	subtaskName := args[0]
	ticketName := GetCurrentBranch()
	
	path := filepath.Join("src", "tickets_v2", ticketName, "ticket.md")
	ticket, _ := ParseTicketMd(path)
	
	for i := range ticket.Subtasks {
		if ticket.Subtasks[i].Name == subtaskName {
			ticket.Subtasks[i].Status = "failed"
			ticket.Subtasks[i].FailTimestamp = time.Now().Format(time.RFC3339)
			break
		}
	}
	WriteTicketMd(path, ticket)
	logInfo("Marked %s as failed", subtaskName)
}

func RunNextSubtask(args []string) {
	RunNext(args)
}

func RunTest(args []string) {
	name := GetCurrentBranch()
	if name == "" {
		logFatal("Must be on a feature branch")
	}
	path := filepath.Join("src", "tickets_v2", name, "ticket.md")
	ticket, _ := ParseTicketMd(path)

	logInfo("Testing all subtasks for %s...", name)
	for _, st := range ticket.Subtasks {
		runDynamicTest(name, st.Name)
	}
}

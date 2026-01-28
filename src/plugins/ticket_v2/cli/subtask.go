package cli

import (
	"fmt"
	"path/filepath"
	"time"
)

func RunSubtask(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: ./dialtone.sh ticket_v2 subtask <command> [args]")
		return
	}
	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "list":
		RunSubtaskList(subArgs)
	case "next":
		RunNext(subArgs)
	case "test":
		RunSubtaskTest(subArgs)
	case "done":
		RunSubtaskDone(subArgs)
	case "failed":
		RunSubtaskFailed(subArgs)
	default:
		fmt.Printf("Unknown subtask command: %s\n", subcommand)
	}
}

func RunSubtaskList(args []string) {
	var ticket *Ticket
	var err error
	if len(args) > 0 {
		path := filepath.Join("src", "tickets_v2", args[0], "ticket.md")
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
		path := filepath.Join("src", "tickets_v2", ticketID, "ticket.md")
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

	st := FindNextSubtask(ticket)
	if st == nil {
		logInfo("No eligible subtasks found. Ticket might be complete.")
		PrintTicketReport(ticket)
		return
	}

	if st.Status == "todo" {
		st.Status = "progress"
		logInfo("Promoting subtask %s to progress", st.Name)
		WriteTicketMd(filepath.Join("src", "tickets_v2", ticketID, "ticket.md"), ticket)
	}

	logInfo("Executing test for subtask: %s", st.Name)
	err = runDynamicTest(ticketID, st.Name)
	
	// Reload
	path := filepath.Join("src", "tickets_v2", ticketID, "ticket.md")
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
		WriteTicketMd(filepath.Join("src", "tickets_v2", ticketID, "ticket.md"), ticket)
		
		// Recurse to next task with same ID
		RunNext([]string{ticketID})
	} else {
		logInfo("Subtask %s failed.", st.Name)
		st.FailTimestamp = time.Now().Format(time.RFC3339)
		st.AgentNotes = err.Error()
		WriteTicketMd(filepath.Join("src", "tickets_v2", ticketID, "ticket.md"), ticket)
		PrintTicketReport(ticket)
	}
}

func RunSubtaskTest(args []string) {
	if len(args) < 2 {
		logFatal("Usage: ./dialtone.sh ticket_v2 subtask test <ticket-name> <subtask-name>")
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
		logFatal("Usage: ./dialtone.sh ticket_v2 test <ticket-name>")
	}
	name := args[0]
	logInfo("Testing all subtasks for %s...", name)
	// For simplicity, this demo runs dynamic runner which runs all registered tests
	err := runDynamicTest(name, "")
	if err != nil {
		logFatal("Tests failed: %v", err)
	}
	logInfo("All tests passed!")
}

func RunSubtaskDone(args []string) {
	if len(args) < 2 {
		logFatal("Usage: ./dialtone.sh ticket_v2 subtask done <ticket-name> <subtask-name>")
	}
	name := args[0]
	subtask := args[1]
	ticket, _ := ParseTicketMd(filepath.Join("src", "tickets_v2", name, "ticket.md"))
	for i := range ticket.Subtasks {
		if ticket.Subtasks[i].Name == subtask {
			ticket.Subtasks[i].Status = "done"
			ticket.Subtasks[i].PassTimestamp = time.Now().Format(time.RFC3339)
		}
	}
	WriteTicketMd(filepath.Join("src", "tickets_v2", name, "ticket.md"), ticket)
}

func RunSubtaskFailed(args []string) {
	if len(args) < 2 {
		logFatal("Usage: ./dialtone.sh ticket_v2 subtask failed <ticket-name> <subtask-name>")
	}
	name := args[0]
	subtask := args[1]
	ticket, _ := ParseTicketMd(filepath.Join("src", "tickets_v2", name, "ticket.md"))
	for i := range ticket.Subtasks {
		if ticket.Subtasks[i].Name == subtask {
			ticket.Subtasks[i].Status = "failed"
			ticket.Subtasks[i].FailTimestamp = time.Now().Format(time.RFC3339)
		}
	}
	WriteTicketMd(filepath.Join("src", "tickets_v2", name, "ticket.md"), ticket)
}

package cli

import (
	"fmt"
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
		RunSubtaskDone(cmdArgs)
	case "failed":
		RunSubtaskFailed(cmdArgs)
	case "note":
		RunSubtaskNote(cmdArgs)
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
		ticket, err = GetTicket(args[0])
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
		ticket, err = GetTicket(ticketID)
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
		if err := SaveTicket(ticket); err != nil {
			logFatal("Could not update ticket %s: %v", ticketID, err)
		}
	}

	logInfo("Executing test for subtask: %s", st.Name)
	testErr := runDynamicTest(ticketID, st.Name)

	// Reload
	ticket, err = GetTicket(ticketID)
	if err != nil {
		logFatal("Error: %v", err)
	}
	st = nil
	for i := range ticket.Subtasks {
		if ticket.Subtasks[i].Status == "progress" {
			st = &ticket.Subtasks[i]
		}
	}

	if testErr == nil {
		logInfo("Subtask %s passed!", st.Name)
		st.Status = "done"
		st.PassTimestamp = time.Now().Format(time.RFC3339)
		if err := SaveTicket(ticket); err != nil {
			logFatal("Could not update ticket %s: %v", ticketID, err)
		}

		// Recurse to next task with same ID
		RunNext([]string{ticketID})
	} else {
		logInfo("Subtask %s failed.", st.Name)
		st.FailTimestamp = time.Now().Format(time.RFC3339)
		st.AgentNotes = testErr.Error()
		if err := SaveTicket(ticket); err != nil {
			logFatal("Could not update ticket %s: %v", ticketID, err)
		}
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

	logSubtaskCommand("done", args)

	ticket, err := GetTicket(name)
	if err != nil {
		logFatal("Error: %v", err)
	}
	for i := range ticket.Subtasks {
		if ticket.Subtasks[i].Name == subtask {
			ticket.Subtasks[i].Status = "done"
			ticket.Subtasks[i].PassTimestamp = time.Now().Format(time.RFC3339)
		}
	}
	if err := SaveTicket(ticket); err != nil {
		logFatal("Could not update ticket %s: %v", name, err)
	}
}

func RunSubtaskFailed(args []string) {
	if len(args) < 2 {
		logFatal("Usage: ./dialtone.sh ticket subtask failed <ticket-name> <subtask-name>")
	}
	name := args[0]
	subtask := args[1]

	logSubtaskCommand("failed", args)

	ticket, err := GetTicket(name)
	if err != nil {
		logFatal("Error: %v", err)
	}
	for i := range ticket.Subtasks {
		if ticket.Subtasks[i].Name == subtask {
			ticket.Subtasks[i].Status = "failed"
			ticket.Subtasks[i].FailTimestamp = time.Now().Format(time.RFC3339)
		}
	}
	if err := SaveTicket(ticket); err != nil {
		logFatal("Could not update ticket %s: %v", name, err)
	}
}

func RunSubtaskNote(args []string) {
	if len(args) < 3 {
		logFatal("Usage: ./dialtone.sh ticket subtask note <ticket-name> <subtask-name> <note>")
	}
	name := args[0]
	subtask := args[1]
	note := strings.Join(args[2:], " ")

	logSubtaskCommand("note", args)

	ticket, err := GetTicket(name)
	if err != nil {
		logFatal("Error: %v", err)
	}
	found := false
	for i := range ticket.Subtasks {
		if ticket.Subtasks[i].Name == subtask {
			ticket.Subtasks[i].AgentNotes = note
			found = true
		}
	}
	if !found {
		logFatal("Subtask not found: %s", subtask)
	}
	if err := SaveTicket(ticket); err != nil {
		logFatal("Could not update ticket %s: %v", name, err)
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

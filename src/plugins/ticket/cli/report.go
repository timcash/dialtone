package cli

import (
	"fmt"
	"os"
	"strings"
)

// PrintTicketReport prints a standardized report of the ticket's subtasks.
func PrintTicketReport(ticket *Ticket) {
	fmt.Printf("\nSubtasks for %s:\n", ticket.ID)
	fmt.Println("---------------------------------------------------")
	for _, st := range ticket.Subtasks {
		status := st.Status
		if len(status) > 4 {
			status = status[:4]
		}
		if status == "" {
			status = "todo"
		}
		fmt.Printf("[%s]      %s\n", status, st.Name)
	}
	fmt.Println("---------------------------------------------------")

	nextSt := FindNextSubtask(ticket)
	if nextSt != nil {
		fmt.Println("Next Subtask:")
		fmt.Printf("Name:            %s\n", nextSt.Name)
		fmt.Printf("Tags:            %s\n", strings.Join(nextSt.Tags, ", "))
		fmt.Printf("Dependencies:    %s\n", strings.Join(nextSt.Dependencies, ", "))
		fmt.Printf("Description:     %s\n", nextSt.Description)
		for i, cond := range nextSt.TestConditions {
			fmt.Printf("Test-Condition-%d: %s\n", i+1, cond.Condition)
		}
		fmt.Printf("Agent-Notes:     %s\n", nextSt.AgentNotes)
		fmt.Printf("Pass-Timestamp:  %s\n", nextSt.PassTimestamp)
		fmt.Printf("Fail-Timestamp:  %s\n", nextSt.FailTimestamp)
		fmt.Printf("Status:          %s\n", nextSt.Status)
	}
}

// FindNextSubtask finds the first incomplete subtask in the ticket.
func FindNextSubtask(ticket *Ticket) *Subtask {
	// First look for one in progress
	for i := range ticket.Subtasks {
		if ticket.Subtasks[i].Status == "progress" {
			return &ticket.Subtasks[i]
		}
	}
	// Then look for the first todo whose dependencies are met
	for i := range ticket.Subtasks {
		if ticket.Subtasks[i].Status == "todo" || ticket.Subtasks[i].Status == "" {
			if dependenciesMet(ticket, &ticket.Subtasks[i]) {
				return &ticket.Subtasks[i]
			}
		}
	}
	return nil
}

func dependenciesMet(ticket *Ticket, st *Subtask) bool {
	if len(st.Dependencies) == 0 {
		return true
	}
	doneTasks := make(map[string]bool)
	for _, s := range ticket.Subtasks {
		if s.Status == "done" || s.Status == "skipped" {
			doneTasks[s.Name] = true
		}
	}
	for _, d := range st.Dependencies {
		if !doneTasks[strings.TrimSpace(d)] {
			return false
		}
	}
	return true
}

// Helpers for consistent logging
func logInfo(format string, a ...interface{}) {
	fmt.Printf("[ticket] "+format+"\n", a...)
}

func logFatal(format string, a ...interface{}) {
	fmt.Printf("[ticket] FATAL: "+format+"\n", a...)
	os.Exit(1)
}

package cli

import (
	"fmt"
	"strings"
)

func printReviewIteration(ticket *Ticket) {
	if ticket == nil {
		return
	}

	fmt.Println()
	fmt.Printf("[REVIEW] Ticket: %s\n", ticket.ID)
	if strings.TrimSpace(ticket.Description) != "" {
		fmt.Printf("- goal: %s\n", strings.TrimSpace(ticket.Description))
	}
	fmt.Println()
	fmt.Println("[REVIEW] Questions (ticket):")
	fmt.Println("1. is the goal aligned with subtasks")
	fmt.Println("2. should there be more subtasks")
	fmt.Println("3. are any subtasks too large")
	fmt.Println("4. is there work that should be put into a different ticket because it is not relevant")
	fmt.Println("5. does this ticket create a new plugin")
	fmt.Println("6. does this ticket have a update documentation subtask")
	fmt.Println()

	for _, st := range ticket.Subtasks {
		name := strings.TrimSpace(st.Name)
		if name == "" {
			continue
		}
		fmt.Println("---------------------------------------------------")
		fmt.Printf("[REVIEW] Subtask: %s\n", name)
		if strings.TrimSpace(st.Description) != "" {
			fmt.Printf("- description: %s\n", strings.TrimSpace(st.Description))
		}
		if len(st.Dependencies) > 0 {
			fmt.Printf("- deps: %s\n", strings.Join(st.Dependencies, ", "))
		}
		if strings.TrimSpace(st.TestCommand) != "" {
			fmt.Printf("- test-command: %s\n", strings.TrimSpace(st.TestCommand))
		} else {
			fmt.Printf("- test-command: (missing)\n")
		}
		if strings.TrimSpace(st.ReviewedTimestamp) != "" {
			fmt.Printf("- reviewed-timestamp: %s\n", strings.TrimSpace(st.ReviewedTimestamp))
		}
		fmt.Println()
		fmt.Println("[REVIEW] Questions (subtask):")
		fmt.Println("1. is this subtask aligned with the ticket goal")
		fmt.Println("2. is this subtask too large (should it be split)")
		fmt.Println("3. should any work move to a different ticket")
		fmt.Println("4. does this subtask include/require documentation updates")
		fmt.Println("5. does this subtask have the correct test-command")
		fmt.Println()
	}
}


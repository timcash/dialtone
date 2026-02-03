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
	fmt.Println("[REVIEW] Field checks (ticket):")
	fmt.Printf("- id: %s -> is this correct?\n", valueOrEmpty(ticket.ID))
	fmt.Printf("- name: %s -> is this correct?\n", valueOrEmpty(ticket.Name))
	fmt.Printf("- tags: %s -> is this correct?\n", valueOrEmpty(strings.Join(ticket.Tags, ", ")))
	fmt.Printf("- description: %s -> is this correct?\n", valueOrEmpty(ticket.Description))
	fmt.Printf("- state: %s -> is this correct?\n", valueOrEmpty(ticket.State))
	fmt.Printf("- start_time: %s -> is this correct?\n", valueOrEmpty(ticket.StartTime))
	fmt.Printf("- last_summary_time: %s -> is this correct?\n", valueOrEmpty(ticket.LastSummaryTime))
	fmt.Printf("- agent_summary: %s -> is this correct?\n", summarizeText(ticket.AgentSummary))
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
		fmt.Println("[REVIEW] Field checks (subtask):")
		fmt.Printf("- name: %s -> is this correct?\n", valueOrEmpty(st.Name))
		fmt.Printf("- tags: %s -> is this correct?\n", valueOrEmpty(strings.Join(st.Tags, ", ")))
		fmt.Printf("- dependencies: %s -> is this correct?\n", valueOrEmpty(strings.Join(st.Dependencies, ", ")))
		fmt.Printf("- description: %s -> is this correct?\n", valueOrEmpty(st.Description))
		fmt.Printf("- test-conditions: %s -> is this correct?\n", valueOrEmpty(formatTestConditions(st.TestConditions)))
		fmt.Printf("- test-command: %s -> is this correct?\n", valueOrEmpty(st.TestCommand))
		fmt.Printf("- agent-notes: %s -> is this correct?\n", valueOrEmpty(st.AgentNotes))
		fmt.Printf("- reviewed-timestamp: %s -> is this correct?\n", valueOrEmpty(st.ReviewedTimestamp))
		fmt.Printf("- pass-timestamp: %s -> is this correct?\n", valueOrEmpty(st.PassTimestamp))
		fmt.Printf("- fail-timestamp: %s -> is this correct?\n", valueOrEmpty(st.FailTimestamp))
		fmt.Printf("- status: %s -> is this correct?\n", valueOrEmpty(st.Status))
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

func valueOrEmpty(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "(empty)"
	}
	return trimmed
}

func summarizeText(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "(empty)"
	}
	if len(trimmed) > 120 {
		return trimmed[:117] + "..."
	}
	return trimmed
}

func formatTestConditions(conditions []TestCondition) string {
	if len(conditions) == 0 {
		return ""
	}
	parts := make([]string, 0, len(conditions))
	for _, c := range conditions {
		cond := strings.TrimSpace(c.Condition)
		if cond == "" {
			continue
		}
		parts = append(parts, cond)
	}
	return strings.Join(parts, "; ")
}


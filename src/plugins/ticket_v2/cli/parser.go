package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ParseTicketMd parses a ticket.md file into a Ticket struct.
func ParseTicketMd(path string) (*Ticket, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	ticket := &Ticket{}
	var currentSubtask *Subtask
	scanner := bufio.NewScanner(file)

	inGoal := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(line, "# Name:") {
			ticket.ID = strings.TrimSpace(strings.TrimPrefix(line, "# Name:"))
			ticket.Name = ticket.ID
			continue
		}

		if strings.HasPrefix(line, "# Tags:") {
			tagsStr := strings.TrimSpace(strings.TrimPrefix(line, "# Tags:"))
			if tagsStr != "" {
				parts := strings.Split(tagsStr, ",")
				for _, p := range parts {
					ticket.Tags = append(ticket.Tags, strings.TrimSpace(p))
				}
			}
			continue
		}

		if strings.HasPrefix(line, "# Goal") {
			inGoal = true
			continue
		}

		if inGoal && !strings.HasPrefix(line, "## SUBTASK:") && trimmedLine != "" && !strings.HasPrefix(line, "#") {
			if ticket.Description == "" {
				ticket.Description = trimmedLine
			} else {
				ticket.Description += "\n" + trimmedLine
			}
			continue
		}

		if strings.HasPrefix(line, "## SUBTASK:") {
			inGoal = false
			if currentSubtask != nil {
				ticket.Subtasks = append(ticket.Subtasks, *currentSubtask)
			}
			currentSubtask = &Subtask{}
			continue
		}

		if currentSubtask != nil {
			if strings.HasPrefix(trimmedLine, "- name:") {
				currentSubtask.Name = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "- name:"))
			} else if strings.HasPrefix(trimmedLine, "- tags:") {
				tagsStr := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "- tags:"))
				if tagsStr != "" {
					parts := strings.Split(tagsStr, ",")
					for _, p := range parts {
						currentSubtask.Tags = append(currentSubtask.Tags, strings.TrimSpace(p))
					}
				}
			} else if strings.HasPrefix(trimmedLine, "- dependencies:") {
				depsStr := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "- dependencies:"))
				if depsStr != "" {
					parts := strings.Split(depsStr, ",")
					for _, p := range parts {
						currentSubtask.Dependencies = append(currentSubtask.Dependencies, strings.TrimSpace(p))
					}
				}
			} else if strings.HasPrefix(trimmedLine, "- description:") {
				currentSubtask.Description = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "- description:"))
			} else if strings.HasPrefix(trimmedLine, "- test-condition-") {
				cond := strings.TrimSpace(strings.SplitN(trimmedLine, ":", 2)[1])
				currentSubtask.TestConditions = append(currentSubtask.TestConditions, TestCondition{Condition: cond})
			} else if strings.HasPrefix(trimmedLine, "- agent-notes:") {
				currentSubtask.AgentNotes = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "- agent-notes:"))
			} else if strings.HasPrefix(trimmedLine, "- pass-timestamp:") {
				currentSubtask.PassTimestamp = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "- pass-timestamp:"))
			} else if strings.HasPrefix(trimmedLine, "- fail-timestamp:") {
				currentSubtask.FailTimestamp = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "- fail-timestamp:"))
			} else if strings.HasPrefix(trimmedLine, "- status:") {
				currentSubtask.Status = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "- status:"))
			}
		}
	}

	if currentSubtask != nil {
		ticket.Subtasks = append(ticket.Subtasks, *currentSubtask)
	}

	// Validation
	if ticket.ID == "" {
		return nil, fmt.Errorf("ticket is missing '# Name:' header")
	}

	validStatuses := map[string]bool{
		"todo":     true,
		"progress": true,
		"done":     true,
		"failed":   true,
		"skipped":  true,
		"":         true, // Allow empty for initial scaffold
	}

	for _, st := range ticket.Subtasks {
		if st.Name == "" {
			return nil, fmt.Errorf("subtask is missing '- name:' field")
		}
		if !validStatuses[st.Status] {
			return nil, fmt.Errorf("subtask %s has invalid status: %s", st.Name, st.Status)
		}
	}

	return ticket, scanner.Err()
}

// WriteTicketMd writes a Ticket struct back to a ticket.md file.
func WriteTicketMd(path string, ticket *Ticket) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	fmt.Fprintf(writer, "# Name: %s\n", ticket.ID)
	if len(ticket.Tags) > 0 {
		fmt.Fprintf(writer, "# Tags: %s\n", strings.Join(ticket.Tags, ", "))
	}
	fmt.Fprintln(writer)

	fmt.Fprintln(writer, "# Goal")
	fmt.Fprintln(writer, ticket.Description)
	fmt.Fprintln(writer)

	for _, st := range ticket.Subtasks {
		fmt.Fprintf(writer, "## SUBTASK: %s\n", strings.Title(strings.ReplaceAll(st.Name, "-", " ")))
		fmt.Fprintf(writer, "- name: %s\n", st.Name)
		if len(st.Tags) > 0 {
			fmt.Fprintf(writer, "- tags: %s\n", strings.Join(st.Tags, ", "))
		}
		if len(st.Dependencies) > 0 {
			fmt.Fprintf(writer, "- dependencies: %s\n", strings.Join(st.Dependencies, ", "))
		}
		fmt.Fprintf(writer, "- description: %s\n", st.Description)
		for i, cond := range st.TestConditions {
			fmt.Fprintf(writer, "- test-condition-%d: %s\n", i+1, cond.Condition)
		}
		fmt.Fprintf(writer, "- agent-notes: %s\n", st.AgentNotes)
		fmt.Fprintf(writer, "- pass-timestamp: %s\n", st.PassTimestamp)
		fmt.Fprintf(writer, "- fail-timestamp: %s\n", st.FailTimestamp)
		fmt.Fprintf(writer, "- status: %s\n", st.Status)
		fmt.Fprintln(writer)
	}

	return writer.Flush()
}

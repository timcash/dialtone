package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func subtaskSummaryFilename(subtaskName string) string {
	// Subtask names are expected to be lowercase + dashes, but keep it safe.
	safe := strings.TrimSpace(subtaskName)
	safe = strings.ReplaceAll(safe, "/", "-")
	safe = strings.ReplaceAll(safe, "\\", "-")
	return fmt.Sprintf("%s-summary.md", safe)
}

func subtaskSummaryPath(ticketID, subtaskName string) string {
	return filepath.Join(ticketDir(ticketID), subtaskSummaryFilename(subtaskName))
}

func ensureSubtaskSummaryFile(ticketID, subtaskName string) (string, error) {
	if err := os.MkdirAll(ticketDir(ticketID), 0755); err != nil {
		return "", err
	}
	path := subtaskSummaryPath(ticketID, subtaskName)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	// Create a helpful starter template (kept intentionally short).
	template := fmt.Sprintf(`# Ticket: %s
## Subtask: %s

## What changed
- TODO

## Commands / verification
- TODO

## Notes
- TODO
`, ticketID, subtaskName)

	if err := os.WriteFile(path, []byte(template), 0644); err != nil {
		return "", err
	}
	return path, nil
}

func ensureAllSubtaskSummaryFiles(ticket *Ticket) error {
	if ticket == nil {
		return fmt.Errorf("ticket is nil")
	}
	for _, st := range ticket.Subtasks {
		if strings.TrimSpace(st.Name) == "" {
			continue
		}
		if _, err := ensureSubtaskSummaryFile(ticket.ID, st.Name); err != nil {
			return err
		}
	}
	return nil
}

func readSubtaskSummary(ticketID, subtaskName string) (string, string, error) {
	path, err := ensureSubtaskSummaryFile(ticketID, subtaskName)
	if err != nil {
		return "", path, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", path, err
	}
	return strings.TrimSpace(string(b)), path, nil
}

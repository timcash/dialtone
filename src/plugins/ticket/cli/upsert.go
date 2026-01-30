package cli

import (
	"encoding/json"
	"flag"
	"io"
	"os"
)

func RunUpsert(args []string) {
	fs := flag.NewFlagSet("ticket upsert", flag.ExitOnError)
	filePath := fs.String("file", "", "Path to ticket JSON (defaults to stdin)")
	setCurrent := fs.Bool("set-current", true, "Set ticket as current after upsert")
	fs.Parse(args)

	var payload []byte
	var err error
	if *filePath != "" {
		payload, err = os.ReadFile(*filePath)
	} else {
		payload, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		logFatal("Could not read ticket JSON: %v", err)
	}
	if len(payload) == 0 {
		logFatal("Ticket JSON is empty")
	}

	var ticket Ticket
	if err := json.Unmarshal(payload, &ticket); err != nil {
		logFatal("Could not parse ticket JSON: %v", err)
	}
	if ticket.ID == "" {
		logFatal("Ticket id is required")
	}
	if ticket.Name == "" {
		ticket.Name = ticket.ID
	}
	for _, st := range ticket.Subtasks {
		if st.Status != "" || st.PassTimestamp != "" || st.FailTimestamp != "" {
			logInfo("Ignoring subtask status fields from upsert payload. Use ticket CLI commands instead.")
			break
		}
	}
	for i := range ticket.Subtasks {
		ticket.Subtasks[i].Status = ""
		ticket.Subtasks[i].PassTimestamp = ""
		ticket.Subtasks[i].FailTimestamp = ""
	}

	if err := SaveTicket(&ticket); err != nil {
		logFatal("Could not save ticket %s: %v", ticket.ID, err)
	}
	if *setCurrent {
		if err := SetCurrentTicket(ticket.ID); err != nil {
			logFatal("Could not set current ticket %s: %v", ticket.ID, err)
		}
	}
	logTicketCommand(ticket.ID, "upsert", args)
	logInfo("Upserted ticket %s", ticket.ID)
}

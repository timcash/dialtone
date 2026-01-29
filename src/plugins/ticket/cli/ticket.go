package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Run(args []string) {
	if len(args) == 0 {
		printUsage()
		return
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "add":
		RunAdd(subArgs)
	case "start":
		RunStart(subArgs)
	case "ask":
		RunAsk(subArgs)
	case "log":
		RunLog(subArgs)
	case "list":
		RunList(subArgs)
	case "validate":
		RunValidate(subArgs)
	case "next":
		RunNext(subArgs)
	case "done":
		RunDone(subArgs)
	case "upsert":
		RunUpsert(subArgs)
	case "subtask":
		RunSubtask(subArgs)
	case "test":
		RunTest(subArgs)
	default:
		fmt.Printf("Unknown ticket subcommand: %s\n", subcommand)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh ticket <command> [args]")
	fmt.Println("Commands: add, start, ask, log, list, validate, next, done, upsert, subtask, test")
}

func RunAdd(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ./dialtone.sh ticket add <ticket-name>")
	}
	name := args[0]
	dir := filepath.Join("src", "tickets", name)
	os.MkdirAll(filepath.Join(dir, "test"), 0755)

	if _, err := GetTicket(name); err != nil {
		if !errors.Is(err, ErrTicketNotFound) {
			logFatal("Could not load ticket %s: %v", name, err)
		}
		ticket := &Ticket{
			ID:          name,
			Name:        name,
			Description: "",
			Subtasks: []Subtask{
				{
					Name:        "init",
					Description: "Initialization",
					Status:      "todo",
				},
			},
		}
		if err := SaveTicket(ticket); err != nil {
			logFatal("Could not create ticket %s: %v", name, err)
		}
		logInfo("Created %s", ticketDBPath())
	}
	if err := SetCurrentTicket(name); err != nil {
		logFatal("Could not set current ticket %s: %v", name, err)
	}

	testGo := filepath.Join(dir, "test", "test.go")
	if _, err := os.Stat(testGo); os.IsNotExist(err) {
		content := fmt.Sprintf(`package test
import (
	"dialtone/cli/src/dialtest"
)
func init() {
	dialtest.RegisterTicket("%s")
	dialtest.AddSubtaskTest("example", RunExample, nil)
}
func RunExample() error {
	return nil
}
`, name)
		os.WriteFile(testGo, []byte(content), 0644)
		logInfo("Created %s", testGo)
	}

	logTicketCommand(name, "add", args)
}

func RunStart(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ./dialtone.sh ticket start <ticket-name>")
	}
	name := args[0]
	RunAdd(args)
	logTicketCommand(name, "start", args)
	logInfo("Ticket %s started successfully", name)
}

func RunAsk(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ./dialtone.sh ticket ask [--subtask <subtask-name>] <question>")
	}

	subtask := ""
	if strings.HasPrefix(args[0], "--subtask=") {
		subtask = strings.TrimPrefix(args[0], "--subtask=")
		args = args[1:]
	} else if len(args) >= 2 && args[0] == "--subtask" {
		subtask = args[1]
		args = args[2:]
	}

	if len(args) < 1 {
		logFatal("Usage: ./dialtone.sh ticket ask [--subtask <subtask-name>] <question>")
	}

	question := strings.Join(args, " ")
	ticket, err := GetCurrentTicket()
	if err != nil {
		logFatal("Error getting current ticket: %v", err)
	}

	appendTicketLogEntry(ticket.ID, "question", question, subtask)
}

func RunLog(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ./dialtone.sh ticket log <message>")
	}

	message := strings.Join(args, " ")
	ticket, err := GetCurrentTicket()
	if err != nil {
		logFatal("Error getting current ticket: %v", err)
	}

	appendTicketLogEntry(ticket.ID, "log", message, "")
}

func appendTicketLogEntry(ticketID, entryType, message, subtask string) {
	if err := AppendTicketLogEntry(ticketID, entryType, message, subtask); err != nil {
		logFatal("Could not write log for %s: %v", ticketID, err)
	}
	logInfo("Captured %s in %s", entryType, ticketDBPath())
}

func logTicketCommand(ticketID, command string, args []string) {
	if ticketID == "" || command == "" {
		return
	}

	message := fmt.Sprintf("ticket %s %s", command, strings.Join(args, " "))
	message = strings.TrimSpace(message)
	appendTicketLogEntry(ticketID, "command", message, "")
}

func RunList(args []string) {
	fmt.Println("Tickets (v2):")
	ids, err := ListTickets()
	if err != nil {
		logFatal("Could not list tickets: %v", err)
	}
	for _, id := range ids {
		fmt.Printf("- %s\n", id)
	}

	if len(args) > 0 {
		logTicketCommand(args[0], "list", args)
	} else if ticket, err := GetCurrentTicket(); err == nil {
		logTicketCommand(ticket.ID, "list", args)
	}
}

func RunValidate(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ./dialtone.sh ticket validate <ticket-name>")
	}
	name := args[0]
	logTicketCommand(name, "validate", args)
	_, err := GetTicket(name)
	if err != nil {
		logFatal("Validation failed: %v", err)
	}
	logInfo("Ticket %s is valid", name)
}

func RunDone(args []string) {
	// Simple validation: all subtasks must be done
	ticket, err := GetCurrentTicket()
	if err != nil {
		logFatal("Error getting current ticket: %v", err)
	}
	for _, st := range ticket.Subtasks {
		if st.Status != "done" && st.Status != "failed" && st.Status != "skipped" {
			logFatal("Subtask %s is still %s", st.Name, st.Status)
		}
	}

	logInfo("Finalizing ticket %s...", ticket.ID)
	logTicketCommand(ticket.ID, "done", args)
	logInfo("Ticket %s completed", ticket.ID)
}

func GetCurrentTicket() (*Ticket, error) {
	name, err := GetCurrentTicketID()
	if err != nil {
		return nil, err
	}
	ticket, err := GetTicket(name)
	if err != nil {
		if errors.Is(err, ErrTicketNotFound) {
			return nil, fmt.Errorf("no ticket found for current ticket %s", name)
		}
		return nil, err
	}
	return ticket, nil
}

package cli

import (
	"fmt"
	"strings"
	"time"
)

func RunSubtask(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: ./dialtone.sh ticket subtask <command> [args]")
		fmt.Println("Commands: add, list, status, done, failed, note, test, testcmd")
		return
	}

	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "add":
		RunSubtaskAdd(cmdArgs)
	case "list":
		logSubtaskCommand(command, cmdArgs)
		RunSubtaskList(cmdArgs)
	case "status":
		RunSubtaskStatus(cmdArgs)
	case "test":
		logSubtaskCommand(command, cmdArgs)
		RunSubtaskTestCmd(cmdArgs)
	case "testcmd":
		RunSubtaskTestCmdSet(cmdArgs)
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

func RunSubtaskTestCmdSet(args []string) {
	if len(args) < 2 {
		logFatal("Usage: ./dialtone.sh ticket subtask testcmd <subtask-name> <test-command...>")
	}

	subtaskName := args[0]
	testCommand := strings.TrimSpace(strings.Join(args[1:], " "))
	if testCommand == "" {
		logFatal("Test command cannot be empty")
	}

	ticket, err := GetCurrentTicket()
	if err != nil {
		logFatal("Error getting current ticket: %v", err)
	}

	found := false
	for i := range ticket.Subtasks {
		if ticket.Subtasks[i].Name == subtaskName {
			ticket.Subtasks[i].TestCommand = testCommand
			found = true
			break
		}
	}
	if !found {
		logFatal("Subtask not found: %s", subtaskName)
	}

	if err := SaveTicket(ticket); err != nil {
		logFatal("Could not save ticket: %v", err)
	}

	logSubtaskCommand("testcmd", append([]string{subtaskName}, args[1:]...))
	logInfo("Set test command for subtask %s", subtaskName)
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

	// V2: Check for unacknowledged questions
	entries, err := GetLogEntries(ticketID)
	if err == nil {
		lastQuestion := ""
		lastAckTime := ""
		for _, e := range entries {
			if e.EntryType == "question" {
				lastQuestion = e.Message
			}
			if e.EntryType == "ack" {
				lastAckTime = e.Timestamp
			}
		}
		if lastQuestion != "" && lastAckTime == "" {
			fmt.Printf("[BLOCK] Cannot proceed to 'next' subtask.\n")
			fmt.Printf("[MESSAGE] A question is pending response or acknowledgement: \"%s\"\n", lastQuestion)
			fmt.Printf("[ACTION] Please acknowledge to continue: ./dialtone.sh ticket ack\n")
			return
		}
	}

	// V2: Check for summary timeout (10 minutes)
	idle := false
	for _, arg := range args {
		if arg == "--idle" {
			idle = true
		}
	}

	if ticket.LastSummaryTime != "" && !idle {
		lastTime, err := time.Parse(time.RFC3339, ticket.LastSummaryTime)
		if err == nil {
			if time.Since(lastTime) > 10*time.Minute {
				fmt.Printf("[BLOCK] 10-minute activity window exceeded.\n")
				fmt.Printf("[MESSAGE] Please provide a summary of work or use --idle.\n")
				fmt.Printf("[EXAMPLE] Recommended format for agent_summary.md:\n\n")
				fmt.Printf("## Ran commands to find source files\n")
				fmt.Printf("1. searched with grep - result nothing\n")
				fmt.Printf("2. searched with `./dialtone.sh ticket search \"vertex node\"` - 2 results\n\n")
				fmt.Printf("[ACTION] Update agent_summary.md and run: ./dialtone.sh ticket summary update\n")
				return
			}
		}
	}

	// V2: Design Doc Alert (Scaffold)
	// In a real scenario, we'd check file mod times.
	if strings.Contains(ticketID, "api") {
		fmt.Println("[ALERT] Design Doc 'api_spec.md' was recently modified.")
		fmt.Println("[DIFF] Field 'webhook_url' renamed to 'callback_uri'.")
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
		if err := SaveTicket(ticket); err != nil {
			logFatal("Could not update ticket %s: %v", ticketID, err)
		}
	}

	logInfo("Executing test for subtask: %s", st.Name)
	printDialtone(
		[]string{
			fmt.Sprintf("ticket: %s", ticketID),
			fmt.Sprintf("subtask: %s", st.Name),
			"policy: DIALTONE does not auto-run tests; agent must run and report results",
			"verify: tests pass; logs contain no ERROR/EXCEPTION; tests clean up resources",
		},
		"Run the subtask test command(s) now.\nIf it fails, modify code/tests and re-run until it passes. Then review logs and submit a summary.",
		[]string{
			"./dialtone.sh ticket subtask list",
			"./dialtone.sh plugin test <plugin-name>",
			"./dialtone.sh logs --lines 200",
			"./dialtone.sh ticket summary update",
			"./dialtone.sh ticket subtask done <ticket-name> <subtask-name>",
		},
	)

	// Stop here: do not execute tests automatically.
}

func RunSubtaskTestCmd(args []string) {
	if len(args) < 2 {
		logFatal("Usage: ./dialtone.sh ticket subtask test <ticket-name> <subtask-name>")
	}
	ticketName := args[0]
	subtaskName := args[1]
	ticket, err := GetTicket(ticketName)
	if err != nil {
		logFatal("Error: %v", err)
	}
	subtaskIndex := -1
	for i := range ticket.Subtasks {
		if ticket.Subtasks[i].Name == subtaskName {
			subtaskIndex = i
			break
		}
	}
	if subtaskIndex == -1 {
		logFatal("Subtask not found: %s", subtaskName)
	}

	err = runSubtaskCommandTest(ticketName, subtaskName)
	if err != nil {
		ticket.Subtasks[subtaskIndex].Status = "todo"
		ticket.Subtasks[subtaskIndex].FailTimestamp = time.Now().Format(time.RFC3339)
		ticket.Subtasks[subtaskIndex].AgentNotes = err.Error()
		if saveErr := SaveTicket(ticket); saveErr != nil {
			logFatal("Could not update ticket %s: %v", ticketName, saveErr)
		}
		logFatal("Test failed: %v", err)
	}
	logInfo("Test passed!")
}

func RunTest(args []string) {
	name := ""
	startIndex := 0

	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name = args[0]
		startIndex = 1
	} else {
		currentID, err := GetCurrentTicketID()
		if err != nil {
			logFatal("No ticket name provided and %v", err)
		}
		name = currentID
	}

	subtask := ""
	for i := startIndex; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--subtask=") {
			subtask = strings.TrimPrefix(args[i], "--subtask=")
		} else if args[i] == "--subtask" && i+1 < len(args) {
			subtask = args[i+1]
			i++
		}
	}

	logTicketCommand(name, "test", args)
	if subtask != "" {
		logInfo("Testing subtask %s for %s...", subtask, name)
	} else {
		logInfo("Testing all subtasks for %s...", name)
	}

	ticket, err := GetTicket(name)
	if err != nil {
		logFatal("Error: %v", err)
	}

	failures := 0
	if subtask != "" {
		subtaskIndex := -1
		for i := range ticket.Subtasks {
			if ticket.Subtasks[i].Name == subtask {
				subtaskIndex = i
				break
			}
		}
		if subtaskIndex == -1 {
			logFatal("Subtask not found: %s", subtask)
		}
		if err := runSubtaskCommandTest(name, subtask); err != nil {
			failures++
			ticket.Subtasks[subtaskIndex].Status = "todo"
			ticket.Subtasks[subtaskIndex].FailTimestamp = time.Now().Format(time.RFC3339)
			ticket.Subtasks[subtaskIndex].AgentNotes = err.Error()
		}
	} else {
		for i := range ticket.Subtasks {
			stName := ticket.Subtasks[i].Name
			if err := runSubtaskCommandTest(name, stName); err != nil {
				logInfo("Subtask %s failed: %v", stName, err)
				failures++
				ticket.Subtasks[i].Status = "todo"
				ticket.Subtasks[i].FailTimestamp = time.Now().Format(time.RFC3339)
				ticket.Subtasks[i].AgentNotes = err.Error()
			}
		}
	}

	if failures > 0 {
		if saveErr := SaveTicket(ticket); saveErr != nil {
			logFatal("Could not update ticket %s: %v", name, saveErr)
		}
		logFatal("Tests failed: %d subtask(s)", failures)
	}

	logInfo("Tests passed!")
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
	logInfo("Subtask %s marked as done", subtask)

	printDialtone(
		[]string{
			fmt.Sprintf("ticket: %s", name),
			fmt.Sprintf("subtask: %s", subtask),
			"record: subtask status marked done (manual verification assumed)",
			"next: submit agent summary and prepare a git commit",
		},
		"Please confirm:\n- You ran the subtask tests and they passed\n- You reviewed logs and found no ERROR/EXCEPTION\n- Tests cleaned up any resources they created\n\nThen submit a summary and create a commit.",
		[]string{
			"./dialtone.sh ticket summary update",
			"git status -sb",
			"git add .",
			"git commit -m \"Describe the change\"",
			"./dialtone.sh ticket done",
		},
	)
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

func RunSubtaskAdd(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ./dialtone.sh ticket subtask add <name> [--desc \"...\"] [--deps dep1,dep2] [--status todo|progress]")
	}

	name := args[0]
	desc := ""
	deps := []string{}
	status := "todo"

	// Parse optional flags
	for i := 1; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--desc=") {
			desc = strings.TrimPrefix(args[i], "--desc=")
		} else if args[i] == "--desc" && i+1 < len(args) {
			desc = args[i+1]
			i++
		} else if strings.HasPrefix(args[i], "--deps=") {
			deps = strings.Split(strings.TrimPrefix(args[i], "--deps="), ",")
		} else if args[i] == "--deps" && i+1 < len(args) {
			deps = strings.Split(args[i+1], ",")
			i++
		} else if strings.HasPrefix(args[i], "--status=") {
			status = strings.TrimPrefix(args[i], "--status=")
		} else if args[i] == "--status" && i+1 < len(args) {
			status = args[i+1]
			i++
		}
	}

	ticket, err := GetCurrentTicket()
	if err != nil {
		logFatal("Error getting current ticket: %v", err)
	}

	// Check if subtask already exists
	for _, st := range ticket.Subtasks {
		if st.Name == name {
			logFatal("Subtask %s already exists", name)
		}
	}

	// Add new subtask
	ticket.Subtasks = append(ticket.Subtasks, Subtask{
		Name:         name,
		Description:  desc,
		Dependencies: deps,
		Status:       status,
	})

	if err := SaveTicket(ticket); err != nil {
		logFatal("Could not save ticket: %v", err)
	}

	logSubtaskCommand("add", args)
	logInfo("Added subtask %s to ticket %s", name, ticket.ID)
}

func RunSubtaskStatus(args []string) {
	if len(args) < 2 {
		logFatal("Usage: ./dialtone.sh ticket subtask status <name> <status>")
	}

	name := args[0]
	status := args[1]

	validStatuses := map[string]bool{
		"todo":     true,
		"progress": true,
		"done":     true,
		"failed":   true,
		"skipped":  true,
	}
	if !validStatuses[status] {
		logFatal("Invalid status: %s (must be todo, progress, done, failed, or skipped)", status)
	}

	ticket, err := GetCurrentTicket()
	if err != nil {
		logFatal("Error getting current ticket: %v", err)
	}

	found := false
	for i := range ticket.Subtasks {
		if ticket.Subtasks[i].Name == name {
			ticket.Subtasks[i].Status = status
			if status == "done" {
				ticket.Subtasks[i].PassTimestamp = time.Now().Format(time.RFC3339)
			} else if status == "failed" {
				ticket.Subtasks[i].FailTimestamp = time.Now().Format(time.RFC3339)
			}
			found = true
			break
		}
	}

	if !found {
		logFatal("Subtask not found: %s", name)
	}

	if err := SaveTicket(ticket); err != nil {
		logFatal("Could not save ticket: %v", err)
	}

	logSubtaskCommand("status", args)
	logInfo("Set subtask %s status to %s", name, status)
}

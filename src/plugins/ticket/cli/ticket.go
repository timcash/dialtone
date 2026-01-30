package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

const testTemplate = `package test
import (
	"dialtone/cli/src/dialtest"
)
func init() {
	dialtest.RegisterTicket("{{.TicketID}}")
	dialtest.AddSubtaskTest("init", RunInitTest, nil)
}
func RunInitTest() error {
	// TODO: Implement verification logic for subtask 'init'
	return nil
}
`

func Run(args []string) {
	if len(args) == 0 {
		printUsage()
		return
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "--help", "-h", "help":
		printUsage()
		return
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
	case "summary":
		RunSummary(subArgs)
	case "search":
		RunSearch(subArgs)
	case "ack":
		RunAck(subArgs)
	case "grant":
		RunGrant(subArgs)
	case "upsert":
		RunUpsert(subArgs)
	case "subtask":
		RunSubtask(subArgs)
	case "test":
		RunTest(subArgs)
	case "install":
		// Ticket plugin doesn't have specific dependencies to install via Go yet
		logInfo("Ticket plugin: No specific dependencies to install.")
	case "key":
		RunKey(subArgs)
	case "delete":
		RunDelete(subArgs)
	case "load":
		RunLoad(subArgs)
	default:
		fmt.Printf("Unknown ticket subcommand: %s\n", subcommand)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh ticket <command> [args]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  start <name>       Start a new ticket and create branch")
	fmt.Println("  add <name>         Add a new ticket without starting it")
	fmt.Println("  list               List all tickets")
	fmt.Println("  next               Mark current subtask done and move to next")
	fmt.Println("  done               Complete the current ticket")
	fmt.Println("  validate <name>    Validate a ticket's structure")
	fmt.Println("  delete <name>      Delete a ticket")
	fmt.Println()
	fmt.Println("  subtask add <name> [--desc \"...\"] [--deps dep1,dep2]")
	fmt.Println("                     Add a subtask to the current ticket")
	fmt.Println("  subtask list       List subtasks for current ticket")
	fmt.Println("  subtask status <name> <status>")
	fmt.Println("                     Set subtask status (todo, progress, done, failed, skipped)")
	fmt.Println()
	fmt.Println("  ask <question>     Log a question for the current ticket")
	fmt.Println("  log <message>      Log a message for the current ticket")
	fmt.Println("  summary [update]   Show or update agent summary")
	fmt.Println("  search <query>     Search ticket summaries")
	fmt.Println()
	fmt.Println("  upsert             Import ticket from JSON (stdin or --file)")
	fmt.Println("  load               Load all duckdb backups into ticket system")
	fmt.Println("  test <name>        Run tests for a ticket")
	fmt.Println()
	fmt.Println("  key add <name> <value> <password>")
	fmt.Println("                     Store an encrypted key")
	fmt.Println("  key list           List stored keys")
	fmt.Println("  key rm <name>      Remove a key")
	fmt.Println("  key <name> <pass>  Retrieve a key value")
	fmt.Println()
	fmt.Println("  ack [message]      Acknowledge messages")
	fmt.Println("  grant              Grant temporary resource access")
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
		tmpl, err := template.New("test").Parse(testTemplate)
		if err != nil {
			logFatal("Could not parse test template: %v", err)
		}

		f, err := os.Create(testGo)
		if err != nil {
			logFatal("Could not create %s: %v", testGo, err)
		}
		defer f.Close()

		data := struct {
			TicketID string
		}{
			TicketID: name,
		}

		if err := tmpl.Execute(f, data); err != nil {
			logFatal("Could not execute test template: %v", err)
		}
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

	// V2: Contextual Alert (Scaffold)
	if strings.Contains(name, "api") || strings.Contains(name, "stripe") {
		fmt.Println("[ALERT] Similar patterns found in 'previous-stripe-plugin'.")
		fmt.Println("[CONTEXT] Found related subtasks in DuckDB archives.")
	}

	logTicketCommand(name, "start", args)

	// V2: Initialize timestamps
	ticket, err := GetTicket(name)
	if err == nil {
		now := time.Now().Format(time.RFC3339)
		ticket.StartTime = now
		ticket.LastSummaryTime = now
		SaveTicket(ticket)
	}

	logInfo("Ticket %s started successfully", name)
}

func RunAck(args []string) {
	ticket, err := GetCurrentTicket()
	if err != nil {
		logFatal("Error getting current ticket: %v", err)
	}

	message := "Acknowledged messages"
	if len(args) > 0 {
		message = strings.Join(args, " ")
	}

	appendTicketLogEntry(ticket.ID, "ack", message, "")
	logInfo("Messages acknowledged for ticket %s", ticket.ID)
}

func RunGrant(args []string) {
	ticket, err := GetCurrentTicket()
	if err != nil {
		logFatal("Error getting current ticket: %v", err)
	}

	logInfo("[AUTH] Temporary resource access granted (scaffold)")
	logTicketCommand(ticket.ID, "grant", args)
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
	// Always run test commands before finalizing
	RunTest(nil)

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

	// V2: Mandatory Agent Summary from agent_summary.md
	dir := filepath.Join("src", "tickets", ticket.ID)
	summaryPath := filepath.Join(dir, "agent_summary.md")
	content, err := os.ReadFile(summaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			// If file is missing, we check if we have any historical summaries
			allSummaries, _ := ListTicketSummaries(ticket.ID)
			if len(allSummaries) == 0 {
				fmt.Printf("[EXAMPLE] Recommended format for agent_summary.md:\n\n")
				fmt.Printf("## Ran commands to find source files\n")
				fmt.Printf("1. searched with grep - result nothing\n")
				fmt.Printf("2. searched with `./dialtone.sh ticket search \"vertex node\"` - 2 results\n\n")
				logFatal("Missing mandatory agent_summary.md and no summary history found for ticket completion")
			}
			// Use history only
			finalLog := "## Unified Agent Summary\n\n"
			for _, s := range allSummaries {
				finalLog += fmt.Sprintf("### %s (%s)\n%s\n\n", s.Timestamp, s.SubtaskName, s.Content)
			}
			ticket.AgentSummary = finalLog
		} else {
			logFatal("Error reading %s: %v", summaryPath, err)
		}
	} else {
		if len(strings.TrimSpace(string(content))) == 0 {
			logFatal("agent_summary.md is empty. A final summary is required.")
		}

		allSummaries, _ := ListTicketSummaries(ticket.ID)
		finalLog := "## Unified Agent Summary\n\n"
		for _, s := range allSummaries {
			finalLog += fmt.Sprintf("### %s (%s)\n%s\n\n", s.Timestamp, s.SubtaskName, s.Content)
		}
		finalLog += "### Final Summary\n" + string(content)
		ticket.AgentSummary = finalLog

		// Cleanup the file after ingestion in done as well
		os.Remove(summaryPath)
	}

	logInfo("Finalizing ticket %s...", ticket.ID)
	logTicketCommand(ticket.ID, "done", args)

	// Backup DB to the ticket folder
	dbPath := ticketDBPath()
	backupPath := filepath.Join("src", "tickets", ticket.ID, "tickets_backup.duckdb")
	if err := backupFile(dbPath, backupPath); err != nil {
		logFatal("Could not backup database: %v", err)
	}
	logInfo("Database backup saved to %s", backupPath)

	if err := SaveTicket(ticket); err != nil {
		logFatal("Could not save final summary: %v", err)
	}
	logInfo("Ticket %s completed", ticket.ID)
}

func backupFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func RunSummary(args []string) {
	ticket, err := GetCurrentTicket()
	if err != nil {
		logFatal("Error getting current ticket: %v", err)
	}

	idle := false
	update := false
	for _, arg := range args {
		if arg == "--idle" {
			idle = true
		}
		if arg == "update" || arg == "save" {
			update = true
		}
	}

	if !idle && !update {
		summaries, err := ListTicketSummaries(ticket.ID)
		if err != nil {
			logFatal("Could not list summaries: %v", err)
		}
		fmt.Printf("# Agent Summaries for Ticket: %s\n\n", ticket.ID)
		if len(summaries) == 0 {
			fmt.Println("*No summaries captured yet.*")
			return
		}
		for _, s := range summaries {
			fmt.Printf("## [%s] %s\n", s.Timestamp, s.SubtaskName)
			fmt.Printf("%s\n\n", s.Content)
			fmt.Println("---")
		}
		return
	}

	summaryContent := ""
	if !idle {
		dir := filepath.Join("src", "tickets", ticket.ID)
		summaryPath := filepath.Join(dir, "agent_summary.md")
		content, err := os.ReadFile(summaryPath)
		if err != nil {
			fmt.Printf("[EXAMPLE] Recommended format for agent_summary.md:\n\n")
			fmt.Printf("## Ran commands to find source files\n")
			fmt.Printf("1. searched with grep - result nothing\n")
			fmt.Printf("2. searched with `./dialtone.sh ticket search \"vertex node\"` - 2 results\n\n")
			logFatal("Could not read %s: %v", summaryPath, err)
		}
		summaryContent = strings.TrimSpace(string(content))
		if summaryContent == "" {
			fmt.Printf("[EXAMPLE] Recommended format for agent_summary.md:\n\n")
			fmt.Printf("## Ran commands to find source files\n")
			fmt.Printf("1. searched with grep - result nothing\n")
			fmt.Printf("2. searched with `./dialtone.sh ticket search \"vertex node\"` - 2 results\n\n")
			logFatal("agent_summary.md is empty. Provide content or use --idle.")
		}

		// SHA256 Verification
		hash := sha256.Sum256([]byte(summaryContent))
		currentHex := hex.EncodeToString(hash[:])

		last, err := GetLastTicketSummary(ticket.ID)
		if err == nil && last != nil {
			lastHash := sha256.Sum256([]byte(last.Content))
			lastHex := hex.EncodeToString(lastHash[:])
			if currentHex == lastHex {
				logFatal("Summary content has not changed. Please update agent_summary.md.")
			}
		}

		// Auto-deletion on success (update)
		defer os.Remove(summaryPath)
	} else {
		summaryContent = "[IDLE] No work performed in this interval."
	}

	subtask := ""
	st := FindNextSubtask(ticket)
	if st != nil && st.Status == "progress" {
		subtask = st.Name
	}

	if err := AppendTicketSummary(ticket.ID, subtask, summaryContent); err != nil {
		logFatal("Could not save summary: %v", err)
	}

	ticket.LastSummaryTime = time.Now().Format(time.RFC3339)
	if err := SaveTicket(ticket); err != nil {
		logFatal("Could not update last summary time: %v", err)
	}

	logInfo("Summary captured and timer reset for ticket %s", ticket.ID)
}

func RunSearch(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ./dialtone.sh ticket search <query>")
	}
	query := strings.Join(args, " ")
	results, err := SearchTicketSummaries(query)
	if err != nil {
		logFatal("Search failed: %v", err)
	}

	fmt.Printf("Search results for \"%s\":\n", query)
	for _, r := range results {
		fmt.Printf("--- %s | %s | %s ---\n", r.TicketID, r.SubtaskName, r.Timestamp)
		fmt.Println(r.Content)
	}
}

func RunKey(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ./dialtone.sh ticket key <command> [args]")
		fmt.Println("Commands: add, list, rm, <name> <password>")
		return
	}

	sub := args[0]
	switch sub {
	case "add":
		if len(args) < 4 {
			logFatal("Usage: ./dialtone.sh ticket key add <name> <value> <password>")
		}
		name, value, password := args[1], args[2], args[3]
		salt, _ := generateSalt()
		derived := deriveKey(password, salt)
		ciphertext, nonce, err := encrypt([]byte(value), derived)
		if err != nil {
			logFatal("Encryption failed: %v", err)
		}
		key := &KeyEntry{Name: name, EncryptedValue: ciphertext, Salt: salt, Nonce: nonce}
		if err := SaveKey(key); err != nil {
			logFatal("Failed to save key: %v", err)
		}
		logInfo("Key '%s' stored securely.", name)
	case "list":
		names, err := ListKeyNames()
		if err != nil {
			logFatal("Failed to list keys: %v", err)
		}
		fmt.Println("Stored Keys:")
		for _, n := range names {
			fmt.Printf("- %s\n", n)
		}
	case "rm":
		if len(args) < 2 {
			logFatal("Usage: ./dialtone.sh ticket key rm <name>")
		}
		if err := DeleteKey(args[1]); err != nil {
			logFatal("Failed to delete key: %v", err)
		}
		logInfo("Key '%s' removed.", args[1])
	default:
		// Attempt to lease/retrieve
		name := args[0]
		if len(args) < 2 {
			logFatal("Usage: ./dialtone.sh ticket key <name> <password>")
		}
		password := args[1]
		key, err := GetKey(name)
		if err != nil || key == nil {
			logFatal("Key '%s' not found.", name)
		}
		derived := deriveKey(password, key.Salt)
		plaintext, err := decrypt(key.EncryptedValue, derived, key.Nonce)
		if err != nil {
			logFatal("Invalid password or corrupted key data.")
		}
		fmt.Print(string(plaintext))
	}
}

func RunDelete(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ./dialtone.sh ticket delete <ticket-name>")
	}
	name := args[0]

	// Delete from database
	if err := DeleteTicket(name); err != nil {
		logFatal("Could not delete ticket %s from database: %v", name, err)
	}

	// Delete from file system
	dir := filepath.Join("src", "tickets", name)
	if _, err := os.Stat(dir); err == nil {
		if err := os.RemoveAll(dir); err != nil {
			logFatal("Could not delete ticket directory %s: %v", dir, err)
		}
	}

	logInfo("Ticket %s deleted successfully", name)
}

func RunLoad(args []string) {
	ticketsDir := filepath.Join("src", "tickets")

	// Find all tickets_backup.duckdb files
	entries, err := os.ReadDir(ticketsDir)
	if err != nil {
		logFatal("Could not read tickets directory: %v", err)
	}

	loaded := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		backupPath := filepath.Join(ticketsDir, entry.Name(), "tickets_backup.duckdb")
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			continue
		}

		logInfo("Loading backup from %s...", backupPath)
		if err := LoadBackupDB(backupPath); err != nil {
			logInfo("Warning: Could not load %s: %v", backupPath, err)
			continue
		}
		loaded++
	}

	if loaded == 0 {
		logInfo("No backup files found in %s", ticketsDir)
	} else {
		logInfo("Loaded %d backup(s) into ticket system", loaded)
	}
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

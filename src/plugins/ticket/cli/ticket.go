package cli

import (
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
	case "review":
		RunReview(subArgs)
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
	fmt.Println("  review <name>      Review ticket DB/subtasks only (no tests/logs/code)")
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
	if err := ensureOnGitBranch(name); err != nil {
		logFatal("Could not switch to branch %s: %v", name, err)
	}
	dir := filepath.Join("src", "tickets", name)
	os.MkdirAll(filepath.Join(dir, "test"), 0755)

	if _, err := GetTicket(name); err != nil {
		if !errors.Is(err, ErrTicketNotFound) {
			logFatal("Could not load ticket %s: %v", name, err)
		}
		// Default: allow "init" to run without manual ticket DB edits.
		// For `www-*` tickets, the expected baseline verification is:
		// `./dialtone.sh plugin test www`
		initTestCmd := ""
		if strings.HasPrefix(name, "www-") {
			initTestCmd = "./dialtone.sh plugin test www"
		}
		ticket := &Ticket{
			ID:          name,
			Name:        name,
			Description: "",
			Subtasks: []Subtask{
				{
					Name:        "init",
					Description: "Initialization",
					TestCommand: initTestCmd,
					Status:      "todo",
				},
			},
		}
		if err := SaveTicket(ticket); err != nil {
			logFatal("Could not create ticket %s: %v", name, err)
		}
		logInfo("Created %s", ticketDBPathFor(name))
	}
	if err := SetCurrentTicket(name); err != nil {
		logFatal("Could not set current ticket %s: %v", name, err)
	}
	// `add` is prep work, but keep default mode as review to avoid implying execution.
	_ = SetCurrentTicketMode("review")

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
	_ = SetCurrentTicketMode("start")

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
		_ = ensureAllSubtaskSummaryFiles(ticket)
	}

	logInfo("Ticket %s started successfully", name)
	printDialtone(
		[]string{
			fmt.Sprintf("ticket: %s", name),
			"goal: keep work ticket-driven; run tests yourself and summarize results",
			"verify: git branch is correct and working tree is clean before starting",
		},
		"Run the next command(s) to validate environment and begin the first subtask.\nThen summarize results and what to do next.",
		[]string{
			"./dialtone.sh ticket subtask list",
			"./dialtone.sh plugin test <plugin-name>",
			"./dialtone.sh www dev",
		},
	)
}

func RunReview(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ./dialtone.sh ticket review <ticket-name>")
	}
	name := args[0]
	_ = SetCurrentTicketMode("review")

	// Ensure we are on the branch named exactly like the ticket.
	if err := ensureOnGitBranch(name); err != nil {
		logFatal("Could not switch to branch %s: %v", name, err)
	}

	// Ensure ticket exists (scaffolds if missing) and set current ticket.
	RunAdd([]string{name})

	ticket, err := GetTicket(name)
	if err != nil {
		printDialtone(
			[]string{
				fmt.Sprintf("ticket: %s", name),
				"mode: review (prep-only)",
				"blocker: ticket validation failed",
			},
			fmt.Sprintf("Dialtone could not load/validate the ticket DB.\n\nError:\n%v\n\nFix the ticket structure (subtasks/deps/status/test commands) and retry.", err),
			[]string{
				"./dialtone.sh ticket subtask list " + name,
				"./dialtone.sh ticket subtask add <name> --desc \"...\"",
				"./dialtone.sh ticket validate " + name,
				"./dialtone.sh ticket review " + name,
			},
		)
		logFatal("Could not load ticket %s: %v", name, err)
	}

	// Create per-subtask summary files to make later work smoother.
	if err := ensureAllSubtaskSummaryFiles(ticket); err != nil {
		logFatal("Could not create subtask summary files: %v", err)
	}

	logTicketCommand(name, "review", args)

	// `GetTicket` already performs structural validation; make that explicit in review output.
	logInfo("Ticket %s is valid", name)

	// Review-only heuristics: warnings are OK; goal is readiness for later `start`.
	nameSet := map[string]bool{}
	progressCount := 0
	for _, st := range ticket.Subtasks {
		n := strings.TrimSpace(st.Name)
		if n != "" {
			nameSet[n] = true
		}
		if st.Status == "progress" {
			progressCount++
		}
	}

	if len(ticket.Subtasks) == 0 {
		logWarn("Review warning: ticket has 0 subtasks")
	}
	if progressCount > 1 {
		logWarn("Review warning: %d subtasks are in progress (usually should be 1)", progressCount)
	}

	for _, st := range ticket.Subtasks {
		stName := strings.TrimSpace(st.Name)
		if stName == "" {
			logWarn("Review warning: subtask with empty name")
			continue
		}
		if strings.TrimSpace(st.Description) == "" {
			logWarn("Review warning: subtask %s has empty description", stName)
		}
		if strings.TrimSpace(st.TestCommand) == "" {
			logWarn("Review warning: subtask %s has no test command set", stName)
		}
		for _, d := range st.Dependencies {
			dep := strings.TrimSpace(d)
			if dep == "" {
				continue
			}
			if !nameSet[dep] {
				logWarn("Review warning: subtask %s depends on unknown subtask %s", stName, dep)
			}
		}
	}

	PrintTicketReport(ticket)
	printDialtone(
		[]string{
			fmt.Sprintf("ticket: %s", name),
			"mode: review (prep-only)",
			"policy: do not demand tests, logs, or code changes",
			"goal: ensure ticket DB/subtasks look correct and ready for `ticket start` later",
			"verify: branch name matches ticket name",
			"validate: ticket DB/subtasks loaded successfully",
		},
		"Review the ticket structure now:\n- subtasks have clear descriptions\n- dependencies are correct\n- each subtask has an explicit test command (when applicable)\n- summary files exist at `src/tickets/<ticket>/<subtask>-summary.md`",
		[]string{
			"./dialtone.sh ticket subtask list",
			"./dialtone.sh ticket validate " + name,
			"./dialtone.sh ticket subtask add <name> --desc \"...\"",
			"./dialtone.sh ticket start " + name,
		},
	)
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
	logInfo("Captured %s in %s", entryType, ticketDBPathFor(ticketID))
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
	ticket, err := GetCurrentTicket()
	if err != nil {
		logFatal("Error getting current ticket: %v", err)
	}

	// Mode gate: `done` is only allowed in `start` mode.
	if GetCurrentTicketMode() == "review" {
		printDialtone(
			[]string{
				fmt.Sprintf("ticket: %s", ticket.ID),
				"mode: review (prep-only)",
				"blocker: cannot finalize tickets in review mode",
			},
			"You're currently in `review` mode.\n\nUse `review` to make the ticket DB/subtasks ready.\nWhen you're ready to execute and finalize, switch to start mode:\n- `./dialtone.sh ticket start <ticket>`",
			[]string{
				"./dialtone.sh ticket review " + ticket.ID,
				"./dialtone.sh ticket start " + ticket.ID,
				"./dialtone.sh ticket done",
			},
		)
		return
	}

	// DIALTONE mode: do not run tests automatically. Guide the agent instead.
	// `done` is only allowed after the agent has run tests, reviewed logs, and submitted summary.

	// Simple validation: all subtasks must be done (or failed/skipped)
	for _, st := range ticket.Subtasks {
		if st.Status != "done" && st.Status != "failed" && st.Status != "skipped" {
			printDialtone(
				[]string{
					fmt.Sprintf("ticket: %s", ticket.ID),
					fmt.Sprintf("blocker: subtask `%s` is still %s", st.Name, st.Status),
					"process: run tests, review logs, submit summary, then mark subtask done",
				},
				"Loop until the subtask test passes and logs are clean.\nThen mark the subtask done and re-run ticket done.",
				[]string{
					"./dialtone.sh ticket subtask list",
					"./dialtone.sh plugin test <plugin-name>",
					"./dialtone.sh logs --lines 200",
					"./dialtone.sh ticket summary update",
					"./dialtone.sh ticket subtask done <ticket-name> <subtask-name>",
					"./dialtone.sh ticket done",
				},
			)
			logFatal("Subtask %s is still %s", st.Name, st.Status)
		}
	}

	// Sync current summary files into DuckDB (no deletion).
	if err := ensureAllSubtaskSummaryFiles(ticket); err != nil {
		logFatal("Could not create subtask summary files: %v", err)
	}
	// Best-effort sync; do not block on unchanged content.
	for _, st := range ticket.Subtasks {
		content, _, err := readSubtaskSummary(ticket.ID, st.Name)
		if err != nil {
			logFatal("Could not read summary file for %s: %v", st.Name, err)
		}
		content = strings.TrimSpace(content)
		if content == "" {
			// If a subtask is marked done, it should already have a summary due to gating in `subtask done`.
			continue
		}
		last, err := GetLastTicketSummaryForSubtask(ticket.ID, st.Name)
		if err == nil && last != nil && strings.TrimSpace(last.Content) == content {
			continue
		}
		if err := AppendTicketSummary(ticket.ID, st.Name, content); err != nil {
			logFatal("Could not save summary: %v", err)
		}
	}

	// Ingest summaries (history) into the consolidated field.
	allSummaries, _ := ListTicketSummaries(ticket.ID)
	finalLog := "## Unified Agent Summary\n\n"
	for _, s := range allSummaries {
		finalLog += fmt.Sprintf("### %s (%s)\n%s\n\n", s.Timestamp, s.SubtaskName, s.Content)
	}
	ticket.AgentSummary = finalLog

	logInfo("Finalizing ticket %s...", ticket.ID)
	logTicketCommand(ticket.ID, "done", args)

	// Backup DB to the ticket folder
	dbPath := ticketDBPathFor(ticket.ID)
	backupPath := filepath.Join("src", "tickets", ticket.ID, ticket.ID+"-backup.duckdb")
	if err := backupFile(dbPath, backupPath); err != nil {
		logFatal("Could not backup database: %v", err)
	}
	logInfo("Database backup saved to %s", backupPath)

	if err := SaveTicket(ticket); err != nil {
		logFatal("Could not save final summary: %v", err)
	}
	logInfo("Ticket %s completed", ticket.ID)

	printDialtone(
		[]string{
			fmt.Sprintf("ticket: %s", ticket.ID),
			fmt.Sprintf("backup: %s", backupPath),
			"next: make a git commit (manual) and open a PR if needed",
		},
		"Please verify git status is clean after committing.\nThis tool intentionally does not run git commands automatically.",
		[]string{
			"git status -sb",
			"git add .",
			"git commit -m \"Describe your changes\"",
			"./dialtone.sh github pr --draft",
		},
	)
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

	// Ensure summary files exist (persistent per-subtask).
	if err := ensureAllSubtaskSummaryFiles(ticket); err != nil {
		logFatal("Could not create subtask summary files: %v", err)
	}

	if idle {
		if err := AppendTicketSummary(ticket.ID, "", "[IDLE] No work performed in this interval."); err != nil {
			logFatal("Could not save summary: %v", err)
		}
		ticket.LastSummaryTime = time.Now().Format(time.RFC3339)
		if err := SaveTicket(ticket); err != nil {
			logFatal("Could not update last summary time: %v", err)
		}
		logInfo("Summary captured and timer reset for ticket %s", ticket.ID)
		return
	}

	// Sync summaries from <subtask>-summary.md files into DuckDB.
	active := ""
	st := FindNextSubtask(ticket)
	if st != nil && st.Status == "progress" {
		active = st.Name
	}

	if active != "" {
		content, path, err := readSubtaskSummary(ticket.ID, active)
		if err != nil {
			logFatal("Could not read summary file: %v", err)
		}
		if strings.TrimSpace(content) == "" {
			fmt.Printf("[BLOCK] Subtask summary is required before continuing.\n")
			fmt.Printf("[MESSAGE] Please update: %s\n", path)
			fmt.Printf("[EXAMPLE] Suggested template:\n\n")
			fmt.Printf("## What changed\n- ...\n\n## Commands / verification\n- ...\n\n")
			fmt.Printf("[ACTION] Update the file and run: ./dialtone.sh ticket summary update\n")
			return
		}
	}

	updated := 0
	for _, s := range ticket.Subtasks {
		name := strings.TrimSpace(s.Name)
		if name == "" {
			continue
		}
		content, _, err := readSubtaskSummary(ticket.ID, name)
		if err != nil {
			logFatal("Could not read summary file for %s: %v", name, err)
		}
		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}

		last, err := GetLastTicketSummaryForSubtask(ticket.ID, name)
		if err == nil && last != nil && strings.TrimSpace(last.Content) == content {
			continue
		}

		if err := AppendTicketSummary(ticket.ID, name, content); err != nil {
			logFatal("Could not save summary: %v", err)
		}
		updated++
	}

	ticket.LastSummaryTime = time.Now().Format(time.RFC3339)
	if err := SaveTicket(ticket); err != nil {
		logFatal("Could not update last summary time: %v", err)
	}

	if updated == 0 {
		logInfo("No summary changes detected; timer reset for ticket %s", ticket.ID)
	} else {
		logInfo("Synced %d subtask summary file(s); timer reset for ticket %s", updated, ticket.ID)
	}
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

	// Find all *-backup.duckdb files
	entries, err := os.ReadDir(ticketsDir)
	if err != nil {
		logFatal("Could not read tickets directory: %v", err)
	}

	loaded := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		ticketID := entry.Name()
		backupPath := filepath.Join(ticketsDir, ticketID, ticketID+"-backup.duckdb")
		if _, err := os.Stat(backupPath); err != nil {
			// Legacy name (pre-rename)
			legacy := filepath.Join(ticketsDir, ticketID, "tickets_backup.duckdb")
			if _, err2 := os.Stat(legacy); err2 != nil {
				continue
			}
			backupPath = legacy
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

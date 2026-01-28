package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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
	fmt.Println("Commands: add, start, ask, log, list, validate, next, done, subtask, test")
}

func RunAdd(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ./dialtone.sh ticket add <ticket-name>")
	}
	name := args[0]
	dir := filepath.Join("src", "tickets", name)
	os.MkdirAll(filepath.Join(dir, "test"), 0755)

	ticketPath := filepath.Join(dir, "ticket.md")
	if _, err := os.Stat(ticketPath); os.IsNotExist(err) {
		content := fmt.Sprintf("# Name: %s\n\n# Goal\n\n## SUBTASK: Init\n- name: init\n- description: Initialization\n- status: todo\n", name)
		os.WriteFile(ticketPath, []byte(content), 0644)
		logInfo("Created %s", ticketPath)
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
	
	// Check if branch exists
	cmdCheck := exec.Command("git", "branch", "--list", name)
	outputCheck, _ := cmdCheck.Output()
	if len(strings.TrimSpace(string(outputCheck))) > 0 {
		logInfo("Branch %s already exists, switching...", name)
		checkoutCmd := exec.Command("git", "checkout", name)
		if output, err := checkoutCmd.CombinedOutput(); err != nil {
			logFatal("Git checkout failed: %v\nOutput: %s", err, string(output))
		}
	} else {
		logInfo("Branching to %s...", name)
		checkoutCmd := exec.Command("git", "checkout", "-b", name)
		if output, err := checkoutCmd.CombinedOutput(); err != nil {
			logFatal("Git checkout failed: %v\nOutput: %s", err, string(output))
		}
	}

	RunAdd(args)
	logTicketCommand(name, "start", args)

	addCmd := exec.Command("git", "add", ".")
	if output, err := addCmd.CombinedOutput(); err != nil {
		logFatal("Git add failed: %v\nOutput: %s", err, string(output))
	}
	commitCmd := exec.Command("git", "commit", "-m", fmt.Sprintf("chore: start ticket %s", name)) 
	if output, err := commitCmd.CombinedOutput(); err != nil {
		logFatal("Git commit failed: %v\nOutput: %s", err, string(output))
	}
	
	logInfo("Pushing branch %s to origin...", name)
	pushCmd := exec.Command("git", "push", "-u", "origin", name)
	if output, err := pushCmd.CombinedOutput(); err != nil {
		logFatal("Git push failed: %v\nOutput: %s", err, string(output))
	}
	
	// PR logic (mocked or calls gh)
	logInfo("Creating Draft Pull Request...")
	prCmd := exec.Command("gh", "pr", "create", "--draft", "--title", name, "--body", "Automated ticket PR")
	if output, err := prCmd.CombinedOutput(); err != nil {
		logInfo("Note: gh pr create failed (likely auth or remote): %v\nOutput: %s", err, string(output))
	}
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
	logPath, err := ensureTicketLog(ticketID)
	if err != nil {
		logFatal("Could not initialize log for %s: %v", ticketID, err)
	}

	entry := fmt.Sprintf("## %s\n", time.Now().Format(time.RFC3339))
	if subtask != "" {
		entry += fmt.Sprintf("- subtask: %s\n", subtask)
	}
	entry += fmt.Sprintf("- %s: %s\n\n", entryType, message)

	file, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		logFatal("Could not write %s: %v", logPath, err)
	}
	defer file.Close()

	if _, err := file.WriteString(entry); err != nil {
		logFatal("Could not write %s: %v", logPath, err)
	}

	logInfo("Captured %s in %s", entryType, logPath)
}

func logTicketCommand(ticketID, command string, args []string) {
	if ticketID == "" || command == "" {
		return
	}

	message := fmt.Sprintf("ticket %s %s", command, strings.Join(args, " "))
	message = strings.TrimSpace(message)
	appendTicketLogEntry(ticketID, "command", message, "")
}

func ensureTicketLog(ticketID string) (string, error) {
	if ticketID == "" {
		return "", fmt.Errorf("ticket ID is empty")
	}
	dir := filepath.Join("src", "tickets", ticketID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	logPath := filepath.Join(dir, "log.md")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		if err := os.WriteFile(logPath, []byte("# Log\n\n"), 0644); err != nil {
			return "", err
		}
	}
	return logPath, nil
}

func RunList(args []string) {
	dir := "src/tickets"
	files, err := os.ReadDir(dir)
	if err != nil {
		logFatal("Could not read tickets directory: %v", err)
	}
	fmt.Println("Tickets (v2):")
	for _, f := range files {
		if f.IsDir() {
			fmt.Printf("- %s\n", f.Name())
		}
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
	path := filepath.Join("src", "tickets", name, "ticket.md")
	_, err := ParseTicketMd(path)
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
	logTicketCommand(ticket.ID, "done", args)
	for _, st := range ticket.Subtasks {
		if st.Status != "done" && st.Status != "failed" && st.Status != "skipped" {
			logFatal("Subtask %s is still %s", st.Name, st.Status)
		}
	}

	logInfo("Finalizing ticket %s...", ticket.ID)

	// Check git hygiene
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusOutput, _ := statusCmd.Output()
	if len(strings.TrimSpace(string(statusOutput))) > 0 {
		logFatal("Git status is not clean. Please commit or stash changes before running 'done'.")
	}

	logInfo("Pushing final changes...")
	pushCmd := exec.Command("git", "push")
	if output, err := pushCmd.CombinedOutput(); err != nil {
		logFatal("Git push failed: %v\nOutput: %s", err, string(output))
	}
	
	logInfo("Marking PR as ready for review...")
	readyCmd := exec.Command("gh", "pr", "ready")
	if output, err := readyCmd.CombinedOutput(); err != nil {
		logInfo("Note: gh pr ready failed: %v\nOutput: %s", err, string(output))
	}
	
	logInfo("Switching back to main branch...")
	checkoutCmd := exec.Command("git", "checkout", "main")
	if output, err := checkoutCmd.CombinedOutput(); err != nil {
		logFatal("Git checkout failed: %v\nOutput: %s", err, string(output))
	}
	logInfo("Ticket %s completed", ticket.ID)
}

func GetCurrentTicket() (*Ticket, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(string(output))
	path := filepath.Join("src", "tickets", name, "ticket.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("no ticket found for branch %s at %s", name, path)
	}
	return ParseTicketMd(path)
}

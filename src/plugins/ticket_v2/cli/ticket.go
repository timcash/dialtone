package cli

import (
	"fmt"
	"os"
	"os/exec"
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
		fmt.Printf("Unknown ticket_v2 subcommand: %s\n", subcommand)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh ticket_v2 <command> [args]")
	fmt.Println("Commands: add, start, list, validate, next, done, subtask, test")
}

func RunAdd(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ./dialtone.sh ticket_v2 add <ticket-name>")
	}
	name := args[0]
	dir := filepath.Join("src", "tickets_v2", name)
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
}

func RunStart(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ./dialtone.sh ticket_v2 start <ticket-name>")
	}
	name := args[0]
	RunAdd(args)

	// Git logic
	logInfo("Branching to %s...", name)
	exec.Command("git", "checkout", "-b", name).Run()
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", fmt.Sprintf("chore: start ticket %s", name)).Run()
	logInfo("Pushing branch %s to origin...", name)
	exec.Command("git", "push", "-u", "origin", name).Run()
	
	// PR logic (mocked or calls gh)
	logInfo("Creating Draft Pull Request...")
	exec.Command("gh", "pr", "create", "--draft", "--title", name, "--body", "Automated ticket PR").Run()
	logInfo("Ticket %s started successfully", name)
}

func RunList(args []string) {
	dir := "src/tickets_v2"
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
}

func RunValidate(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ./dialtone.sh ticket_v2 validate <ticket-name>")
	}
	name := args[0]
	path := filepath.Join("src", "tickets_v2", name, "ticket.md")
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
	exec.Command("git", "push").Run()
	logInfo("Marking PR as ready for review...")
	exec.Command("gh", "pr", "ready").Run()
	logInfo("Switching back to main branch...")
	exec.Command("git", "checkout", "main").Run()
	logInfo("Ticket %s completed", ticket.ID)
}

func GetCurrentTicket() (*Ticket, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(string(output))
	path := filepath.Join("src", "tickets_v2", name, "ticket.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("no ticket found for branch %s at %s", name, path)
	}
	return ParseTicketMd(path)
}

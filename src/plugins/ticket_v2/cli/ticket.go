package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func logInfo(format string, args ...interface{}) {
	fmt.Printf("[ticket_v2] "+format+"\n", args...)
}

func logFatal(format string, args ...interface{}) {
	fmt.Printf("[ticket_v2] FATAL: "+format+"\n", args...)
	os.Exit(1)
}

// Run handles all ticket_v2 subcommands
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
	fmt.Println("Usage: dialtone.sh ticket_v2 <subcommand> [options]")
	fmt.Println("\nSubcommands:")
	fmt.Println("  add [<name>]       Add a new local ticket")
	fmt.Println("  start <name>       Start a new ticket (branch + scaffold + PR)")
	fmt.Println("  next               Automated TDD loop")
	fmt.Println("  list               List local tickets and remote issues")
	fmt.Println("  validate [<name>]  Validate ticket.md format")
	fmt.Println("  done [<name>]      Verify and complete ticket")
	fmt.Println("  subtask <subcmd>   Manage subtasks")
	fmt.Println("  test [<name>]      Test all subtasks")
}

func RunAdd(args []string) {
	ticketName := ""
	if len(args) > 0 {
		ticketName = args[0]
	} else {
		ticketName = GetCurrentBranch()
	}

	if ticketName == "" {
		logFatal("Usage: ticket_v2 add <name> (or run from a feature branch)")
	}

	ScaffoldTicket(ticketName)
	logInfo("Ticket %s added successfully", ticketName)
}

func RunStart(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ticket_v2 start <name>")
	}

	ticketName := args[0]

	// 1. Git Branching
	cmd := exec.Command("git", "branch", "--list", ticketName)
	output, _ := cmd.Output()
	if strings.TrimSpace(string(output)) == "" {
		logInfo("Creating new branch: %s", ticketName)
		exec.Command("git", "checkout", "-b", ticketName).Run()
	} else {
		logInfo("Switching to existing branch: %s", ticketName)
		exec.Command("git", "checkout", ticketName).Run()
	}

	// 2. Scaffolding
	ScaffoldTicket(ticketName)

	// 3. Commit and Push
	logInfo("Committing scaffolding...")
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "chore: start ticket "+ticketName).Run()

	logInfo("Pushing branch to origin...")
	exec.Command("git", "push", "-u", "origin", ticketName).Run()

	// 4. Draft PR
	logInfo("Creating Draft Pull Request...")
	// Assuming github plugin is available via dialtone.sh
	exec.Command("./dialtone.sh", "github", "pr", "--draft").Run()

	logInfo("Ticket %s started successfully", ticketName)
}

func ScaffoldTicket(name string) {
	dir := filepath.Join("src", "tickets_v2", name)
	os.MkdirAll(dir, 0755)
	os.MkdirAll(filepath.Join(dir, "test"), 0755)

	ticketMd := filepath.Join(dir, "ticket.md")
	if _, err := os.Stat(ticketMd); os.IsNotExist(err) {
		ticket := &Ticket{
			ID:          name,
			Name:        name,
			Description: "Implement the " + name + " feature.",
			Status:      "todo",
			Subtasks: []Subtask{
				{
					Name:        "example-subtask",
					Description: "This is an example subtask.",
					Status:      "todo",
					TestConditions: []TestCondition{
						{Condition: "the system works"},
					},
				},
			},
		}
		WriteTicketMd(ticketMd, ticket)
		logInfo("Created %s", ticketMd)
	}

	testGo := filepath.Join(dir, "test", "test.go")
	if _, err := os.Stat(testGo); os.IsNotExist(err) {
		content := fmt.Sprintf(`package test

import (
	"dialtone/cli/src/dialtest"
)

func init() {
	dialtest.RegisterTicket("%%s")
	dialtest.AddSubtaskTest("example-subtask", RunExample, []string{"example"})
}

func RunExample() error {
	return nil
}
`, name)
		os.WriteFile(testGo, []byte(content), 0644)
		logInfo("Created %s", testGo)
	}
}

func RunList(args []string) {
	files, _ := os.ReadDir(filepath.Join("src", "tickets_v2"))
	fmt.Println("\nLocal Tickets (v2):")
	fmt.Println("---------------------------------------------------")
	for _, f := range files {
		if f.IsDir() {
			fmt.Printf("- %s\n", f.Name())
		}
	}
	fmt.Println("---------------------------------------------------")
}

func RunValidate(args []string) {
	name := ""
	if len(args) > 0 {
		name = args[0]
	} else {
		name = GetCurrentBranch()
	}

	if name == "" {
		logFatal("Could not determine ticket name")
	}

	path := filepath.Join("src", "tickets_v2", name, "ticket.md")
	_, err := ParseTicketMd(path)
	if err != nil {
		logFatal("Validation failed: %v", err)
	}
	logInfo("Ticket %s is valid", name)
}

func RunDone(args []string) {
	name := ""
	if len(args) > 0 {
		name = args[0]
	} else {
		name = GetCurrentBranch()
	}

	if name == "" {
		logFatal("Could not determine ticket name")
	}

	path := filepath.Join("src", "tickets_v2", name, "ticket.md")
	ticket, err := ParseTicketMd(path)
	if err != nil {
		logFatal("Failed to parse ticket: %v", err)
	}

	for _, st := range ticket.Subtasks {
		if st.Status != "done" && st.Status != "failed" {
			logFatal("Subtask %s is still %s", st.Name, st.Status)
		}
	}

	logInfo("Finalizing ticket %s...", name)
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "chore: complete ticket "+name).Run()
	exec.Command("git", "push").Run()
	exec.Command("./dialtone.sh", "github", "pr", "--ready").Run()
	exec.Command("git", "checkout", "main").Run()

	logInfo("Ticket %s completed", name)
}

func GetCurrentBranch() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, _ := cmd.Output()
	branch := strings.TrimSpace(string(output))
	if branch == "main" || branch == "master" || branch == "" {
		return ""
	}
	return branch
}

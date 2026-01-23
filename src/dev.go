package dialtone

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"dialtone/cli/src/core/ssh"
	build_cli "dialtone/cli/src/plugins/build/cli"
	chrome_cli "dialtone/cli/src/plugins/chrome/cli"
	deploy_cli "dialtone/cli/src/plugins/deploy/cli"
	github_cli "dialtone/cli/src/plugins/github/cli"
	install_cli "dialtone/cli/src/plugins/install/cli"
	logs_cli "dialtone/cli/src/plugins/logs/cli"
	mavlink_cli "dialtone/cli/src/plugins/mavlink/cli"
	plugin_cli "dialtone/cli/src/plugins/plugin/cli"
	ticket_cli "dialtone/cli/src/plugins/ticket/cli"
	www_cli "dialtone/cli/src/plugins/www/cli"
)

// ExecuteDev is the entry point for the dialtone-dev CLI
func ExecuteDev() {
	if len(os.Args) < 2 {
		printDevUsage()
		return
	}

	// Load configuration
	LoadConfig()

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "build":
		build_cli.RunBuild(args)
	case "deploy":
		deploy_cli.RunDeploy(args)
	case "ssh":
		ssh.RunSSH(args)
	case "provision":
		RunProvision(args)
	case "logs":
		logs_cli.RunLogs(args)
	case "diagnostic":
		RunDiagnostic(args)
	case "install":
		install_cli.RunInstall(args)
	case "clone":
		RunClone(args)
	case "sync-code":
		RunSyncCode(args)
	case "plan":
		runPlan(args)
	case "branch":
		runBranch(args)
	case "test":
		runTest(args)
	case "pull-request", "pr":
		// Delegate to github plugin
		github_cli.RunGithub(append([]string{"pull-request"}, args...))
	case "github":
		github_cli.RunGithub(args)
	case "ticket":
		runTicket(args)
	case "plugin":
		plugin_cli.RunPlugin(args)
	case "chrome":
		chrome_cli.RunChrome(args)
	case "mavlink":
		mavlink_cli.RunMavlink(args)
	case "www":
		www_cli.RunWww(args)
	case "opencode":
		runOpencode(args)
	case "developer":
		runDeveloper(args)
	case "subagent":
		runSubagent(args)
	case "docs":
		runDocs(args)
	case "help", "-h", "--help":
		printDevUsage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printDevUsage()
		os.Exit(1)
	}
}

func printDevUsage() {
	fmt.Println("Usage: dialtone-dev <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  install [path] Install dependencies (--linux-wsl for WSL, --macos-arm for Apple Silicon)")
	fmt.Println("  build         Build web UI and binary (--local, --full, --remote, --podman, --linux-arm, --linux-arm64)")
	fmt.Println("  deploy        Deploy to remote robot")
	fmt.Println("  clone         Clone or update the repository")
	fmt.Println("  sync-code     Sync source code to remote robot")
	fmt.Println("  ssh           SSH tools (upload, download, cmd)")
	fmt.Println("  provision     Generate Tailscale Auth Key")
	fmt.Println("  logs          Tail remote logs")
	fmt.Println("  diagnostic    Run system diagnostics (local or remote)")
	fmt.Println("  plan [name]        List plans or create/view a plan file")
	fmt.Println("  branch <name>      Create or checkout a feature branch")
	fmt.Println("  test [name]        Run tests (all or for specific feature, creates templates if missing)")
	fmt.Println("  pull-request       Create or update a pull request (wrapper around gh CLI)")
	fmt.Println("  ticket <subcmd>    Manage GitHub tickets (wrapper around gh CLI)")
	fmt.Println("  plugin <subcmd>    Manage plugins (create, etc.)")
	fmt.Println("  github <subcmd>    Manage GitHub interactions (pr, check-deploy)")
	fmt.Println("  www <subcmd>       Manage public webpage (Vercel wrapper)")
	fmt.Println("  opencode <subcmd>  Manage opencode AI assistant (start, stop, status, ui)")
	fmt.Println("  developer          Start the autonomous developer loop")
	fmt.Println("  subagent <options> Interface for autonomous subagents")
	fmt.Println("  docs               Update documentation")
	fmt.Println("  help               Show this help message")
}

// runDocs handles the docs command
func runDocs(args []string) {
	LogInfo("Updating documentation...")

	// 1. Capture dialtone-dev help output
	// We need to re-run the current binary with "help" argument
	// However, we are running as "go run ...", so os.Args[0] is a temporary binary.
	// That's fine for capturing output.
	cmd := exec.Command(os.Args[0], "help")
	output, err := cmd.Output()
	if err != nil {
		LogFatal("Failed to run help command: %v", err)
	}

	helpOutput := string(output)

	// 2. Parse help output to extract commands
	lines := strings.Split(helpOutput, "\n")
	var commands []string
	capture := false
	for _, line := range lines {
		if strings.Contains(line, "Commands:") {
			capture = true
			continue
		}
		if capture && strings.TrimSpace(line) != "" {
			commands = append(commands, strings.TrimSpace(line))
		}
	}

	// 3. Format as markdown list
	var markdownLines []string
	markdownLines = append(markdownLines, "### Development CLI (`dialtone.sh`)")
	markdownLines = append(markdownLines, "")

	for i, cmdLine := range commands {
		parts := strings.Fields(cmdLine)
		if len(parts) >= 2 {
			cmdName := parts[0]
			desc := strings.Join(parts[1:], " ")
			markdownLines = append(markdownLines, fmt.Sprintf("%d. `./dialtone.sh %s` — %s", i+1, cmdName, desc))

			// Add examples based on command name
			example := ""
			switch cmdName {
			case "install":
				example = "./dialtone.sh install --linux-wsl"
			case "build":
				example = "./dialtone.sh build --local"
			case "deploy":
				example = "./dialtone.sh deploy"
			case "clone":
				example = "./dialtone.sh clone ./dialtone"
			case "sync-code":
				example = "./dialtone.sh sync-code"
			case "ssh":
				example = "./dialtone.sh ssh download /tmp/log.txt"
			case "provision":
				example = "./dialtone.sh provision"
			case "logs":
				example = "./dialtone.sh logs"
			case "diagnostic":
				example = "./dialtone.sh diagnostic --remote"
			case "branch":
				example = "./dialtone.sh branch my-feature"
			case "plan":
				example = "./dialtone.sh plan my-feature"
			case "test":
				example = "./dialtone.sh test my-feature"
			case "pull-request":
				example = "./dialtone.sh pull-request --draft"
			case "ticket":
				example = "./dialtone.sh ticket view 20"
			case "github":
				example = "./dialtone.sh github pull-request --draft"
			case "www":
				example = "./dialtone.sh www publish"
			case "developer":
				example = "./dialtone.sh developer --capability camera"
			case "subagent":
				example = "./dialtone.sh subagent --task features/fix-logic/task.md"
			case "docs":
				example = "./dialtone.sh docs"
			}

			if example != "" {
				markdownLines = append(markdownLines, fmt.Sprintf("   - Example: `%s`", example))
			}
		}
	}

	newContent := strings.Join(markdownLines, "\n")

	// 4. Update AGENT.md
	agentMdPath := "AGENT.md"
	content, err := os.ReadFile(agentMdPath)
	if err != nil {
		LogFatal("Failed to read AGENT.md: %v", err)
	}

	text := string(content)

	// Regex to find the section
	// We want to replace everything from "### Development CLI (`dialtone-dev.go`)" up to the next "---"
	re := regexp.MustCompile(`(?s)### Development CLI \(` + "`" + `dialtone-dev\.go` + "`" + `\).*?(---)`)

	if !re.MatchString(text) {
		LogFatal("Could not find Development CLI section in AGENT.md")
	}

	// Replace content, keeping the trailing separator
	updatedText := re.ReplaceAllString(text, newContent+"\n\n$1")

	if err := os.WriteFile(agentMdPath, []byte(updatedText), 0644); err != nil {
		LogFatal("Failed to write AGENT.md: %v", err)
	}

	LogInfo("AGENT.md updated successfully!")
}

// runPlan handles the plan command
func runPlan(args []string) {
	planDir := "plan"

	// Ensure plan directory exists
	if err := os.MkdirAll(planDir, 0755); err != nil {
		LogFatal("Failed to create plan directory: %v", err)
	}

	// No args: list all plans
	if len(args) == 0 {
		listPlans(planDir)
		return
	}

	// With name: create or show plan
	name := args[0]
	planFile := filepath.Join(planDir, fmt.Sprintf("plan-%s.md", name))

	if _, err := os.Stat(planFile); os.IsNotExist(err) {
		// Create new plan from template
		createPlan(planFile, name)
	} else {
		// Show existing plan
		showPlan(planFile)
	}
}

// listPlans lists all plan files with their completion status
func listPlans(planDir string) {
	entries, err := os.ReadDir(planDir)
	if err != nil {
		LogFatal("Failed to read plan directory: %v", err)
	}

	fmt.Println("Plan Files:")
	fmt.Println("===========")

	planFound := false
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "plan-") && strings.HasSuffix(entry.Name(), ".md") {
			planFound = true
			planPath := filepath.Join(planDir, entry.Name())
			completed, total := countProgress(planPath)

			// Extract feature name from filename
			name := strings.TrimPrefix(entry.Name(), "plan-")
			name = strings.TrimSuffix(name, ".md")

			status := "[ ]"
			if total > 0 {
				if completed == total {
					status = "[x]"
				} else if completed > 0 {
					status = "[~]"
				}
			}

			fmt.Printf("  %s %s [%d/%d] %s\n", status, name, completed, total, entry.Name())
		}
	}

	if !planFound {
		fmt.Println("  No plan files found.")
		fmt.Println("\nCreate a new plan with: dialtone-dev plan <feature-name>")
	}
}

// countProgress counts completed items (- [x]) vs total items (- [ ] or - [x])
func countProgress(planPath string) (completed, total int) {
	content, err := os.ReadFile(planPath)
	if err != nil {
		return 0, 0
	}

	lines := strings.Split(string(content), "\n")
	checkboxPattern := regexp.MustCompile(`^- \[([ xX])\]`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		matches := checkboxPattern.FindStringSubmatch(trimmed)
		if len(matches) > 1 {
			total++
			if matches[1] == "x" || matches[1] == "X" {
				completed++
			}
		}
	}

	return completed, total
}

// createPlan creates a new plan file from template
func createPlan(planPath, name string) {
	template := fmt.Sprintf(`# Plan: %s

## Goal
[Describe the goal of this feature]

## Tests
- [ ] test_example_1: [Description of first test]
- [ ] test_example_2: [Description of second test]

## Notes
- [Add any relevant notes]

## Blocking Tickets
- None

## Progress Log
- %s: Created plan file
`, name, time.Now().Format("2006-01-02"))

	if err := os.WriteFile(planPath, []byte(template), 0644); err != nil {
		LogFatal("Failed to create plan file: %v", err)
	}

	LogInfo("Created plan file: %s", planPath)
	fmt.Println("\nPlan Template Created:")
	fmt.Println("======================")
	fmt.Println(template)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Edit the plan file to define your goal and tests")
	fmt.Println("  2. Create a branch: dialtone-dev branch", name)
	fmt.Println("  3. Start implementing tests from the plan")
}

// showPlan displays the contents of a plan file
func showPlan(planPath string) {
	content, err := os.ReadFile(planPath)
	if err != nil {
		LogFatal("Failed to read plan file: %v", err)
	}

	completed, total := countProgress(planPath)

	fmt.Println("Plan File:", planPath)
	fmt.Printf("Progress: %d/%d tests completed\n", completed, total)
	fmt.Println("======================")
	fmt.Println(string(content))
}

// runBranch handles the branch command
func runBranch(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: dialtone-dev branch <name>")
		fmt.Println("\nThis command creates or checks out a feature branch.")
		os.Exit(1)
	}

	branchName := args[0]

	// Check if branch exists
	cmd := exec.Command("git", "branch", "--list", branchName)
	output, err := cmd.Output()
	if err != nil {
		LogFatal("Failed to check branches: %v", err)
	}

	if strings.TrimSpace(string(output)) != "" {
		// Branch exists, checkout
		LogInfo("Branch '%s' exists, checking out...", branchName)
		cmd = exec.Command("git", "checkout", branchName)
	} else {
		// Branch doesn't exist, create
		LogInfo("Creating new branch '%s'...", branchName)
		cmd = exec.Command("git", "checkout", "-b", branchName)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		LogFatal("Git operation failed: %v", err)
	}

	LogInfo("Now on branch: %s", branchName)
}

// runTest handles the test command
func runTest(args []string) {
	var testPackages []string

	if len(args) == 0 {
		LogInfo("Running all tests (core, plugins, and tickets)...")
		testPackages = []string{"./test/...", "./src/plugins/...", "./tickets/..."}
	} else {
		name := args[0]

		// Hierarchical Discovery:
		// 1. Check tickets directory
		ticketTestDir := filepath.Join("tickets", name, "test")
		if _, err := os.Stat(ticketTestDir); err == nil {
			LogInfo("Running ticket tests for: %s", name)
			testPackages = []string{"./" + ticketTestDir + "/..."}
		} else {
			// 2. Check plugins directory
			pluginTestDir := filepath.Join("src", "plugins", name, "test")
			if _, err := os.Stat(pluginTestDir); err == nil {
				LogInfo("Running plugin tests for: %s", name)
				testPackages = []string{"./" + pluginTestDir + "/..."}
			} else {
				// 3. Fallback to global test directory
				testDir := filepath.Join("test", name)
				LogInfo("Running core tests for: %s", name)
				ensureTestFiles(testDir, name)
				testPackages = []string{"./" + testDir + "/..."}
			}
		}
	}

	testArgs := append([]string{"test", "-v", "-p", "1"}, testPackages...)
	cmd := exec.Command("go", testArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	var testErr error
	testErr = cmd.Run()

	// Run live web tests if in the dialtone context or if specifically requested
	if len(args) == 0 || (len(args) > 0 && args[0] == "www") {
		runLiveWebTest()
	}

	// Exit with Go test error if there was one
	if testErr != nil {
		if exitErr, ok := testErr.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		LogFatal("Failed to run tests: %v", testErr)
	}
}

// runLiveWebTest runs the Puppeteer live site verification test
func runLiveWebTest() {
	testScript := filepath.Join("dialtone-earth", "test", "live_test.ts")
	if _, err := os.Stat(testScript); os.IsNotExist(err) {
		return
	}

	LogInfo("Running Puppeteer live site verification...")

	// Command to source NVM and run the test
	// We use bash -c to ensure environment sourcing works
	script := fmt.Sprintf(`export NVM_DIR="$HOME/.nvm" && [ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh" && nvm use 22 && npx ts-node test/live_test.ts`)

	cmd := exec.Command("bash", "-c", script)
	cmd.Dir = "dialtone-earth"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		LogInfo("Puppeteer test failed (this is expected if system libraries are missing)")
		// We don't exit here as web tests might be optional or environment-dependent
	} else {
		LogInfo("Puppeteer live site verification successful!")
	}
}

// ensureTestFiles creates the test directory and template test files if they don't exist
func ensureTestFiles(testDir, featureName string) {
	// Create test directory if it doesn't exist
	if err := os.MkdirAll(testDir, 0755); err != nil {
		LogFatal("Failed to create test directory: %v", err)
	}

	// Convert feature-name to package name (replace - with _)
	packageName := strings.ReplaceAll(featureName, "-", "_")

	// Define test file templates
	testFiles := map[string]string{
		"unit_test.go":        generateUnitTestTemplate(packageName, featureName),
		"integration_test.go": generateIntegrationTestTemplate(packageName, featureName),
		"end_to_end_test.go":  generateEndToEndTestTemplate(packageName, featureName),
	}

	for filename, template := range testFiles {
		filePath := filepath.Join(testDir, filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			if err := os.WriteFile(filePath, []byte(template), 0644); err != nil {
				LogFatal("Failed to create test file %s: %v", filename, err)
			}
			LogInfo("Created test file: %s", filePath)
		} else {
			LogInfo("Test file already exists: %s", filePath)
		}
	}
}

// generateUnitTestTemplate creates a unit test template
func generateUnitTestTemplate(packageName, featureName string) string {
	return fmt.Sprintf(`package %s

import (
	"testing"

	dialtone "dialtone/cli/src"
)

// Unit tests: Simple tests that run locally without IO operations
// These tests should be fast and test individual functions/components

func TestUnit_Example(t *testing.T) {
	dialtone.LogInfo("Running unit test for %s")
	
	// TODO: Add your unit tests here
	t.Log("not yet implemented")
}

func TestUnit_Validation(t *testing.T) {
	// Test input validation, data parsing, etc.
	dialtone.LogInfo("Testing validation for %s")
	
	// TODO: Add validation tests
	t.Log("not yet implemented")
}
`, packageName, featureName, featureName)
}

// generateIntegrationTestTemplate creates an integration test template
func generateIntegrationTestTemplate(packageName, featureName string) string {
	return fmt.Sprintf(`package %s

import (
	"os"
	"path/filepath"
	"testing"

	dialtone "dialtone/cli/src"
)

// Integration tests: Test 2+ components together using test_data/
// These tests may use files, but should not require network or external services

func TestIntegration_Example(t *testing.T) {
	dialtone.LogInfo("Running integration test for %s")
	
	// Get project root for test data access
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %%v", err)
	}
	projectRoot := filepath.Join(cwd, "..", "..")
	_ = projectRoot // Use for accessing test_data/
	
	// TODO: Add your integration tests here
	t.Log("not yet implemented")
}

func TestIntegration_Components(t *testing.T) {
	// Test how multiple components work together
	dialtone.LogInfo("Testing component integration for %s")
	
	// TODO: Add component integration tests
	t.Log("not yet implemented")
}
`, packageName, featureName, featureName)
}

// generateEndToEndTestTemplate creates an end-to-end test template
func generateEndToEndTestTemplate(packageName, featureName string) string {
	return fmt.Sprintf(`package %s

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	dialtone "dialtone/cli/src"
)

// End-to-end tests: Browser and CLI tests on a live system or simulator
// These tests may require network, external services, or user interaction setup

func TestE2E_CLICommand(t *testing.T) {
	// Skip if running in CI without required setup
	if os.Getenv("SKIP_E2E") != "" {
		t.Skip("Skipping E2E test (SKIP_E2E is set)")
	}
	
	dialtone.LogInfo("Running E2E CLI test for %s")
	
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %%v", err)
	}
	projectRoot := filepath.Join(cwd, "..", "..")
	_ = projectRoot
	
	// TODO: Add your end-to-end CLI tests here
	t.Log("not yet implemented")
}

func TestE2E_FullWorkflow(t *testing.T) {
	if os.Getenv("SKIP_E2E") != "" {
		t.Skip("Skipping E2E test (SKIP_E2E is set)")
	}
	
	dialtone.LogInfo("Running full workflow E2E test for %s")
	
	// TODO: Test complete user workflows
	t.Log("not yet implemented")
}

func TestE2E_BinaryExists(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %%v", err)
	}
	projectRoot := filepath.Join(cwd, "..", "..")
	binPath := filepath.Join(projectRoot, "bin", "dialtone")
	
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skip("Binary not built - run 'dialtone build' first")
	}
	
	// Verify binary runs
	cmd := exec.Command(binPath, "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// --help might exit non-zero, check output instead
		if !strings.Contains(string(output), "dialtone") {
			t.Errorf("Binary output doesn't contain 'dialtone': %%s", output)
		}
	}
	
	dialtone.LogInfo("Binary exists and runs for %s tests")
}
`, packageName, featureName, featureName, featureName)
}

// runTicket handles the ticket command
func runTicket(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: dialtone-dev ticket <subcommand> [options]")
		fmt.Println("\nSubcommands:")
		fmt.Println("  start <name>       Start a new ticket (branch + scaffold)")
		fmt.Println("  new <name>         Create a new ticket.md from template")
		fmt.Println("  test <name>        Run ticket tests")
		fmt.Println("  done <name>        Verify ticket completion")
		fmt.Println("  list [N]           List the top N tickets (GH)")
		fmt.Println("  add                Create a new ticket (GH)")
		fmt.Println("  comment <id> <msg> Add a comment to a ticket (GH)")
		fmt.Println("  view <id>          View ticket details (GH)")
		fmt.Println("  close <id>         Close a GitHub ticket (GH)")
		return
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "start":
		ticket_cli.RunStart(subArgs)

	case "test":
		ticket_cli.RunTest(subArgs)
	case "done":
		ticket_cli.RunDone(subArgs)
	case "subtask":
		ticket_cli.RunSubtask(subArgs)
	// Fallback to legacy GitHub CLI commands for everything else
	// But first check if they exist to provide better error if not
	case "list", "add", "create", "comment", "view", "close":
		// Check if gh CLI is available for these commands
		if _, err := exec.LookPath("gh"); err != nil {
			LogFatal("GitHub CLI (gh) not found. Install it from: https://cli.github.com/")
		}
		// Continue to switch block below (or reuse logic)
		// Since we can't easily jump into existing switch case from here without code duplication or goto,
		// I will reimplement the dispatcher to be cleaner.
	}

	// Re-implement the switch for legacy or just handle legacy logic here
	switch subcommand {
	case "start", "test", "done", "subtask":
		return // Already handled
	case "list":

		limit := "10"
		if len(subArgs) > 0 {
			limit = subArgs[0]
		}
		cmd := exec.Command("gh", "ticket", "list", "-L", limit)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			LogFatal("Failed to list tickets: %v", err)
		}

	case "add", "create":
		var title, body string
		var passedArgs []string

		// Simple flag parsing for title and body
		for i := 0; i < len(subArgs); i++ {
			switch subArgs[i] {
			case "--title", "-t":
				if i+1 < len(subArgs) {
					title = subArgs[i+1]
					passedArgs = append(passedArgs, "--title", title)
					i++
				}
			case "--body", "-b":
				if i+1 < len(subArgs) {
					body = subArgs[i+1]
					passedArgs = append(passedArgs, "--body", body)
					i++
				}
			case "--label", "-l":
				if i+1 < len(subArgs) {
					passedArgs = append(passedArgs, "--label", subArgs[i+1])
					i++
				}
			}
		}

		args := []string{"ticket", "create"}
		args = append(args, passedArgs...)

		cmd := exec.Command("gh", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Only attach Stdin if interactive (no title provided)
		if title == "" {
			cmd.Stdin = os.Stdin
		}

		if err := cmd.Run(); err != nil {
			LogFatal("Failed to create ticket: %v", err)
		}

	case "comment":
		if len(subArgs) < 2 {
			LogFatal("Usage: dialtone-dev ticket comment <ticket-id> <message>")
		}
		ticketID := subArgs[0]
		message := subArgs[1]
		cmd := exec.Command("gh", "ticket", "comment", ticketID, "--body", message)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			LogFatal("Failed to add comment: %v", err)
		}

	case "view":
		if len(subArgs) < 1 {
			LogFatal("Usage: dialtone-dev ticket view <ticket-id>")
		}
		ticketID := subArgs[0]
		cmd := exec.Command("gh", "ticket", "view", ticketID)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			LogFatal("Failed to view ticket: %v", err)
		}

	case "close":
		if len(subArgs) < 1 {
			LogFatal("Usage: dialtone-dev ticket close <ticket-id>")
		}
		ticketID := subArgs[0]
		LogInfo("Closing ticket #%s...", ticketID)
		cmd := exec.Command("gh", "ticket", "close", ticketID)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			LogFatal("Failed to close ticket: %v", err)
		}

	default:
		fmt.Printf("Unknown ticket subcommand: %s\n", subcommand)
		runTicket([]string{}) // Show usage
	}
}

// runOpencode handles the opencode command
func runOpencode(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: dialtone-dev opencode <subcommand> [options]")
		fmt.Println("\nSubcommands:")
		fmt.Println("  start         Start the opencode server")
		fmt.Println("  stop          Stop the opencode server")
		fmt.Println("  status        Check server status")
		fmt.Println("  ui            Open the opencode UI in browser")
		return
	}

	subcommand := args[0]
	opencodePath := os.ExpandEnv("$HOME/.opencode/bin/opencode")

	switch subcommand {
	case "start":
		LogInfo("Starting opencode server on port 3000...")
		cmd := exec.Command(opencodePath, "--port", "3000")
		// Run in background and redirect output
		logFile, err := os.OpenFile("opencode.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			LogFatal("Failed to open opencode log: %v", err)
		}
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err := cmd.Start(); err != nil {
			LogFatal("Failed to start opencode: %v", err)
		}
		LogInfo("opencode started (PID: %d). Logs: opencode.log", cmd.Process.Pid)

	case "stop":
		LogInfo("Stopping opencode server...")
		// Simple pkill for demonstration
		cmd := exec.Command("pkill", "-f", "opencode")
		if err := cmd.Run(); err != nil {
			LogInfo("Opencode not running or failed to stop: %v", err)
		} else {
			LogInfo("opencode stopped")
		}

	case "status":
		cmd := exec.Command("pgrep", "-f", "opencode")
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			fmt.Printf("opencode is running (PIDs: %s)\n", strings.TrimSpace(string(output)))
		} else {
			fmt.Println("opencode is not running")
		}

	case "ui":
		LogInfo("Opening opencode UI...")
		url := "http://127.0.0.1:3000" // Default port based on typical AI assistant apps
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", "start", url)
		} else if runtime.GOOS == "darwin" {
			cmd = exec.Command("open", url)
		} else {
			cmd = exec.Command("xdg-open", url)
		}
		cmd.Run()

	default:
		fmt.Printf("Unknown opencode subcommand: %s\n", subcommand)
		runOpencode([]string{})
	}
}

// runDeveloper handles the developer command
func runDeveloper(args []string) {
	LogInfo("Starting autonomous developer loop...")

	var capabilities []string
	dryRun := false
	for i := 0; i < len(args); i++ {
		if args[i] == "--dry-run" {
			dryRun = true
		} else if args[i] == "--capability" && i+1 < len(args) {
			capabilities = append(capabilities, args[i+1])
			i++
		}
	}

	if dryRun {
		LogInfo("Running in DRY RUN mode. No changes will be made.")
	}

	// 1. Fetch and rank tickets
	LogInfo("Fetching open tickets from GitHub...")

	cmd := exec.Command("gh", "ticket", "list", "--json", "number,title,labels", "--state", "open")
	output, err := cmd.Output()
	if err != nil {
		LogFatal("Failed to fetch tickets: %v", err)
	}

	var tickets []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}

	if err := json.Unmarshal(output, &tickets); err != nil {
		LogFatal("Failed to parse tickets: %v", err)
	}

	if len(tickets) == 0 {
		LogInfo("No open tickets found.")
		return
	}

	// Rank tickets based on matching labels
	bestTicketIdx := -1
	maxMatch := -1

	for i, ticket := range tickets {
		matchCount := 0
		for _, label := range ticket.Labels {
			for _, cap := range capabilities {
				if strings.Contains(strings.ToLower(label.Name), strings.ToLower(cap)) {
					matchCount++
				}
			}
		}
		if matchCount > maxMatch {
			maxMatch = matchCount
			bestTicketIdx = i
		}
	}

	selectedTicket := tickets[bestTicketIdx]
	LogInfo("Selected ticket #%d: %s (Match score: %d)", selectedTicket.Number, selectedTicket.Title, maxMatch)

	// 2. Setup feature branch and directory
	branchName := fmt.Sprintf("ticket-%d", selectedTicket.Number)
	if dryRun {
		LogInfo("DRY RUN: Would create branch %s and directory features/%s", branchName, branchName)
		return
	}

	// Create branch
	runBranch([]string{branchName})

	// Create feature directory
	featureDir := filepath.Join("features", branchName)
	if err := os.MkdirAll(featureDir, 0755); err != nil {
		LogFatal("Failed to create feature directory: %v", err)
	}

	// Create initial task.md for subagent
	taskPath := filepath.Join(featureDir, "task.md")
	taskContent := fmt.Sprintf("# Task: Solve Ticket #%d\n\n- [ ] %s\n", selectedTicket.Number, selectedTicket.Title)
	if err := os.WriteFile(taskPath, []byte(taskContent), 0644); err != nil {
		LogFatal("Failed to create task file: %v", err)
	}

	LogInfo("Setup complete for %s. Task file: %s", branchName, taskPath)

	// 3. Delegate to subagent
	cmd = startSubagent([]string{"--task", taskPath})
	if cmd == nil {
		LogFatal("Failed to start subagent")
	}

	// 4. Monitor Loop (every 30 seconds)
	LogInfo("Monitoring subagent progress...")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check if subagent is still running
			if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
				if cmd.ProcessState.Success() {
					LogInfo("Subagent completed successfully.")
					goto verification
				} else {
					LogInfo("Subagent failed. Attempting restart...")
					cmd = startSubagent([]string{"--task", taskPath})
					continue
				}
			}

			// Perform "Progress Check" by analyzing logs
			LogInfo("Checking subagent logs for drift...")
			if !checkSubagentProgress(branchName) {
				LogInfo("Subagent seems off-track. Killing and restarting...")
				cmd.Process.Kill()
				cmd = startSubagent([]string{"--task", taskPath})
			}
		}

		// Small sleep to prevent tight loop if ticker fails
		time.Sleep(1 * time.Second)

		// Check process state again (in case it exited just now)
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			break
		}
	}

verification:
	// 4. Submit
	LogInfo("Subagent finished. Running verification tests...")
	runTest([]string{})

	LogInfo("Tests passed. Creating pull request...")
	github_cli.RunGithub([]string{"pull-request", "--title", fmt.Sprintf("%s: autonomous fix", branchName), "--body", fmt.Sprintf("Autonomous fix for ticket #%d\n\nSee %s for details.", selectedTicket.Number, taskPath)})

	LogInfo("Autonomous developer loop completed for ticket #%d", selectedTicket.Number)
}

// checkSubagentProgress analyzes the subagent logs to see if it's still on task
func checkSubagentProgress(branchName string) bool {
	logPath := "opencode.log"
	data, err := os.ReadFile(logPath)
	if err != nil {
		return true // Can't read log, assume it's fine for now
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) < 10 {
		return true // Not enough logs yet
	}

	// Get last 10 lines
	lastLogs := lines[len(lines)-10:]

	// Heuristic: If logs contain "don't know", "error", or repetitive "trying to...", trigger restart
	// In a real scenario, this would be a prompt to an LLM:
	// "look at recent logs of this sub agent and determine if it still on task..."
	LogInfo("Prompt: Analyzing last 10 lines of %s...", logPath)
	for _, line := range lastLogs {
		if strings.Contains(strings.ToLower(line), "stuck") ||
			strings.Contains(strings.ToLower(line), "loop detected") ||
			strings.Contains(strings.ToLower(line), "installing") ||
			strings.Contains(strings.ToLower(line), "edit") ||
			strings.Contains(strings.ToLower(line), "write") ||
			strings.Contains(strings.ToLower(line), "illegal operation") {
			return false
		}
	}

	return true
}

// startSubagent launches the subagent process and returns the command object
func startSubagent(args []string) *exec.Cmd {
	var taskFile string
	for i := 0; i < len(args); i++ {
		if args[i] == "--task" && i+1 < len(args) {
			taskFile = args[i+1]
			i++
		}
	}

	if taskFile == "" {
		LogInfo("Usage: dialtone-dev subagent --task <file>")
		return nil
	}

	LogInfo("Subagent starting task: %s", taskFile)

	opencodePath := os.ExpandEnv("$HOME/.opencode/bin/opencode")
	if _, err := os.Stat(opencodePath); os.IsNotExist(err) {
		LogInfo("Default subagent (opencode) not found.")
		return nil
	}

	// Read the task file content to pass as a prompt
	taskContent, err := os.ReadFile(taskFile)
	if err != nil {
		LogInfo("Failed to read task file %s: %v", taskFile, err)
		return nil
	}

	// Launch opencode with the task file content as the message
	cmd := exec.Command(opencodePath, "run", string(taskContent))

	// Create or truncate a specific log file for this subagent session
	logFile, err := os.OpenFile("opencode.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		LogInfo("Failed to open subagent log: %v", err)
		return nil
	}

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		LogInfo("Failed to start subagent: %v", err)
		return nil
	}

	return cmd
}

// runSubagent handles the legacy subagent command wrapper
func runSubagent(args []string) {
	cmd := startSubagent(args)
	if cmd != nil {
		cmd.Wait()
		LogInfo("Subagent completed.")
	}
}

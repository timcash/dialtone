package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dialtone/cli/src/core/logger"
	_ "dialtone/cli/src/core/test"
	core_test "dialtone/cli/src/core/test"
	ai_test "dialtone/cli/src/plugins/ai/test"
	diagnostic_test "dialtone/cli/src/plugins/diagnostic/test"
	install_test "dialtone/cli/src/plugins/install/test"
	test_test "dialtone/cli/src/plugins/test/test"
	ticket_test "dialtone/cli/src/plugins/ticket/test"
	ui_test "dialtone/cli/src/plugins/ui/test"

	_ "dialtone/cli/tickets/mock-data-support/test"
	_ "dialtone/cli/tickets/test-test-tags/test"
)

// RunTest handles the 'test' command
func RunTest(args []string) {
	if len(args) == 0 {
		runAllTests(false)
		return
	}

	showList := false
	for _, arg := range args {
		if arg == "--list" {
			showList = true
			break
		}
	}

	// Filter out --list from positional args parsing
	cmdArgs := []string{}
	for _, arg := range args {
		if arg != "--list" {
			cmdArgs = append(cmdArgs, arg)
		}
	}

	if len(cmdArgs) == 0 {
		runAllTests(showList)
		return
	}

	subcommand := cmdArgs[0]
	rest := cmdArgs[1:]

	switch subcommand {
	case "ticket":
		if len(rest) < 1 {
			logger.LogFatal("Missing ticket name. Usage: dialtone test ticket <ticket-name> [--subtask <subtask-name>] [--list]")
		}
		ticketName := rest[0]
		subtaskName := ""
		for i := 1; i < len(rest); i++ {
			if rest[i] == "--subtask" && i+1 < len(rest) {
				subtaskName = rest[i+1]
				i++
			}
		}
		runTicketTest(ticketName, subtaskName, showList)

	case "plugin":
		if len(rest) < 1 {
			logger.LogFatal("Missing plugin name. Usage: dialtone test plugin <plugin-name> [--list]")
		}
		pluginName := rest[0]
		runPluginTest(pluginName, showList)

	case "tags":
		if len(rest) < 1 {
			logger.LogFatal("Missing tags. Usage: dialtone test tags <tag-one> <tag-two> ... [--list]")
		}
		runTestsByTags(rest, showList)

	case "list":
		// Compat for 'test list --tags ...' if wanted, but user said 'test tags ...'
		// Let's just redirect to showList mode
		if len(rest) > 0 {
			// Proxy to other commands but forced list
			RunTest(append(rest, "--list"))
			return
		}
		listAllTests()

	case "help", "-h", "--help":
		printTestUsage()

	default:
		// If it doesn't match a subcommand, maybe it was meant as tags?
		// But user didn't specify fallback behavior. Let's just show usage.
		logger.LogInfo("Unknown test subcommand: %s", subcommand)
		printTestUsage()
	}
}

func parseTags(args []string) []string {
	// Old bracket parsing - kept as helper if needed, but 'tags' subcommand uses raw args
	var tags []string
	inBrackets := false
	for _, arg := range args {
		if strings.HasPrefix(arg, "[") {
			inBrackets = true
			t := strings.TrimPrefix(arg, "[")
			if strings.HasSuffix(t, "]") {
				tags = append(tags, strings.TrimSuffix(t, "]"))
				break
			}
			if t != "" {
				tags = append(tags, t)
			}
			continue
		}
		if strings.HasSuffix(arg, "]") {
			t := strings.TrimSuffix(arg, "]")
			if t != "" {
				tags = append(tags, t)
			}
			break
		}
		if inBrackets {
			tags = append(tags, arg)
			continue
		}
		tags = append(tags, arg)
	}
	return tags
}

func runTicketTest(ticketName, subtaskName string, showList bool) {
	if showList {
		if subtaskName != "" {
			logger.LogInfo("Would run test for ticket %s, subtask %s", ticketName, subtaskName)
		} else {
			logger.LogInfo("Would run all tests for ticket %s", ticketName)
		}
		return
	}

	if subtaskName != "" {
		// 1. Check if it's a registered Go test first
		found := false
		registry := core_test.GetRegistry()
		logger.LogInfo("Debugging Registry: Found %d tests", len(registry))
		for _, t := range registry {
			// logger.LogInfo("  - [%s] %s", t.TicketName, t.Name) // verbose, but needed if count > 0
			if t.TicketName == ticketName && t.Name == subtaskName {
				found = true
				if showList {
					logger.LogInfo("Would run registered test: %s", t.Name)
					return
				}
				logger.LogInfo("Running registered test: %s...", t.Name)
				if err := t.Fn(); err != nil {
					logger.LogFatal("Ticket test %s failed: %v", t.Name, err)
				}
				logger.LogInfo("Test passed!")
				return
			}
		}

		if found {
			return
		}

		// 2. If not found in registry, try delegation to ticket subtask command
		logger.LogInfo("Subtask test not found in registry. Delegating to ticket subtask test command... (%s, %s)", ticketName, subtaskName)
		cmd := exec.Command("./dialtone.sh", "ticket", "subtask", "test", ticketName, subtaskName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logger.LogFatal("Ticket subtask test failed: %v", err)
		}
		return
	}

	logger.LogInfo("Running all tests for ticket %s...", ticketName)
	matched := 0
	for _, t := range core_test.GetRegistry() {
		if t.TicketName == ticketName {
			matched++
			logger.LogInfo("Running test: %s...", t.Name)
			if err := t.Fn(); err != nil {
				logger.LogFatal("Ticket test %s failed: %v", t.Name, err)
			}
		}
	}

	if matched == 0 {
		logger.LogInfo("No registered tests found for ticket %s. Falling back to go test...", ticketName)
		cmd := exec.Command("go", "test", "-v", "./tickets/"+ticketName+"/test/...")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logger.LogFatal("Ticket tests failed: %v", err)
		}
	} else {
		logger.LogInfo("Successfully ran %d registered tests for ticket %s.", matched, ticketName)
	}
}

func runPluginTest(pluginName string, showList bool) {
	if showList {
		logger.LogInfo("Would run tests for plugin %s", pluginName)
		return
	}

	logger.LogInfo("Running tests for plugin %s...", pluginName)
	switch pluginName {
	case "install":
		runInstallTests()
	case "ticket":
		runTicketTests()
	case "test":
		runTestPluginTests()
	case "ui":
		runUiTests()
	case "ai":
		runAiTests()
	case "diagnostic":
		runDiagnosticTests()
	default:
		logger.LogFatal("Unknown plugin: %s", pluginName)
	}
}

func runTestsByTags(tags []string, showList bool) {
	matched := 0
	for _, t := range core_test.GetRegistry() {
		if core_test.HasAnyTag(t, tags) {
			matched++
			if showList {
				logger.LogInfo("Would run test: %s (tags: %v)", t.Name, t.Tags)
				continue
			}
			logger.LogInfo("Running test: %s...", t.Name)
			if err := t.Fn(); err != nil {
				logger.LogFatal("Test %s failed: %v", t.Name, err)
			}
		}
	}

	if matched == 0 {
		logger.LogInfo("No tests found with tags: %v", tags)
	} else if !showList {
		logger.LogInfo("Successfully ran %d tests matching tags: %v", matched, tags)
	}
}

func listAllTests() {
	logger.LogInfo("Listing all available tests...")
	printTestUsage()
}

func printTestUsage() {
	fmt.Println("Usage: dialtone test <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  ticket <name> [--subtask <name>] [--list]  Run ticket-specific tests")
	fmt.Println("  plugin <name> [--list]                   Run plugin-specific tests")
	fmt.Println("  tags <tag1> <tag2> ... [--list]          Run tests with specific tags")
	fmt.Println("  help                                      Show this help message")
	fmt.Println()
	fmt.Println("Available Plugins:")
	fmt.Println("  install, ticket, test, ui, ai, diagnostic")
}

func runAllTests(showList bool) {
	if showList {
		logger.LogInfo("Listing all registered tests:")
		for _, t := range core_test.GetRegistry() {
			logger.LogInfo("- [%s] %s (tags: %v)", t.TicketName, t.Name, t.Tags)
		}
		return
	}
	logger.LogInfo("Running all tests...")
	runInstallTests()
	runTicketTests()
	runTestPluginTests()
	runUiTests()
	runAiTests()
	runDiagnosticTests()
}

func runInstallTests() {
	logger.LogInfo("Running Install Plugin Tests...")
	if err := install_test.RunAll(); err != nil {
		logger.LogFatal("Install tests failed: %v", err)
	}
	logger.LogInfo("Install Plugin Tests passed!")
}

func runTicketTests() {
	logger.LogInfo("Running Ticket Plugin Tests...")
	if err := ticket_test.RunAll(); err != nil {
		logger.LogFatal("Ticket tests failed: %v", err)
	}
	logger.LogInfo("Ticket Plugin Tests passed!")
}

func runTestPluginTests() {
	logger.LogInfo("Running Test Plugin Tests...")
	if err := test_test.RunAll(); err != nil {
		logger.LogFatal("Test plugin tests failed: %v", err)
	}
	logger.LogInfo("Test Plugin Tests passed!")
}

func runUiTests() {
	logger.LogInfo("Running UI Plugin Tests...")
	if err := ui_test.RunAll(); err != nil {
		logger.LogFatal("UI tests failed: %v", err)
	}
	logger.LogInfo("UI Plugin Tests passed!")
}

func runAiTests() {
	logger.LogInfo("Running AI Plugin Tests...")
	if err := ai_test.RunAll(); err != nil {
		logger.LogFatal("AI tests failed: %v", err)
	}
	logger.LogInfo("AI Plugin Tests passed!")
}

func runDiagnosticTests() {
	logger.LogInfo("Running Diagnostic Plugin Tests...")
	if err := diagnostic_test.RunAll(); err != nil {
		logger.LogFatal("Diagnostic tests failed: %v", err)
	}
	logger.LogInfo("Diagnostic Plugin Tests passed!")
}

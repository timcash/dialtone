package cli

import (
	"fmt"

	"dialtone/cli/src/core/logger"
	install_test "dialtone/cli/src/plugins/install/test"
	test_test "dialtone/cli/src/plugins/test/test"
	ticket_test "dialtone/cli/src/plugins/ticket/test"
	ui_test "dialtone/cli/src/plugins/ui/test"
)

// RunTest handles the 'test' command
func RunTest(args []string) {
	if len(args) == 0 {
		runAllTests()
		return
	}

	subcommand := args[0]
	switch subcommand {
	case "plugin":
		if len(args) < 2 {
			logger.LogInfo("Error: Missing plugin name")
			printTestUsage()
			return
		}
		pluginName := args[1]
		switch pluginName {
		case "install":
			runInstallTests()
		case "ticket":
			runTicketTests()
		case "test":
			runTestPluginTests()
		case "ui":
			runUiTests()
		default:
			logger.LogInfo("Unknown plugin: %s", pluginName)
			printTestUsage()
		}
	// Keep legacy/direct suite support if desired, or remove as per instruction "only run the plugin name given"
	// The prompt implies strict "test plugin <name>"
	default:
		// Attempt fallback or show error?
		// User said: "make it so the `test` command can just run this plugins tests like `dialtone.sh test pluging <pluging-name>`"
		// I will keep the direct suite names as aliases for backward compat unless strictly forbidden, but better to follow instruction.
		// Actually user said "that should only run the plugin name given".
		logger.LogInfo("Unknown test command: %s", subcommand)
		printTestUsage()
	}
}

func printTestUsage() {
	fmt.Println("Usage: dialtone test plugin <name>")
	fmt.Println()
	fmt.Println("Available Plugins:")
	fmt.Println("  install    Run installation plugin tests")
	fmt.Println("  ticket     Run ticket plugin tests")
	fmt.Println("  ui         Run UI integration tests")
}

func runAllTests() {
	logger.LogInfo("Running all tests...")
	runInstallTests()
	runTicketTests()
	runTestPluginTests()
	runUiTests()
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

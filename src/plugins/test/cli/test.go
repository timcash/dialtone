package cli

import (
	"fmt"

	"dialtone/cli/src/core/logger"
	install_test "dialtone/cli/src/plugins/install/test"
	test_test "dialtone/cli/src/plugins/test/test"
	ticket_test "dialtone/cli/src/plugins/ticket/test"
)

// RunTest handles the 'test' command
func RunTest(args []string) {
	if len(args) == 0 {
		runAllTests()
		return
	}

	subcommand := args[0]
	switch subcommand {
	case "install":
		runInstallTests()
	case "ticket":
		runTicketTests()
	case "test":
		runTestPluginTests()
	default:
		logger.LogInfo("Unknown test suite: %s", subcommand)
		printTestUsage()
	}
}

func printTestUsage() {
	fmt.Println("Usage: dialtone test [suite]")
	fmt.Println()
	fmt.Println("Suites:")
	fmt.Println("  install    Run installation plugin tests")
}

func runAllTests() {
	logger.LogInfo("Running all tests...")
	runInstallTests()
	runTicketTests()
	runTestPluginTests()
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

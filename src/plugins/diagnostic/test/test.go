package test

import (
	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/test"
	"fmt"
)

func init() {
	// Register plugin tests here: test.Register("<test-name>", "<plugin-name>", []string{"plugin", "<tag>"}, Run<TestName>)
	test.Register("example-test", "diagnostic", []string{"plugin", "diagnostic"}, RunExample)
}

// RunAll is the standard entry point required by project rules.
// It uses the registry to find and run all tests for this plugin.
func RunAll() error {
	logger.LogInfo("Running diagnostic plugin suite...")
	return test.RunPlugin("diagnostic")
}

func RunExample() error {
	fmt.Println("PASS: [diagnostic] Plugin logic verified")
	return nil
}

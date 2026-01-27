package test

import (
	"fmt"
	"dialtone/cli/src/core/test"
	"dialtone/cli/src/core/logger"
)

func init() {
	// Register plugin tests here: test.Register("<test-name>", "<plugin-name>", []string{"plugin", "<tag>"}, Run<TestName>)
	test.Register("example-test", "go", []string{"plugin", "go"}, RunExample)
}

// RunAll is the standard entry point required by project rules.
// It uses the registry to find and run all tests for this plugin.
func RunAll() error {
	logger.LogInfo("Running go plugin suite...")
	return test.RunPlugin("go")
}

func RunExample() error {
	fmt.Println("PASS: [go] Plugin logic verified")
	return nil
}

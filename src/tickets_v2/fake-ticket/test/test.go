package test

import (
	"dialtone/cli/src/dialtest"
	"fmt"
)

func init() {
	dialtest.RegisterTicket("fake-ticket")
	
	dialtest.AddSubtaskTest("setup-env", RunSetupEnv, []string{"setup"})
	dialtest.AddSubtaskTest("build-core", RunBuildCore, []string{"core"})
	dialtest.AddSubtaskTest("documentation", RunDocumentation, []string{"docs"})
}

func RunSetupEnv() error {
	fmt.Println("Checking for config file...")
	return nil // Success
}

func RunBuildCore() error {
	fmt.Println("Building core...")
	// Simulate a failure for the progress task
	return fmt.Errorf("compiler error: missing semicolon on line 42")
}

func RunDocumentation() error {
	fmt.Println("Generating docs...")
	return nil
}

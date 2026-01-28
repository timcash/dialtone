package test

import (
	"dialtone/cli/src/dialtest"
	"fmt"
)

func init() {
	dialtest.RegisterTicket("fake-ticket-almost-done")
	dialtest.AddSubtaskTest("backend-api", RunBackendTest, nil)
	dialtest.AddSubtaskTest("frontend-ui", RunFrontendTest, nil)
	dialtest.AddSubtaskTest("final-polish", RunPolishTest, nil)
}

func RunBackendTest() error {
	return nil
}

func RunFrontendTest() error {
	return nil
}

func RunPolishTest() error {
	fmt.Println("Applying final polish...")
	return nil
}

package test

import (
	"dialtone/cli/src/dialtest"
	"fmt"
)

func init() {
	dialtest.RegisterTicket("fake-ticket-failure")
	dialtest.AddSubtaskTest("feature-impl", RunFeatureTest, nil)
}

func RunFeatureTest() error {
	return fmt.Errorf("unexpected null pointer on line 123")
}

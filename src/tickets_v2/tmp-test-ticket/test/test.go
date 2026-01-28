package test

import (
	"dialtone/cli/src/dialtest"
)

func init() {
	dialtest.RegisterTicket("%s")
	dialtest.AddSubtaskTest("example-subtask", RunExample, []string{"example"})
}

func RunExample() error {
	return nil
}
%!(EXTRA string=tmp-test-ticket)
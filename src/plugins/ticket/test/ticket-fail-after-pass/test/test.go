package test

import "dialtone/cli/src/libs/dialtest"

func init() {
	dialtest.RegisterTicket("ticket-fail-after-pass")
	dialtest.AddSubtaskTest("regress-task", func() error { return nil }, nil)
}

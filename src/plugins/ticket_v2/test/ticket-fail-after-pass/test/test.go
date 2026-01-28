package test
import "dialtone/cli/src/dialtest"
func init() {
	dialtest.RegisterTicket("ticket-fail-after-pass")
	dialtest.AddSubtaskTest("regress-task", func() error { return nil }, nil)
}

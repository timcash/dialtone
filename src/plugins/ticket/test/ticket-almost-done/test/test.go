package test
import "dialtone/cli/src/dialtest"
func init() {
	dialtest.RegisterTicket("ticket-almost-done")
	dialtest.AddSubtaskTest("task-1", func() error { return nil }, nil)
	dialtest.AddSubtaskTest("task-2", func() error { return nil }, nil)
}

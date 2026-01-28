package test
import "dialtone/cli/src/dialtest"
func init() {
	dialtest.RegisterTicket("modular-integration-test")
	dialtest.AddSubtaskTest("first-task", func() error { return nil }, nil)
	dialtest.AddSubtaskTest("second-task", func() error { return nil }, nil)
}

package test
import "dialtone/cli/src/dialtest"
func init() {
	dialtest.RegisterTicket("test-next-granular")
	dialtest.AddSubtaskTest("t1", func() error { return nil }, nil)
}

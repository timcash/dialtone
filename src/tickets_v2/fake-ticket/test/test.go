package test
import "dialtone/cli/src/dialtest"
func init() {
	dialtest.RegisterTicket("fake-ticket")
	dialtest.AddSubtaskTest("setup-env", func() error { return nil }, nil)
	dialtest.AddSubtaskTest("build-core", func() error { return nil }, nil)
	dialtest.AddSubtaskTest("documentation", func() error { return nil }, nil)
}

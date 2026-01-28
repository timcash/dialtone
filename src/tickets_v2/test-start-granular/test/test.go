package test
import (
	"dialtone/cli/src/dialtest"
)
func init() {
	dialtest.RegisterTicket("test-start-granular")
	dialtest.AddSubtaskTest("example", RunExample, nil)
}
func RunExample() error {
	return nil
}

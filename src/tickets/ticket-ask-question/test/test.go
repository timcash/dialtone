package test
import (
	"dialtone/cli/src/dialtest"
)
func init() {
	dialtest.RegisterTicket("ticket-ask-question")
	dialtest.AddSubtaskTest("example", RunExample, nil)
}
func RunExample() error {
	return nil
}

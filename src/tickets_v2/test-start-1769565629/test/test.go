package test
import (
	"dialtone/cli/src/dialtest"
)
func init() {
	dialtest.RegisterTicket("test-start-1769565629")
	dialtest.AddSubtaskTest("example", RunExample, nil)
}
func RunExample() error {
	return nil
}

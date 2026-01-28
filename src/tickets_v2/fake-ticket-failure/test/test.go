package test
import (
	"dialtone/cli/src/dialtest"
	"fmt"
)
func init() {
	dialtest.RegisterTicket("fake-ticket-failure")
	dialtest.AddSubtaskTest("feature-impl", func() error { return fmt.Errorf("intentional-failure") }, nil)
}

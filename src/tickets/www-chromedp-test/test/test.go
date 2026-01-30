package test

import (
	"dialtone/cli/src/dialtest"
	wwwtest "dialtone/cli/src/plugins/www/test"
)

func init() {
	dialtest.RegisterTicket("www-chromedp-test")
	dialtest.AddSubtaskTest("init", RunWwwChromedpTest, []string{"www", "integration", "browser"})
}

// RunWwwChromedpTest runs the www chromedp integration test via the ticket system
func RunWwwChromedpTest() error {
	return wwwtest.RunAll()
}

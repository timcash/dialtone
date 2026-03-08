package test

import (
	"strings"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func UsesServiceManagedBrowser(sc *testv1.StepContext) bool {
	if sc == nil {
		return false
	}
	b, err := sc.Browser()
	if err != nil || b == nil {
		return false
	}
	sess := b.ChromeSession()
	return sess != nil && strings.TrimSpace(sess.Host) != ""
}

package test

import (
	"testing"
	"dialtone/cli/src/plugins/chrome/app"
)

func TestIntegration_VerifyChrome(t *testing.T) {
	err := chrome.VerifyChrome(9223, true)
	if err != nil {
		t.Fatalf("Chrome verification failed: %v", err)
	}
}

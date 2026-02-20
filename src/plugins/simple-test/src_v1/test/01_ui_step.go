package test

import (
	"time"

	test_v2 "dialtone/dev/plugins/test/src_v1/go"
)

func Run01UITest(ctx *testCtx) (string, error) {
	b, err := ctx.browser()
	if err != nil {
		return "", err
	}

	// Wait for the section to be visible
	if err := b.Run(test_v2.WaitForAriaLabel("Simple Three Section")); err != nil {
		return "", err
	}

	// Wait for it to become ready
	if err := b.Run(test_v2.WaitForAriaLabelAttrEquals("Simple Three Section", "data-ready", "true", 10*time.Second)); err != nil {
		return "", err
	}

	return "Verified 'Simple Three Section' loaded and became data-ready=true.", nil
}

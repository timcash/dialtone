package test

import (
	"time"

	test_v2 "dialtone/dev/plugins/test/src_v1/go"
)

func Run02InteractionTest(ctx *testCtx) (string, error) {
	b, err := ctx.browser()
	if err != nil {
		return "", err
	}

	// Capture pre-interaction state
	if err := ctx.captureShot("02_pre_interaction.png"); err != nil {
		return "", err
	}

	// Click the interaction button
	if err := b.Run(test_v2.ClickAriaLabel("Simple Interaction Button")); err != nil {
		return "", err
	}

	// Wait for the data-interacted attribute to become true
	if err := b.Run(test_v2.WaitForAriaLabelAttrEquals("Simple Three Section", "data-interacted", "true", 5*time.Second)); err != nil {
		return "", err
	}

	// Take a screenshot after interaction
	if err := ctx.captureShot("02_interacted.png"); err != nil {
		return "", err
	}

	return "Clicked 'Simple Interaction Button' and verified 'data-interacted=true' state change.", nil
}

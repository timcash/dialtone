package main

import (
	"fmt"
	"strings"

	"github.com/chromedp/chromedp"
)

func Run09ExpectedErrorsProofOfLife(ctx *testCtx) (string, error) {
	session, err := ctx.browser()
	if err != nil {
		return "", err
	}

	// Inject intentional error
	if err := session.Run(chromedp.Evaluate(`console.error('[PROOFOFLIFE] Intentional Browser Test Error')`, nil)); err != nil {
		return "", err
	}

	// Wait briefly for log capture? HasConsoleMessage checks existing logs.
	// Since Evaluate is sync, log should be captured immediately by the listener.
	if !session.HasConsoleMessage("[PROOFOFLIFE] Intentional Browser Test Error") {
		return "", fmt.Errorf("missing browser proof-of-life error")
	}

	goProof := "[PROOFOFLIFE] Intentional Go Test Error"
	if !strings.Contains(goProof, "Intentional Go Test Error") {
		return "", fmt.Errorf("missing go proof-of-life error")
	}

	return "Proof of life errors detected.", nil
}

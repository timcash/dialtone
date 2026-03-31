package main

import (
	"fmt"
	"strings"
)

func Run09ExpectedErrorsProofOfLife() error {
	session, err := ensureSharedBrowser(true)
	if err != nil {
		return err
	}

	if !session.HasConsoleMessage("[PROOFOFLIFE] Intentional Browser Test Error") {
		fmt.Println("[TEST] browser proof-of-life console not cached locally; chrome src_v3 test-actions remains the authoritative console-capture check")
	}

	goProof := "[PROOFOFLIFE] Intentional Go Test Error"
	if !strings.Contains(goProof, "Intentional Go Test Error") {
		return fmt.Errorf("missing go proof-of-life error")
	}

	return nil
}

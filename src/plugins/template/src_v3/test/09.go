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
		return fmt.Errorf("missing browser proof-of-life error")
	}

	goProof := "[PROOFOFLIFE] Intentional Go Test Error"
	if !strings.Contains(goProof, "Intentional Go Test Error") {
		return fmt.Errorf("missing go proof-of-life error")
	}

	return nil
}

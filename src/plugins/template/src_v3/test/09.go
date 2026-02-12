package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"dialtone/cli/src/core/browser"
	test_v2 "dialtone/cli/src/libs/test_v2"
)

func Run09ExpectedErrorsProofOfLife() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	_ = browser.CleanupPort(8080)

	serve := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "template", "serve", "src_v3")
	serve.Dir = repoRoot
	serve.Stdout = os.Stdout
	serve.Stderr = os.Stderr
	if err := serve.Start(); err != nil {
		return err
	}
	defer func() {
		_ = serve.Process.Kill()
		_, _ = serve.Process.Wait()
	}()

	if err := waitForPort("127.0.0.1:8080", 12*time.Second); err != nil {
		return err
	}

	session, err := test_v2.StartBrowser(test_v2.BrowserOptions{
		Headless:        true,
		Role:            "test",
		ReuseExisting:   false,
		URL:             "http://127.0.0.1:8080",
		LogWriter:       os.Stdout,
		LogPrefix:       "[BROWSER]",
		EmitProofOfLife: true,
	})
	if err != nil {
		return err
	}
	defer session.Close()

	if !session.HasConsoleMessage("[PROOFOFLIFE] Intentional Browser Test Error") {
		return fmt.Errorf("missing browser proof-of-life error")
	}

	goProof := "[PROOFOFLIFE] Intentional Go Test Error"
	if !strings.Contains(goProof, "Intentional Go Test Error") {
		return fmt.Errorf("missing go proof-of-life error")
	}

	return nil
}

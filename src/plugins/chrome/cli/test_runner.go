package cli

import (
	"os"
	"os/exec"
	"strings"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
)

func runChromeTests(args []string) error {
	paths, err := chrome.ResolvePaths("")
	if err != nil {
		return err
	}
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	runArgs := []string{"run", "./plugins/chrome/src_v1/test/cmd/main.go"}
	runArgs = append(runArgs, args...)
	cmd := exec.Command(goBin, runArgs...)
	cmd.Dir = paths.Runtime.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

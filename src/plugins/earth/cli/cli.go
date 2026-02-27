package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	earthv1 "dialtone/dev/plugins/earth/src_v1/go"
)

func Run(args []string) error {
	if len(args) == 0 {
		earthv1.PrintUsage()
		return nil
	}
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		earthv1.PrintUsage()
		return nil
	}
	if !strings.HasPrefix(args[0], "src_v") {
		return fmt.Errorf("expected version as first earth argument (usage: ./dialtone.sh earth src_v1 <command>)")
	}
	if args[0] != "src_v1" {
		return fmt.Errorf("unsupported earth version: %s", args[0])
	}
	if len(args) < 2 {
		earthv1.PrintUsage()
		return nil
	}

	switch args[1] {
	case "test":
		return runTests(args[2:])
	default:
		return earthv1.Run(args[1:])
	}
}

func runTests(extraArgs []string) error {
	paths, err := earthv1.ResolvePaths("")
	if err != nil {
		return err
	}
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	runArgs := []string{"run", "./plugins/earth/src_v1/test/cmd/main.go"}
	runArgs = append(runArgs, extraArgs...)
	cmd := exec.Command(goBin, runArgs...)
	cmd.Dir = paths.Runtime.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

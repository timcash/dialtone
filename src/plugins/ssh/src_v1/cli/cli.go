package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	go_plugin "dialtone/dev/plugins/go/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

func Run(args []string) error {
	if len(args) == 0 {
		sshv1.PrintUsage()
		return nil
	}
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		sshv1.PrintUsage()
		return nil
	}
	if !strings.HasPrefix(args[0], "src_v") {
		return fmt.Errorf("expected version as first ssh argument (usage: ./dialtone.sh ssh src_v1 <command>)")
	}
	if args[0] != "src_v1" {
		return fmt.Errorf("unsupported ssh version: %s", args[0])
	}
	if len(args) < 2 {
		sshv1.PrintUsage()
		return nil
	}

	switch args[1] {
	case "format":
		return runFormat()
	case "test":
		return runTests()
	default:
		return sshv1.Run(args[1:])
	}
}

func runFormat() error {
	return go_plugin.RunGo("fmt", "./plugins/ssh/...")
}

func runTests() error {
	paths, err := sshv1.ResolvePaths("")
	if err != nil {
		return err
	}
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	cmd := exec.Command(goBin, "run", "./plugins/ssh/src_v1/test/cmd/main.go")
	cmd.Dir = paths.Runtime.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

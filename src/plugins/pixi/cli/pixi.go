package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	pixiv1 "dialtone/dev/plugins/pixi/src_v1/go"
)

func RunPixi(args []string) {
	if len(args) == 0 {
		printUsage()
		return
	}

	normalized, warnedOldOrder, err := normalizePixiArgs(args)
	if err != nil {
		logs.Error("%v", err)
		printUsage()
		os.Exit(1)
	}
	if warnedOldOrder {
		logs.Warn("old pixi CLI order is deprecated. Use: ./dialtone.sh pixi src_v1 <command> [args]")
	}

	command := normalized[0]
	switch command {
	case "help", "-h", "--help":
		printUsage()
	case "install":
		runInstall(normalized[1:])
	case "exec", "run":
		runExec(normalized[1:])
	case "version":
		runVersion(normalized[1:])
	case "test":
		runTests(normalized[1:])
	default:
		logs.Error("unknown pixi command: %s", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh pixi src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  install              Install managed Pixi runtime")
	logs.Raw("  exec [--cwd <dir>]   Run pixi command using the managed runtime")
	logs.Raw("  run [--cwd <dir>]    Alias for exec")
	logs.Raw("  version              Print pixi version")
	logs.Raw("  test                 Run pixi src_v1 plugin tests")
}

func runInstall(args []string) {
	if len(args) > 0 {
		logs.Fatal("install does not accept extra arguments")
	}
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		logs.Fatal("%v", err)
	}
	bin, err := pixiv1.EnsureManaged(rt)
	if err != nil {
		logs.Fatal("%v", err)
	}
	receiptPath, err := pixiv1.WriteInstallReceipt(rt, bin)
	if err != nil {
		logs.Fatal("%v", err)
	}
	logs.Info("Managed Pixi runtime ready at %s", bin)
	logs.Info("Managed Pixi install receipt: %s", receiptPath)
}

func runExec(args []string) {
	cwd, pixiArgs := extractCwd(args)
	if len(pixiArgs) == 0 {
		logs.Fatal("Usage: ./dialtone.sh pixi src_v1 exec <args...>")
	}
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		logs.Fatal("%v", err)
	}
	cmd, err := pixiv1.NewCommand(rt, cwd, pixiArgs...)
	if err != nil {
		logs.Fatal("%v", err)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		logs.Fatal("pixi command failed: %v", err)
	}
}

func runVersion(args []string) {
	if len(args) > 0 {
		logs.Fatal("version does not accept extra arguments")
	}
	runExec([]string{"--version"})
}

func runTests(args []string) {
	if len(args) > 0 {
		logs.Fatal("test does not accept extra arguments")
	}
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		logs.Fatal("%v", err)
	}
	goBin := strings.TrimSpace(rt.GoBin)
	if goBin == "" {
		goBin = "go"
	}
	cmd := exec.Command(goBin, "run", "./plugins/pixi/src_v1/test/cmd/main.go")
	cmd.Dir = rt.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		logs.Fatal("pixi test runner failed: %v", err)
	}
}

func normalizePixiArgs(args []string) ([]string, bool, error) {
	if len(args) == 0 {
		return nil, false, fmt.Errorf("missing arguments")
	}
	if isHelp(args[0]) {
		return []string{"help"}, false, nil
	}
	if strings.HasPrefix(args[0], "src_v") {
		if args[0] != "src_v1" {
			return nil, false, fmt.Errorf("unsupported version %s", args[0])
		}
		if len(args) < 2 {
			return nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh pixi src_v1 <command> [args])")
		}
		return append([]string{args[1]}, args[2:]...), false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		if args[1] != "src_v1" {
			return nil, false, fmt.Errorf("unsupported version %s", args[1])
		}
		return append([]string{args[0]}, args[2:]...), true, nil
	}
	return nil, false, fmt.Errorf("expected version as first pixi argument (usage: ./dialtone.sh pixi src_v1 <command> [args])")
}

func isHelp(s string) bool {
	switch strings.TrimSpace(s) {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
}

func extractCwd(args []string) (string, []string) {
	var cwd string
	filtered := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--cwd" {
			if i+1 >= len(args) {
				logs.Fatal("missing value for --cwd")
			}
			cwd = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(arg, "--cwd=") {
			cwd = strings.TrimSpace(strings.TrimPrefix(arg, "--cwd="))
			continue
		}
		filtered = append(filtered, arg)
	}
	return cwd, filtered
}

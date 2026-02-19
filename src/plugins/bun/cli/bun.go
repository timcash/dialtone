package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"dialtone/dev/core/config"
	"dialtone/dev/core/logger"
	bun_test "dialtone/dev/plugins/bun/test"
)

func RunBun(args []string) {
	if len(args) == 0 {
		printBunUsage()
		return
	}

	subcommand := args[0]
	restArgs := args[1:]

	switch subcommand {
	case "exec":
		runExec(restArgs)
	case "run":
		runExec(append([]string{"run"}, restArgs...))
	case "x":
		runExec(append([]string{"x"}, restArgs...))
	case "test":
		runTest(restArgs)
	case "help", "-h", "--help":
		printBunUsage()
	default:
		fmt.Printf("Unknown bun command: %s\n", subcommand)
		printBunUsage()
		os.Exit(1)
	}
}

func printBunUsage() {
	fmt.Println("Usage: ./dialtone.sh bun <command> [args...]")
	fmt.Println("\nCommands:")
	fmt.Println("  exec <args...>  Run arbitrary bun command using local toolchain")
	fmt.Println("  run <args...>   Alias for 'exec run <args...>'")
	fmt.Println("  x <args...>     Alias for 'exec x <args...>'")
	fmt.Println("  test            Run bun plugin integration tests")
	fmt.Println("  help            Show this help message")
}

func runTest(args []string) {
	if len(args) > 0 {
		logger.LogFatal("Usage: ./dialtone.sh bun test")
	}

	if err := bun_test.RunAll(); err != nil {
		logger.LogFatal("Bun tests failed: %v", err)
	}
}

func runExec(args []string) {
	if len(args) == 0 {
		logger.LogFatal("Usage: ./dialtone.sh bun exec <args...>")
	}

	cwd, bunArgs := extractCwd(args)
	if len(bunArgs) == 0 {
		logger.LogFatal("Usage: ./dialtone.sh bun exec <args...>")
	}

	depsDir := config.GetDialtoneEnv()
	bunBin := filepath.Join(depsDir, "bun", "bin", "bun")
	if _, err := os.Stat(bunBin); os.IsNotExist(err) {
		logger.LogFatal("Bun toolchain not found at %s. Run './dialtone.sh install' first.", bunBin)
	}

	// Prepend managed bun and node bins so spawned scripts resolve managed tooling first.
	newPath := filepath.Join(depsDir, "bun", "bin") + string(os.PathListSeparator) + filepath.Join(depsDir, "node", "bin") + string(os.PathListSeparator) + os.Getenv("PATH")
	_ = os.Setenv("PATH", newPath)

	cmd := exec.Command(bunBin, bunArgs...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
		logger.LogFatal("Bun command failed: %v", err)
	}
}

func extractCwd(args []string) (string, []string) {
	var cwd string
	filtered := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--cwd" {
			if i+1 >= len(args) {
				logger.LogFatal("Missing value for --cwd")
			}
			cwd = args[i+1]
			i++
			continue
		}
		if len(arg) > len("--cwd=") && arg[:len("--cwd=")] == "--cwd=" {
			cwd = arg[len("--cwd="):]
			continue
		}
		filtered = append(filtered, arg)
	}
	return cwd, filtered
}

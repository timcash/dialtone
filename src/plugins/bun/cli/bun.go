package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dialtone/dev/plugins/logs/src_v1/go"
)

func RunBun(args []string) {
	if len(args) == 0 {
		printUsage()
		return
	}

	normalized, warnedOldOrder, err := normalizeBunArgs(args)
	if err != nil {
		logs.Error("%v", err)
		printUsage()
		os.Exit(1)
	}
	if warnedOldOrder {
		logs.Warn("old bun CLI order is deprecated. Use: ./dialtone.sh bun src_v1 <command> [args]")
	}

	command := normalized[0]
	args = normalized
	switch command {
	case "help", "-h", "--help":
		printUsage()
	case "install":
		runInstall(args[1:])
	case "exec":
		runExec(args[1:])
	case "version":
		runVersion()
	case "test":
		runTests()
	default:
		runExec(args)
	}
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh bun src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  install              Install dependencies (bun install)")
	logs.Raw("  exec [--cwd <dir>]   Run bun command in directory")
	logs.Raw("  version              Print bun version")
	logs.Raw("  test                 Run bun src_v1 plugin tests")
}

func runInstall(args []string) {
	depsDir := logs.GetDialtoneEnv()
	bunBin := filepath.Join(depsDir, "bun", "bin", "bun")

	cwd, bunArgs := extractCwd(args)

	cmd := exec.Command(bunBin, append([]string{"install"}, bunArgs...)...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

func runVersion() {
	depsDir := logs.GetDialtoneEnv()
	bunBin := filepath.Join(depsDir, "bun", "bin", "bun")
	cmd := exec.Command(bunBin, "--version")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func runExec(args []string) {
	if len(args) == 0 {
		logs.Fatal("Usage: ./dialtone.sh bun src_v1 exec <args...>")
	}

	cwd, bunArgs := extractCwd(args)
	if len(bunArgs) == 0 {
		logs.Fatal("Usage: ./dialtone.sh bun src_v1 exec <args...>")
	}

	depsDir := logs.GetDialtoneEnv()
	bunBin := filepath.Join(depsDir, "bun", "bin", "bun")
	if _, err := os.Stat(bunBin); os.IsNotExist(err) {
		logs.Fatal("Bun toolchain not found at %s. Run './dialtone.sh bun src_v1 install' first.", bunBin)
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
		logs.Fatal("Bun command failed: %v", err)
	}
}

func normalizeBunArgs(args []string) ([]string, bool, error) {
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
			return nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh bun src_v1 <command> [args])")
		}
		return append([]string{args[1]}, args[2:]...), false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		if args[1] != "src_v1" {
			return nil, false, fmt.Errorf("unsupported version %s", args[1])
		}
		return append([]string{args[0]}, args[2:]...), true, nil
	}
	return nil, false, fmt.Errorf("expected version as first bun argument (usage: ./dialtone.sh bun src_v1 <command> [args])")
}

func isHelp(s string) bool {
	switch strings.TrimSpace(s) {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
}

func runTests() {
	repoRoot, err := findRepoRoot()
	if err != nil {
		logs.Fatal("%v", err)
	}
	cmd := exec.Command("go", "run", "./plugins/bun/src_v1/test/cmd/main.go")
	cmd.Dir = filepath.Join(repoRoot, "src")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", fmt.Errorf("repo root not found")
		}
		cwd = parent
	}
}

func extractCwd(args []string) (string, []string) {
	var cwd string
	filtered := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--cwd" {
			if i+1 >= len(args) {
				logs.Fatal("Missing value for --cwd")
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

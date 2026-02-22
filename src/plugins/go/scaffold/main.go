package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	version, command, cmdArgs, warnedOldOrder, err := parseArgs(os.Args[1:])
	if err != nil {
		logs.Error("%v", err)
		printUsage()
		os.Exit(1)
	}
	if warnedOldOrder {
		logs.Warn("old go CLI order is deprecated. Use: ./dialtone.sh go src_v1 <command> [args]")
	}
	if version != "src_v1" {
		logs.Error("Unsupported version %s", version)
		os.Exit(1)
	}

	switch command {
	case "help", "-h", "--help":
		printUsage()
	case "install":
		runInstall(cmdArgs)
	case "exec", "run":
		runExec(cmdArgs)
	case "version":
		runExec([]string{"version"})
	case "test":
		runTests()
	default:
		logs.Error("Unknown go scaffold command: %s", command)
		printUsage()
		os.Exit(1)
	}
}

func parseArgs(args []string) (version, command string, rest []string, warnedOldOrder bool, err error) {
	if len(args) == 0 {
		return "", "", nil, false, fmt.Errorf("missing arguments")
	}
	if isHelp(args[0]) {
		return "src_v1", "help", nil, false, nil
	}
	if strings.HasPrefix(args[0], "src_v") {
		if len(args) < 2 {
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh go src_v1 <command> [args])")
		}
		return args[0], args[1], args[2:], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], args[0], args[2:], true, nil
	}
	return "", "", nil, false, fmt.Errorf("expected version as first go argument (usage: ./dialtone.sh go src_v1 <command> [args])")
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh go src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  install [--latest]   Install managed Go runtime")
	logs.Raw("  exec <args...>       Run managed go command")
	logs.Raw("  run <args...>        Alias for exec")
	logs.Raw("  version              Print managed go version")
	logs.Raw("  test                 Run go src_v1 plugin tests")
}

func runInstall(args []string) {
	pluginDir, err := os.Getwd()
	if err != nil {
		logs.Error("Failed to resolve go plugin directory: %v", err)
		os.Exit(1)
	}
	installer := filepath.Join(pluginDir, "install.sh")
	cmd := exec.Command("bash", append([]string{installer}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

func runExec(args []string) {
	if len(args) == 0 {
		logs.Error("Usage: ./dialtone.sh go src_v1 exec <args...>")
		os.Exit(1)
	}

	dialtoneEnv := os.Getenv("DIALTONE_ENV")
	if dialtoneEnv == "" {
		logs.Error("DIALTONE_ENV is not set")
		os.Exit(1)
	}

	// Find main module root (the one with dialtone/dev)
	cwd, _ := os.Getwd()
	moduleRoot := cwd
	for {
		goMod := filepath.Join(moduleRoot, "go.mod")
		if data, err := os.ReadFile(goMod); err == nil {
			if strings.Contains(string(data), "module dialtone/dev") {
				break
			}
		}
		parent := filepath.Dir(moduleRoot)
		if parent == moduleRoot {
			moduleRoot = cwd // Fallback
			break
		}
		moduleRoot = parent
	}

	goBin := filepath.Join(dialtoneEnv, "go", "bin", "go")
	cmd := exec.Command(goBin, args...)
	cmd.Dir = moduleRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		logs.Error("go command failed: %v", err)
		os.Exit(1)
	}
}

func runTests() {
	repoRoot, err := findRepoRoot()
	if err != nil {
		logs.Error("%v", err)
		os.Exit(1)
	}
	cmd := exec.Command("go", "run", "./plugins/go/src_v1/test/cmd/main.go")
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

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
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
	installer, err := resolveInstallerPath()
	if err != nil {
		logs.Error("Failed to resolve go installer: %v", err)
		os.Exit(1)
	}
	cmd := exec.Command("bash", append([]string{installer}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

func resolveInstallerPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(cwd, "install.sh"),
		filepath.Join(cwd, "plugins", "go", "install.sh"),
		filepath.Join(cwd, "src", "plugins", "go", "install.sh"),
	}
	if repoRoot, err := findRepoRoot(); err == nil {
		candidates = append(candidates, filepath.Join(repoRoot, "src", "plugins", "go", "install.sh"))
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("go installer not found in expected locations")
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

	args = maybeInjectBuildOutput(args, moduleRoot)
	goBinName := "go"
	if os.Getenv("OS") == "Windows_NT" || filepath.Separator == '\\' {
		goBinName = "go.exe"
	}
	goBin := filepath.Join(dialtoneEnv, "go", "bin", goBinName)
	args = maybeInjectBuildOutput(args, moduleRoot)
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

func maybeInjectBuildOutput(args []string, moduleRoot string) []string {
	if len(args) == 0 {
		return args
	}
	if args[0] != "build" {
		return args
	}
	if hasBuildOutputFlag(args[1:]) {
		return args
	}
	target, ok := extractSingleBuildTarget(args[1:])
	if !ok {
		return args
	}
	outPath, outOK := deriveAutoBuildOutput(moduleRoot, target)
	if !outOK {
		return args
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		logs.Warn("go build auto-output mkdir failed: %v", err)
		return args
	}
	logs.Info("go build auto-output: %s", outPath)
	withOut := make([]string, 0, len(args)+2)
	withOut = append(withOut, "build", "-o", outPath)
	withOut = append(withOut, args[1:]...)
	return withOut
}

func hasBuildOutputFlag(args []string) bool {
	for _, a := range args {
		if a == "-o" || strings.HasPrefix(a, "-o=") {
			return true
		}
	}
	return false
}

func extractSingleBuildTarget(args []string) (string, bool) {
	valueFlags := map[string]bool{
		"-C": true, "-p": true, "-asmflags": true, "-gcflags": true, "-ldflags": true,
		"-mod": true, "-modfile": true, "-pkgdir": true, "-tags": true, "-toolexec": true,
		"-buildmode": true, "-compiler": true, "-overlay": true, "-trimpath": false,
	}
	targets := make([]string, 0, 2)
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			flagName := a
			if idx := strings.Index(flagName, "="); idx >= 0 {
				flagName = flagName[:idx]
			}
			if valueFlags[flagName] && !strings.Contains(a, "=") && i+1 < len(args) {
				i++
			}
			continue
		}
		targets = append(targets, a)
		if len(targets) > 1 {
			return "", false
		}
	}
	if len(targets) != 1 {
		return "", false
	}
	t := strings.TrimSpace(targets[0])
	if t == "" || strings.Contains(t, "...") {
		return "", false
	}
	return t, true
}

func deriveAutoBuildOutput(moduleRoot, target string) (string, bool) {
	rt, err := configv1.ResolveRuntime(moduleRoot)
	if err != nil {
		return "", false
	}
	normalized := filepath.ToSlash(strings.TrimSpace(target))
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.TrimPrefix(normalized, "/")

	name := filepath.Base(normalized)
	if strings.HasSuffix(name, ".go") {
		name = strings.TrimSuffix(name, ".go")
	}
	if strings.TrimSpace(name) == "" || name == "." {
		return "", false
	}

	pluginRe := regexp.MustCompile(`^plugins/([^/]+)/(src_v[^/]+)/`)
	if m := pluginRe.FindStringSubmatch(normalized); len(m) == 3 {
		outDir := configv1.RepoPath(rt, ".dialtone", "bin", "plugins", m[1], m[2])
		return filepath.Join(outDir, name), true
	}

	if normalized == "dev.go" || strings.HasSuffix(normalized, "/dev.go") {
		outDir := configv1.RepoPath(rt, ".dialtone", "bin", "dev")
		return filepath.Join(outDir, name), true
	}

	outDir := configv1.RepoPath(rt, ".dialtone", "bin", "misc")
	return filepath.Join(outDir, name), true
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
		if configv1.HasDialtoneScript(cwd) {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", fmt.Errorf("repo root not found")
		}
		cwd = parent
	}
}

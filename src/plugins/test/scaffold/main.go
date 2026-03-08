package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	version, cmd, passthrough, warnedOldOrder, err := parseArgs(os.Args[1:])
	if err != nil {
		logs.Error("%v", err)
		printUsage()
		os.Exit(1)
	}
	if warnedOldOrder {
		logs.Warn("old test CLI order is deprecated. Use: ./dialtone.sh test src_v1 <command> [args]")
	}

	switch cmd {
	case "install":
		if err := runInstall(version, passthrough); err != nil {
			logs.Error("test install failed: %v", err)
			os.Exit(1)
		}
	case "build":
		if err := runBuild(version, passthrough); err != nil {
			logs.Error("test build failed: %v", err)
			os.Exit(1)
		}
	case "format":
		if err := runFormat(version, passthrough); err != nil {
			logs.Error("test format failed: %v", err)
			os.Exit(1)
		}
	case "test":
		runTests(version, passthrough)
	case "help", "-h", "--help":
		printUsage()
	default:
		logs.Error("Unknown test command: %s", cmd)
		printUsage()
		os.Exit(1)
	}
}

func parseArgs(args []string) (version, command string, passthrough []string, warnedOldOrder bool, err error) {
	if len(args) == 0 {
		return "", "", nil, false, fmt.Errorf("missing arguments")
	}
	if isHelp(args[0]) {
		return "src_v1", "help", nil, false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[0], "src_v") {
		return args[0], args[1], args[2:], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], args[0], append([]string{}, args[2:]...), true, nil
	}
	return "", "", nil, false, fmt.Errorf("expected version as first test argument (for example: ./dialtone.sh test src_v1 test)")
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func runTests(version string, passthrough []string) {
	if version != "src_v1" {
		logs.Error("Unsupported version %s", version)
		os.Exit(1)
	}

	paths, err := testv1.ResolvePaths("")
	if err != nil {
		logs.Error("%v", err)
		os.Exit(1)
	}

	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	args := append([]string{"run", "./plugins/test/src_v1/test/cmd/main.go"}, passthrough...)
	cmd := exec.Command(goBin, args...)
	cmd.Dir = paths.Runtime.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

func runInstall(version string, passthrough []string) error {
	if version != "src_v1" {
		return fmt.Errorf("unsupported version %s", version)
	}
	if len(passthrough) > 0 {
		return fmt.Errorf("install does not accept extra arguments")
	}

	paths, err := testv1.ResolvePaths("")
	if err != nil {
		return err
	}
	uiDir := filepath.Join(paths.Runtime.RepoRoot, "src", "plugins", "test", "src_v1", "ui")
	npmCmd := exec.Command("npm", "install")
	npmCmd.Dir = uiDir
	npmCmd.Stdout = os.Stdout
	npmCmd.Stderr = os.Stderr
	logs.Info("test install ui: npm install")
	return npmCmd.Run()
}

func runBuild(version string, passthrough []string) error {
	if version != "src_v1" {
		return fmt.Errorf("unsupported version %s", version)
	}
	if len(passthrough) > 0 {
		return fmt.Errorf("build does not accept extra arguments")
	}

	paths, err := testv1.ResolvePaths("")
	if err != nil {
		return err
	}

	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}

	goTargets := [][]string{
		{"build", "./plugins/test/scaffold/main.go"},
		{"build", "./plugins/test/src_v1/test/cmd/main.go"},
		{"build", "./plugins/test/src_v1/mock_server"},
	}
	for _, args := range goTargets {
		cmd := exec.Command(goBin, args...)
		cmd.Dir = paths.Runtime.SrcRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		logs.Info("test build go: %s", strings.Join(args, " "))
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	uiDir := filepath.Join(paths.Runtime.RepoRoot, "src", "plugins", "test", "src_v1", "ui")
	npmCmd := exec.Command("npm", "run", "build")
	npmCmd.Dir = uiDir
	npmCmd.Stdout = os.Stdout
	npmCmd.Stderr = os.Stderr
	logs.Info("test build ui: npm run build")
	if err := npmCmd.Run(); err != nil {
		return err
	}

	return nil
}

func runFormat(version string, passthrough []string) error {
	if version != "src_v1" {
		return fmt.Errorf("unsupported version %s", version)
	}
	if len(passthrough) > 0 {
		return fmt.Errorf("format does not accept extra arguments")
	}

	paths, err := testv1.ResolvePaths("")
	if err != nil {
		return err
	}

	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	goCmd := exec.Command(goBin, "fmt", "./plugins/test/...")
	goCmd.Dir = paths.Runtime.SrcRoot
	goCmd.Stdout = os.Stdout
	goCmd.Stderr = os.Stderr
	logs.Info("test format go: go fmt ./plugins/test/...")
	if err := goCmd.Run(); err != nil {
		return err
	}

	uiDir := filepath.Join(paths.Runtime.RepoRoot, "src", "plugins", "test", "src_v1", "ui")
	npmCmd := exec.Command("npm", "run", "format")
	npmCmd.Dir = uiDir
	npmCmd.Stdout = os.Stdout
	npmCmd.Stderr = os.Stderr
	logs.Info("test format ui: npm run format")
	return npmCmd.Run()
}

func printUsage() {
	logs.Info("Usage: ./dialtone.sh test src_v1 <command> [args]")
	logs.Info("  install         Install test plugin UI dependencies")
	logs.Info("  build           Build test plugin Go entrypoints and Vite UI")
	logs.Info("  format          Format test plugin Go code and UI sources")
	logs.Info("  test            Run test plugin verification suite")
}

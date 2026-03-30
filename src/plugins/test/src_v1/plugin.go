package testv1

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	testlib "dialtone/dev/plugins/test/src_v1/go"
)

func Run(args []string) error {
	logs.SetOutput(os.Stdout)
	if len(args) == 0 {
		printUsage()
		return nil
	}

	version, cmd, passthrough, warnedOldOrder, err := parseArgs(args)
	if err != nil {
		return err
	}
	if warnedOldOrder {
		logs.Warn("old test CLI order is deprecated. Use: ./dialtone.sh test src_v1 <command> [args]")
	}
	if version != "src_v1" {
		return fmt.Errorf("unsupported version %s", version)
	}

	switch cmd {
	case "install":
		return runInstall(passthrough)
	case "build":
		return runBuild(passthrough)
	case "format":
		return runFormat(passthrough)
	case "lint":
		return runLint(passthrough)
	case "test":
		return runTests(passthrough)
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown test command: %s", cmd)
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

func runTests(passthrough []string) error {
	paths, err := testlib.ResolvePaths("")
	if err != nil {
		return err
	}
	goBin := managedGoBin(paths)
	args := append([]string{"run", "./plugins/test/src_v1/test/cmd/main.go"}, passthrough...)
	return runCommand(paths.Runtime.SrcRoot, goBin, args, "test run")
}

func runInstall(passthrough []string) error {
	if len(passthrough) > 0 {
		return fmt.Errorf("install does not accept extra arguments")
	}
	paths, err := testlib.ResolvePaths("")
	if err != nil {
		return err
	}
	return runCommand(paths.Preset.UI, managedBunBin(paths), []string{"install"}, "ui install")
}

func runBuild(passthrough []string) error {
	if len(passthrough) > 0 {
		return fmt.Errorf("build does not accept extra arguments")
	}
	paths, err := testlib.ResolvePaths("")
	if err != nil {
		return err
	}
	goBin := managedGoBin(paths)
	binDir := filepath.Join(paths.Runtime.RepoRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}

	type goTarget struct {
		output string
		pkg    string
	}
	goTargets := []goTarget{
		{output: filepath.Join(binDir, "dialtone_test_v1"), pkg: "./plugins/test/scaffold/main.go"},
		{output: filepath.Join(binDir, "dialtone_test_v1_runner"), pkg: "./plugins/test/src_v1/test/cmd/main.go"},
		{output: filepath.Join(binDir, "dialtone_test_v1_mock_server"), pkg: "./plugins/test/src_v1/mock_server"},
	}
	for _, target := range goTargets {
		if err := runCommand(paths.Runtime.SrcRoot, goBin, []string{"build", "-o", target.output, target.pkg}, "go build"); err != nil {
			return err
		}
	}
	return runCommand(paths.Preset.UI, managedBunBin(paths), []string{"run", "build"}, "ui build")
}

func runFormat(passthrough []string) error {
	if len(passthrough) > 0 {
		return fmt.Errorf("format does not accept extra arguments")
	}
	paths, err := testlib.ResolvePaths("")
	if err != nil {
		return err
	}
	if err := runCommand(paths.Runtime.SrcRoot, managedGoBin(paths), []string{"fmt", "./plugins/test/..."}, "go fmt"); err != nil {
		return err
	}
	return runCommand(paths.Preset.UI, managedBunBin(paths), []string{"run", "format"}, "ui format")
}

func runLint(passthrough []string) error {
	if len(passthrough) > 0 {
		return fmt.Errorf("lint does not accept extra arguments")
	}
	paths, err := testlib.ResolvePaths("")
	if err != nil {
		return err
	}
	if err := runCommand(paths.Runtime.SrcRoot, managedGoBin(paths), []string{"vet", "./plugins/test/..."}, "go vet"); err != nil {
		return err
	}
	return runCommand(paths.Preset.UI, managedBunBin(paths), []string{"run", "lint"}, "ui lint")
}

func managedGoBin(paths testlib.Paths) string {
	if v := strings.TrimSpace(paths.Runtime.GoBin); v != "" {
		return v
	}
	return "go"
}

func managedBunBin(paths testlib.Paths) string {
	if v := strings.TrimSpace(paths.Runtime.BunBin); v != "" {
		return v
	}
	return "bun"
}

func runCommand(dir, bin string, args []string, label string) error {
	logs.Info("test %s: %s %s", label, bin, strings.Join(args, " "))
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	return cmd.Run()
}

func printUsage() {
	logs.Info("Usage: ./dialtone.sh test src_v1 <command> [args]")
	logs.Info("  install         Install test plugin UI dependencies")
	logs.Info("  format          Format test plugin Go code and UI sources")
	logs.Info("  lint            Run test plugin Go vet and UI lint checks")
	logs.Info("  build           Build test plugin Go entrypoints and Vite UI")
	logs.Info("  test            Run test plugin verification suite")
}

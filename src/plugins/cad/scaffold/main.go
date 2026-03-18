package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	cadv1 "dialtone/dev/plugins/cad/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)

	version, command, rest, warnedOldOrder, err := parseArgs(os.Args[1:])
	if err != nil {
		logs.Error("%v", err)
		printUsage()
		os.Exit(1)
	}
	if warnedOldOrder {
		logs.Warn("old cad CLI order is deprecated. Use: ./dialtone.sh cad src_v1 <command> [args]")
	}

	switch version {
	case "src_v1":
		switch command {
		case "test":
			if err := runTests(rest); err != nil {
				logs.Error("cad src_v1 tests failed: %v", err)
				os.Exit(1)
			}
			return
		case "format":
			if err := runFormat(); err != nil {
				logs.Error("cad src_v1 format failed: %v", err)
				os.Exit(1)
			}
			return
		case "build":
			if err := runBuild(); err != nil {
				logs.Error("cad src_v1 build failed: %v", err)
				os.Exit(1)
			}
			return
		}
		if err := cadv1.Run(command, rest); err != nil {
			logs.Error("%v", err)
			os.Exit(1)
		}
	default:
		logs.Error("unsupported version %s", version)
		os.Exit(1)
	}
}

func parseArgs(args []string) (version, command string, rest []string, warnedOldOrder bool, err error) {
	if len(args) == 0 {
		return "src_v1", "help", nil, false, nil
	}
	if isHelp(args[0]) {
		return "src_v1", "help", nil, false, nil
	}
	if strings.HasPrefix(args[0], "src_v") {
		if len(args) < 2 {
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh cad src_v1 <command> [args])")
		}
		return args[0], args[1], args[2:], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], args[0], args[2:], true, nil
	}
	// Preserve old single-version command shape like: ./dialtone.sh cad server
	return "src_v1", args[0], args[1:], true, nil
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh cad src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  serve [--port <n>]   Start the CAD backend server")
	logs.Raw("  server [--port <n>]  Alias for serve")
	logs.Raw("  status [--port <n>]  Check local CAD server health")
	logs.Raw("  stop [--port <n>]    Stop the tracked local CAD server")
	logs.Raw("  build                Build the CAD UI assets")
	logs.Raw("  format               Format Go and UI sources")
	logs.Raw("  test                 Run cad src_v1 test suite")
	logs.Raw("  help                 Show this help")
}

func runTests(args []string) error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	cmdArgs := append([]string{"run", "./plugins/cad/src_v1/test/cmd/main.go"}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = filepath.Join(repoRoot, "src")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runFormat() error {
	paths, err := cadv1.ResolvePaths("", "src_v1")
	if err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: cad format: formatting go sources")
	if err := runCmd(paths.Preset.PluginVersionRoot, "gofmt", "-w",
		filepath.Join(paths.Preset.Go, "cad.go"),
		filepath.Join(paths.Preset.Go, "paths.go"),
		filepath.Join(paths.Preset.Go, "plugin.go"),
		filepath.Join(paths.Preset.TestCmd, "main.go"),
		filepath.Join(paths.Preset.Test, "01_self_check", "suite.go"),
		filepath.Join(paths.Preset.Test, "02_browser_smoke", "suite.go"),
	); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: cad format: formatting ui sources")
	if err := runBun(paths, "run", "format"); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: cad format: completed")
	return nil
}

func runBuild() error {
	paths, err := cadv1.ResolvePaths("", "src_v1")
	if err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: cad build: installing ui dependencies")
	if err := runBun(paths, "install"); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: cad build: building ui dist")
	if err := runBun(paths, "run", "build"); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: cad build: ui dist ready")
	return nil
}

func runBun(paths cadv1.Paths, args ...string) error {
	bunBin := filepath.Join(paths.Runtime.DialtoneEnv, "bun", "bin", "bun")
	if _, err := os.Stat(bunBin); err != nil {
		bunBin = "bun"
	}
	return runCmd(paths.UIDir, bunBin, args...)
}

func runCmd(dir, bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
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

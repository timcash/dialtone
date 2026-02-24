package main

import (
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	bun_plugin "dialtone/dev/plugins/bun/src_v1/go"
	go_plugin "dialtone/dev/plugins/go/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	test_plugin "dialtone/dev/plugins/test/src_v1/go"
	wsl_ops "dialtone/dev/plugins/wsl/src_v3/cmd/ops"
)

func main() {
	logs.SetOutput(os.Stdout)
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	version, command, args, warnedOldOrder, err := parseArgs(os.Args[1:])
	if err != nil {
		logs.Error("%v", err)
		printUsage()
		os.Exit(1)
	}
	if warnedOldOrder {
		logs.Warn("old wsl CLI order is deprecated. Use: ./dialtone.sh wsl src_vN <command> [args]")
	}

	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		logs.Error("wsl error: %v", err)
		os.Exit(1)
	}
	repoRoot := rt.RepoRoot

	if version == "src_v3" {
		err = runSrcV3(command, repoRoot, args)
	} else {
		err = runGeneric(version, command, repoRoot, args)
	}
	
	if err != nil {
		logs.Error("wsl error: %v", err)
		os.Exit(1)
	}
}

func parseArgs(args []string) (version, command string, rest []string, warnedOldOrder bool, err error) {
	if len(args) == 0 {
		return "", "", nil, false, fmt.Errorf("missing arguments")
	}
	if isHelp(args[0]) {
		return "src_v3", "help", nil, false, nil
	}
	if strings.HasPrefix(args[0], "src_v") {
		if len(args) < 2 {
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh wsl src_vN <command> [args])")
		}
		return args[0], args[1], args[2:], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], args[0], args[2:], true, nil
	}

	// Fallback: no explicit version provided, use latest version and first arg as command.
	return "", args[0], args[1:], false, nil
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func runSrcV3(command, repoRoot string, args []string) error {
	switch command {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "install":
		return wsl_ops.Install(args...)
	case "build":
		return wsl_ops.Build(args...)
	case "dev":
		return wsl_ops.Dev(repoRoot, args)
	case "fmt":
		pkg := "./plugins/wsl/src_v3/..."
		return go_plugin.RunGo("fmt", pkg)
	case "vet":
		pkg := "./plugins/wsl/src_v3/..."
		return go_plugin.RunGo("vet", pkg)
	case "go-build":
		pkg := "./plugins/wsl/src_v3/..."
		return go_plugin.RunGo("build", pkg)
	case "lint":
		rt, _ := configv1.ResolveRuntime(repoRoot)
		preset := configv1.NewPluginPreset(rt, "wsl", "src_v3")
		return bun_plugin.RunBun(preset.UI, "run", "lint")
	case "test":
		testPkg := "./plugins/wsl/src_v3/test/cmd"
		return go_plugin.RunGo("run", testPkg)
	default:
		return fmt.Errorf("unknown wsl command: %s", command)
	}
}

func runGeneric(version, command, repoRoot string, args []string) error {
	if version == "" {
		version = getLatestVersionDir(repoRoot)
	}
	rt, err := configv1.ResolveRuntime(repoRoot)
	if err != nil {
		return err
	}
	preset := configv1.NewPluginPreset(rt, "wsl", version)

	switch command {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "install":
		return bun_plugin.RunBun(preset.UI, "install", "--force")
	case "fmt":
		pkg := "./plugins/wsl/" + version + "/..."
		return go_plugin.RunGo("fmt", pkg)
	case "vet":
		pkg := "./plugins/wsl/" + version + "/..."
		return go_plugin.RunGo("vet", pkg)
	case "go-build":
		pkg := "./plugins/wsl/" + version + "/..."
		return go_plugin.RunGo("build", pkg)
	case "lint":
		return bun_plugin.RunBun(preset.UI, "run", "lint")
	case "format":
		return bun_plugin.RunBun(preset.UI, "run", "format")
	case "build":
		return bun_plugin.RunBun(preset.UI, "run", "build")
	case "dev":
		pluginDir := preset.PluginVersionRoot
		uiDir := filepath.Join(pluginDir, "ui")
		opts := test_plugin.DevOptions{
			RepoRoot:          repoRoot,
			PluginDir:         pluginDir,
			UIDir:             uiDir,
			DevPort:           3000,
			Role:              "wsl-dev",
			BrowserMetaPath:   filepath.Join(pluginDir, "dev.browser.json"),
			BrowserModeEnvVar: "WSL_DEV_BROWSER_MODE",
		}
		return test_plugin.RunDev(opts)
	case "test":
		// For now, delegate to internal test package if it exists
		testPkg := "./plugins/wsl/" + version + "/test/cmd"
		return go_plugin.RunGo("run", testPkg)
	default:
		return fmt.Errorf("unknown wsl command: %s", command)
	}
}

func getLatestVersionDir(repoRoot string) string {
	rt, err := configv1.ResolveRuntime(repoRoot)
	if err != nil {
		return "src_v3"
	}
	pluginDir := filepath.Join(rt.SrcRoot, "plugins", "wsl")
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return "src_v3"
	}
	maxVer := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "src_v") {
			v, _ := strconv.Atoi(entry.Name()[5:])
			if v > maxVer {
				maxVer = v
			}
		}
	}
	if maxVer == 0 {
		return "src_v3"
	}
	return fmt.Sprintf("src_v%d", maxVer)
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh wsl src_vN <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  install      Install dependencies")
	logs.Raw("  fmt          Run go fmt")
	logs.Raw("  vet          Run go vet")
	logs.Raw("  go-build     Run go build")
	logs.Raw("  lint         Run TS lint")
	logs.Raw("  format       Run TS format")
	logs.Raw("  build        Build UI")
	logs.Raw("  dev          Start dev server")
	logs.Raw("  test         Run tests")
}

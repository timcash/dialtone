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
	case "build-image":
		return wsl_ops.BuildImage(args...)
	case "dev":
		return wsl_ops.Dev(repoRoot, args)
	case "run", "serve":
		return wsl_ops.Run(repoRoot, args)
	case "list", "ls", "status":
		return wsl_ops.List(args)
	case "create", "spawn":
		return wsl_ops.Create(args)
	case "start":
		return wsl_ops.Start(args)
	case "stop":
		return wsl_ops.Stop(args)
	case "delete", "rm":
		return wsl_ops.Delete(args)
	case "exec":
		return wsl_ops.Exec(args)
	case "terminal", "open-terminal":
		return wsl_ops.OpenTerminal(args)
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
	case "run", "serve":
		runArgs := []string{"run", "./plugins/wsl/" + version + "/cmd/server/main.go"}
		runArgs = append(runArgs, args...)
		return go_plugin.RunGo(runArgs...)
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
	logs.Raw("  install      Install CLI/runtime prerequisites (UI skipped by default)")
	logs.Raw("  fmt          Run go fmt")
	logs.Raw("  vet          Run go vet")
	logs.Raw("  go-build     Run go build")
	logs.Raw("  lint         Run TS lint")
	logs.Raw("  format       Run TS format")
	logs.Raw("  build        Build WSL server binary (UI skipped by default)")
	logs.Raw("  build-image  Ensure reusable alpine build image exists for cross-builds")
	logs.Raw("  dev          Start dev server")
	logs.Raw("  run          Start WSL plugin server")
	logs.Raw("  serve        Alias for run")
	logs.Raw("  list         List WSL instances")
	logs.Raw("  status       Alias for list")
	logs.Raw("  create       Create Alpine-backed WSL instance")
	logs.Raw("  start        Start a WSL instance and keep it running")
	logs.Raw("  stop         Stop a WSL instance")
	logs.Raw("  delete       Delete a WSL instance")
	logs.Raw("  exec         Run a command inside a WSL instance")
	logs.Raw("  terminal     Open a desktop terminal attached to a WSL shell")
	logs.Raw("  test         Run tests")
	logs.Raw("")
	logs.Raw("Notes:")
	logs.Raw("  install/build skip the UI by default; pass --with-ui to include frontend assets")
}

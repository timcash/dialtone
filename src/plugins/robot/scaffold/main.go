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
	robot_cli "dialtone/dev/plugins/robot/src_v1/cmd/cli"
	robot_ops "dialtone/dev/plugins/robot/src_v1/cmd/ops"
	test_plugin "dialtone/dev/plugins/test/src_v1/go"
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
		logs.Warn("old robot CLI order is deprecated. Use: ./dialtone.sh robot src_v1 <command> [args]")
	}

	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		logs.Error("robot error: %v", err)
		os.Exit(1)
	}
	repoRoot := rt.RepoRoot

	if version == "src_v1" {
		err = runSrcV1(command, repoRoot, args)
	} else {
		err = runGeneric(version, command, repoRoot, args)
	}
	if err != nil {
		logs.Error("robot error: %v", err)
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
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh robot src_v1 <command> [args])")
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

func runSrcV1(command, repoRoot string, args []string) error {
	switch command {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "install":
		return robot_ops.Install(args...)
	case "fmt":
		return robot_ops.Fmt()
	case "vet":
		return robot_ops.Vet()
	case "go-build":
		return robot_ops.GoBuild()
	case "lint":
		return robot_ops.Lint()
	case "format":
		return robot_ops.Format()
	case "build":
		return robot_ops.Build(args...)
	case "dev":
		return robot_ops.Dev(repoRoot, args)
	case "test":
		return robot_ops.Test(repoRoot, args)
	case "serve":
		return robot_ops.Serve(repoRoot, args...)
	case "sleep":
		return robot_ops.Sleep(repoRoot, args)
	case "wake":
		return robot_ops.Wake(repoRoot, args)
	case "ui-run":
		port := 0
		for i, arg := range args {
			if arg == "--port" && i+1 < len(args) {
				port, _ = strconv.Atoi(args[i+1])
			}
		}
		return robot_ops.UIRun(port)
	case "deploy-test":
		return robot_cli.RunDeployTest("src_v1", args)
	case "deploy":
		return robot_cli.RunDeploy("src_v1", args)
	case "sync-code":
		return robot_cli.RunSyncCode("src_v1", args)
	case "diagnostic":
		return robot_cli.RunDiagnostic("src_v1")
	case "vpn-test":
		return robot_cli.RunVPNTest(args)
	default:
		return fmt.Errorf("unknown robot command: %s", command)
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
	preset := configv1.NewPluginPreset(rt, "robot", version)

	switch command {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "install":
		return bun_plugin.RunBun(preset.UI, "install", "--force")
	case "fmt":
		pkg := "./plugins/robot/" + version + "/..."
		return go_plugin.RunGo("fmt", pkg)
	case "vet":
		pkg := "./plugins/robot/" + version + "/..."
		return go_plugin.RunGo("vet", pkg)
	case "go-build":
		pkg := "./plugins/robot/" + version + "/..."
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
			Role:              "robot-dev",
			BrowserMetaPath:   filepath.Join(pluginDir, "dev.browser.json"),
			BrowserModeEnvVar: "ROBOT_DEV_BROWSER_MODE",
		}
		return test_plugin.RunDev(opts)
	case "test":
		return fmt.Errorf("standard test logic not yet implemented for robot generic fallback")
	default:
		return fmt.Errorf("unknown robot command: %s", command)
	}
}

func getLatestVersionDir(repoRoot string) string {
	rt, err := configv1.ResolveRuntime(repoRoot)
	if err != nil {
		return "src_v1"
	}
	pluginDir := configv1.NewPluginPreset(rt, "robot", "src_v1").PluginBase
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return "src_v1"
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
		return "src_v1"
	}
	return fmt.Sprintf("src_v%d", maxVer)
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh robot src_v1 <command> [args]")
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
	logs.Raw("  serve        Start backend server")
	logs.Raw("  sleep        Run lightweight sleep server")
	logs.Raw("  wake         Repoint Cloudflare relay tunnel back to robot")
	logs.Raw("  ui-run       Start UI server")
	logs.Raw("  deploy       Build and deploy robot service to ROBOT_HOST")
	logs.Raw("  sync-code    Sync minimal robot source tree to remote host for on-device build/test")
	logs.Raw("  deploy-test  Run step-by-step verification on remote robot")
	logs.Raw("  diagnostic   Run UI and connectivity diagnostics")
	logs.Raw("  vpn-test     Test Tailscale connectivity")
}

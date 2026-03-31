package robotv2

import (
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	bun_plugin "dialtone/dev/plugins/bun/src_v1/go"
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	go_plugin "dialtone/dev/plugins/go/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	ssh_plugin "dialtone/dev/plugins/ssh/src_v1/go"
	test_plugin "dialtone/dev/plugins/test/src_v1/go"
)

func Run(version, command string, args []string) error {
	logs.SetOutput(os.Stdout)
	if isHelp(command) {
		PrintUsage()
		return nil
	}

	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return fmt.Errorf("robot error: %w", err)
	}
	repoRoot := rt.RepoRoot

	if version == "src_v1" {
		return fmt.Errorf("robot src_v1 is no longer supported from this scaffold; use ./dialtone.sh robot src_v2 ...")
	}
	if strings.TrimSpace(version) == "" {
		return fmt.Errorf("missing robot version (usage: ./dialtone.sh robot src_v2 <command> [args])")
	}
	return runGeneric(version, command, repoRoot, args)
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func runGeneric(version, command, repoRoot string, args []string) error {
	rt, err := configv1.ResolveRuntime(repoRoot)
	if err != nil {
		return err
	}
	preset := configv1.NewPluginPreset(rt, "robot", version)

	switch command {
	case "help", "-h", "--help":
		PrintUsage()
		return nil
	case "install":
		return ensureRobotUIDeps(repoRoot, preset.UI, true)
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
		if err := ensureRobotUIDeps(repoRoot, preset.UI, false); err != nil {
			return err
		}
		return bun_plugin.RunBun(preset.UI, "run", "lint")
	case "format":
		if err := ensureRobotUIDeps(repoRoot, preset.UI, false); err != nil {
			return err
		}
		return bun_plugin.RunBun(preset.UI, "run", "format")
	case "build":
		if err := ensureRobotUIDeps(repoRoot, preset.UI, false); err != nil {
			return err
		}
		if err := bun_plugin.RunBun(preset.UI, "run", "build"); err != nil {
			return err
		}
		return buildRobotLocalArtifacts(repoRoot)
	case "dev":
		fs := flag.NewFlagSet("robot-dev", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		port := fs.Int("port", 3000, "Dev server port")
		host := fs.String("host", "0.0.0.0", "Vite bind host")
		browserNode := fs.String("browser-node", "", "Optional mesh node for headed browser session (example: legion; use none/off/local to disable)")
		publicURL := fs.String("public-url", "", "Public URL that remote browser should open")
		backendURL := fs.String("backend-url", "", "Backend base URL for Vite proxy routes (/api, /stream, /natsws, /ws)")
		live := fs.Bool("live", false, "Automatically proxy backend to the live robot node configured in env/dialtone.json")
		if err := fs.Parse(args); err != nil {
			return err
		}

		if *live && strings.TrimSpace(*backendURL) == "" {
			if robotNode, err := ssh_plugin.ResolveMeshNode(defaultRobotMeshAlias); err == nil && robotNode.Host != "" {
				*backendURL = fmt.Sprintf("http://%s:18086", robotNode.Host)
				logs.Info("robot dev --live automatically resolved backend url=%s", *backendURL)
			} else {
				logs.Warn("robot dev --live failed to resolve the default robot mesh node: %v", err)
			}
		}

		node := resolveRobotDevBrowserNode(strings.TrimSpace(*browserNode), defaultRobotDevBrowserNode())
		devURL := strings.TrimSpace(*publicURL)
		if node != "" && devURL == "" {
			u, err := inferRobotDevPublicURL(*port)
			if err != nil {
				return err
			}
			devURL = u
		}
		test_plugin.SetRuntimeConfig(test_plugin.RuntimeConfig{BrowserNode: node})
		if node != "" {
			logs.Info("robot src_v2 dev remote browser node=%s url=%s", node, devURL)
		}
		if raw := strings.TrimSpace(*backendURL); raw != "" {
			normalized, err := normalizeRobotBackendURL(raw)
			if err != nil {
				return err
			}
			_ = os.Setenv("VITE_PROXY_TARGET", normalized)
			_ = os.Unsetenv("VITE_NATS_WS_URL")
			logs.Info("robot src_v2 dev backend proxy target=%s (input=%s)", normalized, raw)
		}

		pluginDir := preset.PluginVersionRoot
		uiDir := filepath.Join(pluginDir, "ui")
		opts := test_plugin.DevOptions{
			RepoRoot:        repoRoot,
			PluginDir:       pluginDir,
			UIDir:           uiDir,
			DevPort:         *port,
			DevHost:         strings.TrimSpace(*host),
			DevPublicURL:    devURL,
			Role:            "robot-dev",
			DisableBrowser:  node == "",
			BrowserMetaPath: filepath.Join(pluginDir, "dev.browser.json"),
			NATSURL:         "nats://127.0.0.1:4222",
			NATSSubject:     "logs.dev.robot." + strings.ReplaceAll(version, "_", "-"),
		}
		return test_plugin.RunDev(opts)
	case "test":
		testPkg := "./plugins/robot/" + version + "/test/cmd"
		runArgs := []string{"run", testPkg}
		runArgs = append(runArgs, args...)
		return go_plugin.RunGo(runArgs...)
	case "sync-code":
		return runRobotSyncCode(repoRoot, args)
	case "sync-watch":
		return runRobotSyncWatch(repoRoot, args)
	case "relay":
		return runSrcV2Only(version, command, func() error { return runSrcV2Relay(repoRoot, args) })
	case "publish":
		return runSrcV2Only(version, command, func() error { return runSrcV2Publish(repoRoot, args) })
	case "rollout":
		return runSrcV2Only(version, command, func() error { return runSrcV2Rollout(repoRoot, args) })
	case "nix-diagnostic":
		return runSrcV2Only(version, command, func() error { return runSrcV2NixDiagnostic(repoRoot, args) })
	case "nix-gc":
		return runSrcV2Only(version, command, func() error { return runSrcV2NixGC(args) })
	case "diagnostic":
		return runSrcV2Only(version, command, func() error { return runSrcV2Diagnostic(repoRoot, args) })
	case "clean":
		return runSrcV2Only(version, command, func() error { return runSrcV2Clean(args) })
	default:
		return fmt.Errorf("unknown robot command: %s", command)
	}
}

func runSrcV2Only(version, command string, run func() error) error {
	if version != "src_v2" {
		return fmt.Errorf("%s is currently supported only for robot src_v2", command)
	}
	return run()
}

func defaultRobotDevBrowserNode() string {
	return test_plugin.ResolveDefaultAttachNode(configv1.LookupEnvString("DIALTONE_TEST_BROWSER_NODE"))
}

func resolveRobotDevBrowserNode(requested, fallback string) string {
	requested = strings.TrimSpace(requested)
	if requested == "" || strings.EqualFold(requested, "default") {
		return strings.TrimSpace(fallback)
	}
	if resolved, disabled := test_plugin.ResolveConfiguredAttachNode(requested); disabled {
		return ""
	} else if resolved != "" {
		return resolved
	}
	return strings.TrimSpace(fallback)
}

func inferRobotDevPublicURL(port int) (string, error) {
	wsl, err := ssh_plugin.ResolveMeshNode("wsl")
	if err != nil {
		return "", fmt.Errorf("resolve wsl mesh node for public url: %w", err)
	}
	host := strings.TrimSpace(wsl.Host)
	if host == "" {
		return "", fmt.Errorf("wsl mesh host is empty")
	}
	u := &url.URL{Scheme: "http", Host: fmt.Sprintf("%s:%d", host, port)}
	return u.String(), nil
}

func normalizeRobotBackendURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("invalid --backend-url %q: %w", raw, err)
	}
	if u.Scheme == "" {
		u, err = url.Parse("http://" + strings.TrimSpace(raw))
		if err != nil {
			return "", fmt.Errorf("invalid --backend-url %q: %w", raw, err)
		}
	}
	if strings.TrimSpace(u.Host) == "" {
		return "", fmt.Errorf("invalid --backend-url %q: missing host", raw)
	}
	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func PrintUsage() {
	logs.Raw("Usage: ./dialtone.sh robot src_v2 <command> [args]")
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
	logs.Raw("  sync-code    Sync the robot source tree to a remote host for on-device build/test")
	logs.Raw("  sync-watch   Start, stop, or inspect the continuous robot source sync loop")
	logs.Raw("  relay        Configure a local Cloudflare relay for the robot UI (default subdomain: rover-1)")
	logs.Raw("  publish      Build/publish robot src_v2 composition artifacts to GitHub release only (default target: linux-arm64)")
	logs.Raw("  rollout      Publish + autoswap deploy/update + diagnostic for a robot host over mesh SSH")
	logs.Raw("  nix-diagnostic Verify nix + flake workflow on a robot host over mesh SSH")
	logs.Raw("  nix-gc       Run nix garbage collection on a robot host over mesh SSH")
	logs.Raw("  diagnostic   Verify robot src_v2 composition binaries/processes/endpoints")
	logs.Raw("  clean        Remove remote dialtone source/runtime (use --keep-autoswap to preserve autoswap service/bin)")
}

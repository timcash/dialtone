package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	ssh_plugin "dialtone/dev/plugins/ssh/src_v1/go"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	bun_plugin "dialtone/dev/plugins/bun/src_v1/go"
	go_plugin "dialtone/dev/plugins/go/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	robot_cli "dialtone/dev/plugins/robot/src_v1/cmd/cli"
	robot_ops "dialtone/dev/plugins/robot/src_v1/cmd/ops"
	test_plugin "dialtone/dev/plugins/test/src_v1/go"
	"github.com/chromedp/chromedp"
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
	case "sync-watch":
		return robot_cli.RunSyncWatch("src_v1", args)
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
		fs := flag.NewFlagSet("robot-dev", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		port := fs.Int("port", 3000, "Dev server port")
		host := fs.String("host", "0.0.0.0", "Vite bind host")
		browserNode := fs.String("browser-node", "", "Optional mesh node for headed browser session (example: chroma)")
		publicURL := fs.String("public-url", "", "Public URL that remote browser should open")
		backendURL := fs.String("backend-url", strings.TrimSpace(os.Getenv("ROBOT_DEV_BACKEND_URL")), "Backend base URL for Vite proxy routes (/api, /stream, /natsws, /ws)")
		if err := fs.Parse(args); err != nil {
			return err
		}

		node := strings.TrimSpace(*browserNode)
		if node == "" {
			node = defaultRobotDevBrowserNode()
		}
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
			logs.Info("robot src_v2 dev backend proxy target=%s (input=%s)", normalized, raw)
		}

		pluginDir := preset.PluginVersionRoot
		uiDir := filepath.Join(pluginDir, "ui")
		opts := test_plugin.DevOptions{
			RepoRoot:          repoRoot,
			PluginDir:         pluginDir,
			UIDir:             uiDir,
			DevPort:           *port,
			DevHost:           strings.TrimSpace(*host),
			DevPublicURL:      devURL,
			Role:              "robot-dev",
			BrowserMetaPath:   filepath.Join(pluginDir, "dev.browser.json"),
			BrowserModeEnvVar: "ROBOT_DEV_BROWSER_MODE",
			NATSURL:           "nats://127.0.0.1:4222",
			NATSSubject:       "logs.dev.robot." + strings.ReplaceAll(version, "_", "-"),
		}
		return test_plugin.RunDev(opts)
	case "test":
		testPkg := "./plugins/robot/" + version + "/test/cmd"
		return go_plugin.RunGo("run", testPkg)
	case "sync-code":
		return robot_cli.RunSyncCode(version, args)
	case "sync-watch":
		return robot_cli.RunSyncWatch(version, args)
	case "relay":
		if version != "src_v2" {
			return fmt.Errorf("relay is currently supported only for robot src_v2")
		}
		return runSrcV2Relay(repoRoot, args)
	case "publish":
		if version != "src_v2" {
			return fmt.Errorf("publish is currently supported only for robot src_v2")
		}
		return runSrcV2Publish(repoRoot, args)
	case "rollout":
		if version != "src_v2" {
			return fmt.Errorf("rollout is currently supported only for robot src_v2")
		}
		return runSrcV2Rollout(repoRoot, args)
	case "nix-diagnostic":
		if version != "src_v2" {
			return fmt.Errorf("nix-diagnostic is currently supported only for robot src_v2")
		}
		return runSrcV2NixDiagnostic(repoRoot, args)
	case "nix-gc":
		if version != "src_v2" {
			return fmt.Errorf("nix-gc is currently supported only for robot src_v2")
		}
		return runSrcV2NixGC(args)
	case "diagnostic":
		if version != "src_v2" {
			return fmt.Errorf("diagnostic is currently supported only for robot src_v2")
		}
		return runSrcV2Diagnostic(repoRoot, args)
	case "clean":
		if version != "src_v2" {
			return fmt.Errorf("clean is currently supported only for robot src_v2")
		}
		return runSrcV2Clean(args)
	default:
		return fmt.Errorf("unknown robot command: %s", command)
	}
}

func defaultRobotDevBrowserNode() string {
	if envNode := strings.TrimSpace(os.Getenv("DIALTONE_TEST_BROWSER_NODE")); envNode != "" {
		return envNode
	}
	if robotIsWSL() {
		return "legion"
	}
	return ""
}

func robotIsWSL() bool {
	if strings.TrimSpace(os.Getenv("WSL_DISTRO_NAME")) != "" {
		return true
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
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
	// Use host-root as shared proxy target so /api, /stream, /ws, /natsws stay correct.
	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
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
	logs.Raw("  sync-watch   Start/stop/status continuous source sync loop to remote host")
	logs.Raw("  relay       Configure local Cloudflare relay for robot src_v2 UI (rover-1.dialtone.earth)")
	logs.Raw("  publish      Build/publish robot src_v2 composition artifacts to GitHub release only (default target: linux-arm64)")
	logs.Raw("  rollout      Publish + autoswap deploy/update + diagnostic for robot src_v2 over mesh SSH")
	logs.Raw("  nix-diagnostic Verify nix + flake workflow on remote robot host over mesh SSH")
	logs.Raw("  nix-gc       Run nix garbage collection on remote robot host over mesh SSH")
	logs.Raw("  diagnostic   Verify robot src_v2 composition binaries/processes/endpoints")
	logs.Raw("  clean        Remove remote dialtone source/runtime (use --keep-autoswap to preserve autoswap service/bin)")
	logs.Raw("  deploy-test  Run step-by-step verification on remote robot")
	logs.Raw("  diagnostic   Run UI and connectivity diagnostics")
	logs.Raw("  vpn-test     Test Tailscale connectivity")
}

func runSrcV2Relay(repoRoot string, args []string) error {
	fs := flag.NewFlagSet("robot-src-v2-relay", flag.ContinueOnError)
	subdomain := fs.String("subdomain", "", "Cloudflare relay subdomain (default: DIALTONE_DOMAIN/DIALTONE_HOSTNAME/rover-1)")
	robotUIURL := fs.String("robot-ui-url", "", "Robot UI URL target for relay (overrides --host/--port)")
	// Backward-compatible aliases.
	name := fs.String("name", "", "Deprecated alias for --subdomain")
	url := fs.String("url", "", "Deprecated alias for --robot-ui-url")
	host := fs.String("host", "rover-1", "Robot host for relay target")
	port := fs.String("port", "18086", "Robot web port for relay target")
	service := fs.Bool("service", true, "Install/restart local systemd user relay service")
	if err := fs.Parse(args); err != nil {
		return err
	}

	targetURL := strings.TrimSpace(*robotUIURL)
	if targetURL == "" {
		targetURL = strings.TrimSpace(*url)
	}
	if targetURL == "" {
		h := strings.TrimSpace(*host)
		p := strings.TrimSpace(*port)
		if h == "" {
			return fmt.Errorf("relay requires --host unless --url is provided")
		}
		if p == "" {
			p = "18086"
		}
		targetURL = fmt.Sprintf("http://%s:%s", h, p)
	}

	relayName := strings.TrimSpace(*subdomain)
	if relayName == "" {
		relayName = strings.TrimSpace(*name)
	}
	if relayName == "" {
		relayName = strings.TrimSpace(os.Getenv("DIALTONE_DOMAIN"))
	}
	if relayName == "" {
		relayName = strings.TrimSpace(os.Getenv("DIALTONE_HOSTNAME"))
	}
	if relayName == "" {
		relayName = "rover-1"
	}

	prevDomain, hadDomain := os.LookupEnv("DIALTONE_DOMAIN")
	if err := os.Setenv("DIALTONE_DOMAIN", relayName); err != nil {
		return err
	}
	defer func() {
		if hadDomain {
			_ = os.Setenv("DIALTONE_DOMAIN", prevDomain)
		} else {
			_ = os.Unsetenv("DIALTONE_DOMAIN")
		}
	}()

	if *service {
		if err := robot_ops.Wake(repoRoot, []string{"--url", targetURL}); err != nil {
			return err
		}
		logs.Info("robot src_v2 relay service active: dialtone-proxy-%s.service", relayName)
	} else {
		if err := runDialtone(repoRoot, "cloudflare", "robot", "--name", relayName, "--url", targetURL); err != nil {
			return err
		}
	}
	logs.Info("robot src_v2 relay active: https://%s.dialtone.earth -> %s", relayName, targetURL)
	return nil
}

func runSrcV2Clean(args []string) error {
	fs := flag.NewFlagSet("robot-src-v2-clean", flag.ContinueOnError)
	host := fs.String("host", "", "Mesh node alias/hostname to clean")
	user := fs.String("user", "", "SSH user override (defaults to mesh user)")
	port := fs.String("port", "", "SSH port override (defaults to mesh port)")
	password := fs.String("pass", "", "SSH password (optional; key auth preferred)")
	keepAutoswap := fs.Bool("keep-autoswap", false, "Remove everything except autoswap service/binary")
	restartAutoswap := fs.Bool("restart-autoswap", false, "When --keep-autoswap, restart autoswap after cleanup")
	if err := fs.Parse(args); err != nil {
		return err
	}
	target := strings.TrimSpace(*host)
	if target == "" {
		return fmt.Errorf("clean requires --host")
	}
	node, err := ssh_plugin.ResolveMeshNode(target)
	if err != nil {
		return fmt.Errorf("clean requires mesh --host value: %w", err)
	}
	if node.OS == "windows" {
		return fmt.Errorf("clean currently supports linux/macos targets only; got windows node %q", node.Name)
	}
	opts := ssh_plugin.CommandOptions{
		User:     strings.TrimSpace(*user),
		Port:     strings.TrimSpace(*port),
		Password: *password,
	}

	unitLoop := `for unit in $(systemctl --user list-unit-files --type=service --no-pager | awk '{print $1}' | grep -Ei 'dialtone|robot|rover' || true); do
  [ -z "$unit" ] && continue
  systemctl --user stop "$unit" 2>/dev/null || true
  systemctl --user disable "$unit" 2>/dev/null || true
  systemctl --user reset-failed "$unit" 2>/dev/null || true
done`
	unitFilesCleanup := `rm -f "$HOME/.config/systemd/user"/dialtone_*.service "$HOME/.config/systemd/user"/dialtone-*.service "$HOME/.config/systemd/user"/robot*.service "$HOME/.config/systemd/user"/rover*.service 2>/dev/null || true
rm -rf "$HOME/.config/systemd/user"/dialtone*.service.d "$HOME/.config/systemd/user"/robot*.service.d "$HOME/.config/systemd/user"/rover*.service.d 2>/dev/null || true
rm -f "$HOME/.config/systemd/user/default.target.wants"/dialtone*.service "$HOME/.config/systemd/user/default.target.wants"/robot*.service "$HOME/.config/systemd/user/default.target.wants"/rover*.service 2>/dev/null || true`
	autoswapCleanup := `rm -f "$HOME/.dialtone/autoswap/current" 2>/dev/null || true
rm -rf "$HOME/.dialtone/autoswap/bin" "$HOME/.dialtone/autoswap/artifacts" "$HOME/.dialtone/autoswap/releases" "$HOME/.dialtone/autoswap/manifests" 2>/dev/null || true`
	autoswapKeepCleanup := `rm -f "$HOME/.dialtone/autoswap/current" 2>/dev/null || true
rm -rf "$HOME/.dialtone/autoswap/artifacts" "$HOME/.dialtone/autoswap/releases" "$HOME/.dialtone/autoswap/manifests" 2>/dev/null || true
rm -f "$HOME/.dialtone/autoswap/state/runtime.json" "$HOME/.dialtone/autoswap/state/supervisor.json" 2>/dev/null || true`

	if *keepAutoswap {
		unitLoop = `for unit in $(systemctl --user list-unit-files --type=service --no-pager | awk '{print $1}' | grep -Ei 'dialtone|robot|rover' || true); do
  [ -z "$unit" ] && continue
  if [ "$unit" = "dialtone_autoswap.service" ]; then
    continue
  fi
  systemctl --user stop "$unit" 2>/dev/null || true
  systemctl --user disable "$unit" 2>/dev/null || true
  systemctl --user reset-failed "$unit" 2>/dev/null || true
done`
		unitFilesCleanup = `rm -f "$HOME/.config/systemd/user"/dialtone-proxy*.service "$HOME/.config/systemd/user"/robot*.service "$HOME/.config/systemd/user"/rover*.service 2>/dev/null || true
rm -rf "$HOME/.config/systemd/user"/dialtone-proxy*.service.d "$HOME/.config/systemd/user"/robot*.service.d "$HOME/.config/systemd/user"/rover*.service.d 2>/dev/null || true
rm -f "$HOME/.config/systemd/user/default.target.wants"/dialtone-proxy*.service "$HOME/.config/systemd/user/default.target.wants"/robot*.service "$HOME/.config/systemd/user/default.target.wants"/rover*.service 2>/dev/null || true`
		autoswapCleanup = autoswapKeepCleanup
	}

	autoswapPreCmd := ""
	restartCmd := ""
	if *keepAutoswap && *restartAutoswap {
		restartCmd = `systemctl --user restart dialtone_autoswap.service 2>/dev/null || true`
	}
	if *keepAutoswap {
		autoswapPreCmd = `systemctl --user stop dialtone_autoswap.service 2>/dev/null || true`
	}

	cleanupCmd := fmt.Sprintf(`set -e
if [ -d "$HOME/dialtone" ]; then rm -rf "$HOME/dialtone"; fi
%s
%s
systemctl --user daemon-reload || true
%s
%s
%s
echo CLEAN_DONE`, unitLoop, unitFilesCleanup, autoswapPreCmd, autoswapCleanup, restartCmd)
	out, err := ssh_plugin.RunNodeCommand(node.Name, cleanupCmd, opts)
	if err != nil {
		return fmt.Errorf("remote clean failed on %s: %w", node.Name, err)
	}
	if trimmed := strings.TrimSpace(out); trimmed != "" {
		logs.Debug("robot src_v2 clean output: %s", trimmed)
	}

	verifyCmd := `echo -n "dialtone_repo="; [ -e "$HOME/dialtone" ] && echo present || echo removed
echo -n "autoswap_service_active="; systemctl --user is-active dialtone_autoswap.service 2>/dev/null || echo inactive
echo -n "autoswap_service_enabled="; systemctl --user is-enabled dialtone_autoswap.service 2>/dev/null || echo disabled
echo -n "matching_non_autoswap_unit_files_count="; systemctl --user list-unit-files --type=service --no-pager | awk '{print $1}' | grep -Ei 'dialtone|robot|rover' | grep -Ev '^dialtone_autoswap\.service$' | wc -l
echo -n "matching_non_autoswap_active_units_count="; systemctl --user list-units --type=service --all --no-pager | awk '{print $1}' | grep -Ei 'dialtone|robot|rover' | grep -Ev '^dialtone_autoswap\.service$' | wc -l
echo -n "dialtone_runtime_process_count="; ps -eo args | grep -E '/dialtone_(robot_v2|camera_v1|mavlink_v1|repl_v1)( |$)' | grep -v grep | wc -l
echo -n "autoswap_manifests="; [ -d "$HOME/.dialtone/autoswap/manifests" ] && echo present || echo removed
echo -n "autoswap_artifacts="; [ -d "$HOME/.dialtone/autoswap/artifacts" ] && echo present || echo removed
echo -n "autoswap_bin="; [ -d "$HOME/.dialtone/autoswap/bin" ] && echo present || echo removed
echo -n "autoswap_releases="; [ -d "$HOME/.dialtone/autoswap/releases" ] && echo present || echo removed
echo -n "autoswap_current="; [ -L "$HOME/.dialtone/autoswap/current" ] && echo present || echo removed`
	verifyOut, err := ssh_plugin.RunNodeCommand(node.Name, verifyCmd, opts)
	if err != nil {
		return fmt.Errorf("remote clean verification failed on %s: %w", node.Name, err)
	}
	verifyMap := map[string]string{}
	for _, line := range strings.Split(verifyOut, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		verifyMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	required := map[string]string{
		"dialtone_repo":                            "removed",
		"matching_non_autoswap_unit_files_count":   "0",
		"matching_non_autoswap_active_units_count": "0",
		"dialtone_runtime_process_count":           "0",
		"autoswap_manifests":                       "removed",
		"autoswap_artifacts":                       "removed",
		"autoswap_releases":                        "removed",
		"autoswap_current":                         "removed",
	}
	if *keepAutoswap {
		if *restartAutoswap {
			required["autoswap_service_active"] = "active"
		} else {
			required["autoswap_service_active"] = "inactive"
		}
		required["autoswap_bin"] = "present"
	} else {
		required["autoswap_service_active"] = "inactive"
		required["autoswap_service_enabled"] = "disabled"
		required["autoswap_bin"] = "removed"
	}
	for k, want := range required {
		got := strings.TrimSpace(verifyMap[k])
		if got != want {
			return fmt.Errorf("remote clean verification failed on %s: %s=%q (want %q)", node.Name, k, got, want)
		}
	}
	logs.Info("robot src_v2 clean completed on %s", node.Name)
	logs.Info("%s", strings.TrimSpace(verifyOut))
	return nil
}

func runSrcV2Publish(repoRoot string, args []string) error {
	fs := flag.NewFlagSet("robot-src-v2-publish", flag.ContinueOnError)
	repo := fs.String("repo", "timcash/dialtone", "GitHub repo owner/name")
	version := fs.String("version", "", "Release version/tag (default: current git tag or robot-src-v2-<sha>)")
	skipRelease := fs.Bool("skip-release", false, "Skip GitHub release publish check/upload")
	targetFlag := fs.String("target", "linux-arm64", "Release target GOOS-GOARCH (default: linux-arm64)")
	allTargets := fs.Bool("all-targets", false, "Build/publish all release targets (linux/darwin/windows variants)")
	uiOnly := fs.Bool("ui", false, "Publish only robot src_v2 UI dist artifacts (and manifest); skip binary builds")
	if err := fs.Parse(args); err != nil {
		return err
	}

	builds := [][]string{{"robot", "src_v2", "build"}}
	if !*uiOnly {
		builds = append([][]string{
			{"go", "src_v1", "exec", "build", "-o", "../bin/dialtone_autoswap_v1", "./plugins/autoswap/src_v1/cmd/main.go"},
			{"go", "src_v1", "exec", "build", "-o", "../bin/dialtone_robot_v2", "./plugins/robot/src_v2/cmd/server/main.go"},
			{"go", "src_v1", "exec", "build", "-o", "../bin/dialtone_camera_v1", "./plugins/camera/src_v1/cmd/main.go"},
			{"go", "src_v1", "exec", "build", "-o", "../bin/dialtone_mavlink_v1", "./plugins/mavlink/src_v1/cmd/main.go"},
			{"go", "src_v1", "exec", "build", "-o", "../bin/dialtone_repl_v1", "./plugins/repl/src_v1/cmd/repld/main.go"},
		}, builds...)
	}
	for _, cmdArgs := range builds {
		if err := runDialtone(repoRoot, cmdArgs...); err != nil {
			return err
		}
	}
	if *uiOnly {
		logs.Info("robot src_v2 publish: local UI artifacts built (--ui mode)")
	} else {
		logs.Info("robot src_v2 publish: local artifacts built")
	}
	if !*skipRelease {
		resolvedVersion, err := resolveRobotPublishVersion(repoRoot, strings.TrimSpace(*version))
		if err != nil {
			return err
		}
		targets, err := resolvePublishTargets(strings.TrimSpace(*targetFlag), *allTargets)
		if err != nil {
			return err
		}
		if err := publishRobotSrcV2Release(repoRoot, strings.TrimSpace(*repo), resolvedVersion, targets, *uiOnly); err != nil {
			return err
		}
		logs.Info("robot src_v2 publish: release assets up to date version=%s repo=%s", resolvedVersion, strings.TrimSpace(*repo))
	}
	return nil
}

func runSrcV2Rollout(repoRoot string, args []string) error {
	fs := flag.NewFlagSet("robot-src-v2-rollout", flag.ContinueOnError)
	host := fs.String("host", "rover", "Mesh host for rollout")
	port := fs.String("port", "", "SSH port override")
	user := fs.String("user", "", "SSH user override")
	pass := fs.String("pass", "", "SSH password override")
	repo := fs.String("repo", "timcash/dialtone", "GitHub repo owner/name")
	version := fs.String("version", "", "Optional release version/tag override")
	target := fs.String("target", "linux-arm64", "Release target GOOS-GOARCH")
	allTargets := fs.Bool("all-targets", false, "Build/publish all release targets")
	uiOnly := fs.Bool("ui", false, "Publish only UI assets")
	skipPublish := fs.Bool("skip-publish", false, "Skip publish step")
	skipDeploy := fs.Bool("skip-deploy", false, "Skip autoswap deploy step")
	skipUpdate := fs.Bool("skip-update", false, "Skip autoswap refresh step")
	skipDiagnostic := fs.Bool("skip-diagnostic", false, "Skip robot diagnostic step")
	skipUI := fs.Bool("skip-ui", true, "Skip headed browser UI checks during rollout diagnostic")
	publicCheck := fs.Bool("public-check", false, "Verify public UI endpoint during rollout diagnostic")
	requireNix := fs.Bool("require-nix", false, "Fail if nix is not installed on the rover")
	if err := fs.Parse(args); err != nil {
		return err
	}

	targetHost := strings.TrimSpace(*host)
	if targetHost == "" {
		return fmt.Errorf("rollout requires --host")
	}
	node, err := ssh_plugin.ResolveMeshNode(targetHost)
	if err != nil {
		return fmt.Errorf("rollout requires a mesh node alias/hostname for --host: %w", err)
	}
	targetUser := strings.TrimSpace(*user)
	if targetUser == "" {
		targetUser = node.User
	}
	if targetUser == "" {
		return fmt.Errorf("rollout requires --user or a mesh node with a default user")
	}

	cmdOpts := ssh_plugin.CommandOptions{
		User:     targetUser,
		Port:     strings.TrimSpace(*port),
		Password: strings.TrimSpace(*pass),
	}
	nixProbe := "if command -v nix >/dev/null 2>&1; then nix --extra-experimental-features 'nix-command flakes' --version 2>/dev/null || nix --version; else echo MISSING; fi"
	nixOut, err := ssh_plugin.RunNodeCommand(node.Name, nixProbe, cmdOpts)
	if err != nil {
		return fmt.Errorf("rollout nix probe failed on %s: %w", node.Name, err)
	}
	nixOut = strings.TrimSpace(nixOut)
	if strings.EqualFold(nixOut, "MISSING") {
		if *requireNix {
			return fmt.Errorf("rollout requires nix on %s, but nix is not installed", node.Name)
		}
		logs.Warn("robot src_v2 rollout: nix not installed on %s; continuing with autoswap release artifacts only", node.Name)
	} else {
		logs.Info("robot src_v2 rollout: remote nix available on %s (%s)", node.Name, nixOut)
	}

	if !*skipPublish {
		publishArgs := []string{"--repo", strings.TrimSpace(*repo), "--target", strings.TrimSpace(*target)}
		if strings.TrimSpace(*version) != "" {
			publishArgs = append(publishArgs, "--version", strings.TrimSpace(*version))
		}
		if *allTargets {
			publishArgs = append(publishArgs, "--all-targets")
		}
		if *uiOnly {
			publishArgs = append(publishArgs, "--ui")
		}
		if err := runSrcV2Publish(repoRoot, publishArgs); err != nil {
			return err
		}
	}

	if !*skipDeploy {
		deployArgs := []string{"autoswap", "src_v1", "deploy", "--host", node.Name, "--user", targetUser, "--repo", strings.TrimSpace(*repo), "--service"}
		if strings.TrimSpace(*port) != "" {
			deployArgs = append(deployArgs, "--port", strings.TrimSpace(*port))
		}
		if strings.TrimSpace(*pass) != "" {
			deployArgs = append(deployArgs, "--pass", strings.TrimSpace(*pass))
		}
		if err := runDialtone(repoRoot, deployArgs...); err != nil {
			return err
		}
	}

	if !*skipUpdate {
		updateArgs := []string{"autoswap", "src_v1", "update", "--host", node.Name, "--user", targetUser}
		if strings.TrimSpace(*port) != "" {
			updateArgs = append(updateArgs, "--port", strings.TrimSpace(*port))
		}
		if strings.TrimSpace(*pass) != "" {
			updateArgs = append(updateArgs, "--pass", strings.TrimSpace(*pass))
		}
		if err := runDialtone(repoRoot, updateArgs...); err != nil {
			return err
		}
	}

	if !*skipDiagnostic {
		diagArgs := []string{"--host", node.Name, "--user", targetUser}
		if strings.TrimSpace(*port) != "" {
			diagArgs = append(diagArgs, "--port", strings.TrimSpace(*port))
		}
		if strings.TrimSpace(*pass) != "" {
			diagArgs = append(diagArgs, "--pass", strings.TrimSpace(*pass))
		}
		if *skipUI {
			diagArgs = append(diagArgs, "--skip-ui")
		}
		if !*publicCheck {
			diagArgs = append(diagArgs, "--public-check=false")
		}
		if err := runSrcV2Diagnostic(repoRoot, diagArgs); err != nil {
			return err
		}
	}

	logs.Info("robot src_v2 rollout completed host=%s user=%s", node.Name, targetUser)
	return nil
}

func runSrcV2NixDiagnostic(repoRoot string, args []string) error {
	fs := flag.NewFlagSet("robot-src-v2-nix-diagnostic", flag.ContinueOnError)
	host := fs.String("host", "rover", "Mesh host for nix diagnostic")
	port := fs.String("port", "", "SSH port override")
	user := fs.String("user", "", "SSH user override")
	pass := fs.String("pass", "", "SSH password override")
	remoteRepo := fs.String("remote-repo", "", "Remote repo root (default: first mesh repo candidate or ~/dialtone)")
	syncFlake := fs.Bool("sync-flake", true, "Sync local flake.nix and dialtone.sh to remote repo before diagnostics")
	if err := fs.Parse(args); err != nil {
		return err
	}
	node, err := ssh_plugin.ResolveMeshNode(strings.TrimSpace(*host))
	if err != nil {
		return err
	}
	targetUser := strings.TrimSpace(*user)
	if targetUser == "" {
		targetUser = node.User
	}
	if targetUser == "" {
		return fmt.Errorf("nix-diagnostic requires --user or a mesh node with a default user")
	}
	targetRepo := strings.TrimSpace(*remoteRepo)
	if targetRepo == "" {
		if len(node.RepoCandidates) > 0 {
			targetRepo = strings.TrimSpace(node.RepoCandidates[0])
		}
	}
	if targetRepo == "" {
		targetRepo = filepath.ToSlash(filepath.Join("/home", targetUser, "dialtone"))
	}
	cmdOpts := ssh_plugin.CommandOptions{
		User:     targetUser,
		Port:     strings.TrimSpace(*port),
		Password: strings.TrimSpace(*pass),
	}
	if *syncFlake {
		if _, err := ssh_plugin.RunNodeCommand(node.Name, "mkdir -p "+shellSingleQuote(targetRepo), cmdOpts); err != nil {
			return fmt.Errorf("nix-diagnostic prepare remote repo failed: %w", err)
		}
		if err := ssh_plugin.UploadNodeFile(node.Name, filepath.Join(repoRoot, "flake.nix"), filepath.ToSlash(filepath.Join(targetRepo, "flake.nix")), cmdOpts); err != nil {
			return fmt.Errorf("nix-diagnostic sync flake.nix failed: %w", err)
		}
		if err := ssh_plugin.UploadNodeFile(node.Name, filepath.Join(repoRoot, "dialtone.sh"), filepath.ToSlash(filepath.Join(targetRepo, "dialtone.sh")), cmdOpts); err != nil {
			return fmt.Errorf("nix-diagnostic sync dialtone.sh failed: %w", err)
		}
		if _, err := ssh_plugin.RunNodeCommand(node.Name, "chmod +x "+shellSingleQuote(filepath.ToSlash(filepath.Join(targetRepo, "dialtone.sh"))), cmdOpts); err != nil {
			return fmt.Errorf("nix-diagnostic chmod dialtone.sh failed: %w", err)
		}
	}
	checks := []struct {
		name string
		cmd  string
	}{
		{name: "nix-version", cmd: "nix --extra-experimental-features 'nix-command flakes' --version"},
		{name: "repo-exists", cmd: "test -d " + shellSingleQuote(targetRepo) + " && echo ok"},
		{name: "flake-metadata", cmd: "nix --extra-experimental-features 'nix-command flakes' flake metadata path:" + shellSingleQuote(targetRepo)},
		{name: "develop-toolchain", cmd: "cd " + shellSingleQuote(targetRepo) + " && nix --extra-experimental-features 'nix-command flakes' develop --command bash -c 'go version && bun --version && git --version'"},
		{name: "runtime-apps-build", cmd: "cd " + shellSingleQuote(targetRepo) + " && nix --extra-experimental-features 'nix-command flakes' build .#robot-server .#camera-service .#mavlink-service .#repl-service"},
	}
	for _, check := range checks {
		out, err := ssh_plugin.RunNodeCommand(node.Name, check.cmd, cmdOpts)
		if err != nil {
			return fmt.Errorf("nix-diagnostic %s failed: %w", check.name, err)
		}
		logs.Info("robot src_v2 nix-diagnostic: %s ok: %s", check.name, strings.TrimSpace(firstLine(out)))
	}
	logs.Info("robot src_v2 nix-diagnostic completed host=%s repo=%s", node.Name, targetRepo)
	return nil
}

func runSrcV2NixGC(args []string) error {
	fs := flag.NewFlagSet("robot-src-v2-nix-gc", flag.ContinueOnError)
	host := fs.String("host", "rover", "Mesh host for nix garbage collection")
	port := fs.String("port", "", "SSH port override")
	user := fs.String("user", "", "SSH user override")
	pass := fs.String("pass", "", "SSH password override")
	if err := fs.Parse(args); err != nil {
		return err
	}
	node, err := ssh_plugin.ResolveMeshNode(strings.TrimSpace(*host))
	if err != nil {
		return err
	}
	targetUser := strings.TrimSpace(*user)
	if targetUser == "" {
		targetUser = node.User
	}
	if targetUser == "" {
		return fmt.Errorf("nix-gc requires --user or a mesh node with a default user")
	}
	cmdOpts := ssh_plugin.CommandOptions{
		User:     targetUser,
		Port:     strings.TrimSpace(*port),
		Password: strings.TrimSpace(*pass),
	}
	cmd := strings.Join([]string{
		"set -e",
		"df -h / /nix/store $HOME",
		"echo '---'",
		"rm -rf $HOME/dialtone/bin/releases",
		"find $HOME/dialtone -path '*/ui/dist' -type d -prune -exec rm -rf {} + 2>/dev/null || true",
		"nix --extra-experimental-features 'nix-command flakes' store gc || nix-collect-garbage -d",
		"echo '---'",
		"df -h / /nix/store $HOME",
	}, " && ")
	out, err := ssh_plugin.RunNodeCommand(node.Name, cmd, cmdOpts)
	if err != nil {
		return fmt.Errorf("nix-gc failed on %s: %w", node.Name, err)
	}
	logs.Raw("%s", strings.TrimSpace(out))
	logs.Info("robot src_v2 nix-gc completed host=%s user=%s", node.Name, targetUser)
	return nil
}

func firstLine(raw string) string {
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

type releaseAssetInfo struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

type manifestChannelDoc struct {
	SchemaVersion  string `json:"schema_version"`
	Name           string `json:"name"`
	Channel        string `json:"channel"`
	Repo           string `json:"repo,omitempty"`
	ReleaseVersion string `json:"release_version"`
	ManifestURL    string `json:"manifest_url"`
	ManifestSHA256 string `json:"manifest_sha256,omitempty"`
	PublishedAt    string `json:"published_at,omitempty"`
}

type releaseView struct {
	TagName string             `json:"tagName"`
	Assets  []releaseAssetInfo `json:"assets"`
}

type buildTarget struct {
	GOOS   string
	GOARCH string
}

func publishRobotSrcV2Release(repoRoot, repo, version string, targets []buildTarget, uiOnly bool) error {
	if strings.TrimSpace(repo) == "" {
		return fmt.Errorf("repo is required (owner/name)")
	}
	if len(targets) == 0 {
		return fmt.Errorf("robot src_v2 publish: no release targets selected")
	}
	srcRoot := filepath.Join(repoRoot, "src")
	outDir := filepath.Join(repoRoot, "bin", "releases", "robot_src_v2", sanitizeVersion(version))
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	goBin, err := resolveGoBinary()
	if err != nil {
		return err
	}
	if err := runDialtone(repoRoot, "robot", "src_v2", "build"); err != nil {
		return err
	}
	uiDist := filepath.Join(repoRoot, "src", "plugins", "robot", "src_v2", "ui", "dist")
	manifestSrc := filepath.Join(repoRoot, "src", "plugins", "robot", "src_v2", "config", "composition.manifest.json")

	specs := []struct {
		AssetPrefix string
		MainPath    string
	}{
		{AssetPrefix: "dialtone_autoswap", MainPath: "./plugins/autoswap/src_v1/cmd/main.go"},
		{AssetPrefix: "dialtone_robot_v2", MainPath: "./plugins/robot/src_v2/cmd/server/main.go"},
		{AssetPrefix: "dialtone_camera_v1", MainPath: "./plugins/camera/src_v1/cmd/main.go"},
		{AssetPrefix: "dialtone_mavlink_v1", MainPath: "./plugins/mavlink/src_v1/cmd/main.go"},
		{AssetPrefix: "dialtone_repl", MainPath: "./plugins/repl/src_v1/cmd/repld/main.go"},
	}

	existing, exists, err := githubReleaseAssets(repo, version)
	if err != nil {
		return err
	}

	assetPathByName := map[string]string{}
	for _, t := range targets {
		if !uiOnly {
			for _, s := range specs {
				name := s.AssetPrefix + "-" + t.GOOS + "-" + t.GOARCH
				if t.GOOS == "windows" {
					name += ".exe"
				}
				out := filepath.Join(outDir, name)
				if err := buildGoBinary(goBin, srcRoot, s.MainPath, out, t.GOOS, t.GOARCH); err != nil {
					if s.AssetPrefix == "dialtone_camera_v1" && t.GOOS == "linux" && t.GOARCH == "arm64" {
						logs.Warn("robot src_v2 publish: camera cross-build failed for %s; trying camera plugin podman build fallback", name)
						if ferr := runDialtone(repoRoot, "camera", "src_v1", "build", "--goos", t.GOOS, "--goarch", t.GOARCH, "--out", out, "--podman"); ferr == nil {
							assetPathByName[name] = out
							continue
						}
					}
					logs.Warn("robot src_v2 publish: skip asset %s (%s/%s build failed: %v)", name, t.GOOS, t.GOARCH, err)
					continue
				}
				assetPathByName[name] = out
			}
		}
		uiName := "robot_src_v2_ui_dist-" + t.GOOS + "-" + t.GOARCH + ".tar.gz"
		uiArchive := filepath.Join(outDir, uiName)
		if err := createTarGzFromDir(uiArchive, uiDist); err != nil {
			return err
		}
		assetPathByName[uiName] = uiArchive
	}
	manifestAssetName := "robot_src_v2_composition_manifest.json"
	manifestVersionedAssetName := "robot_src_v2_composition_manifest-" + sanitizeVersion(version) + ".json"
	manifestAssetPath := filepath.Join(outDir, manifestAssetName)
	manifestVersionedAssetPath := filepath.Join(outDir, manifestVersionedAssetName)
	manifestRaw, err := os.ReadFile(manifestSrc)
	if err != nil {
		return fmt.Errorf("robot src_v2 publish: read manifest failed: %w", err)
	}
	var manifestDoc map[string]any
	if err := json.Unmarshal(manifestRaw, &manifestDoc); err != nil {
		return fmt.Errorf("robot src_v2 publish: parse manifest failed: %w", err)
	}
	assetSHA := map[string]string{}
	for name, digest := range existing {
		d := strings.TrimSpace(strings.TrimPrefix(digest, "sha256:"))
		if d != "" {
			assetSHA[name] = d
		}
	}
	for name, p := range assetPathByName {
		sum, serr := fileSHA256(p)
		if serr != nil {
			return fmt.Errorf("robot src_v2 publish: asset sha failed for %s: %w", name, serr)
		}
		assetSHA[name] = sum
	}
	manifestDoc["release_version"] = strings.TrimSpace(version)
	manifestDoc["release_published_at"] = time.Now().UTC().Format(time.RFC3339)
	manifestDoc["release_asset_sha256"] = assetSHA
	manifestDoc["manifest_asset"] = manifestVersionedAssetName
	if artifactsRaw, ok := manifestDoc["artifacts"].(map[string]any); ok {
		if releaseRaw, ok := artifactsRaw["release"].(map[string]any); ok {
			for depKey, bindingRaw := range releaseRaw {
				binding, ok := bindingRaw.(map[string]any)
				if !ok {
					continue
				}
				assetTpl, _ := binding["asset"].(string)
				assetTpl = strings.TrimSpace(assetTpl)
				if assetTpl == "" {
					continue
				}
				byTarget := map[string]string{}
				for _, t := range targets {
					targetKey := t.GOOS + "-" + t.GOARCH
					assetName := renderReleaseAssetTemplate(assetTpl, t.GOOS, t.GOARCH)
					if sha, ok := assetSHA[assetName]; ok && strings.TrimSpace(sha) != "" {
						byTarget[targetKey] = sha
					}
				}
				if len(byTarget) == 0 {
					continue
				}
				binding["sha256_by_target"] = byTarget
				hostKey := runtime.GOOS + "-" + runtime.GOARCH
				if v, ok := byTarget[hostKey]; ok {
					binding["sha256"] = v
				}
				releaseRaw[depKey] = binding
			}
			artifactsRaw["release"] = releaseRaw
		}
		manifestDoc["artifacts"] = artifactsRaw
	}
	manifestOut, err := json.MarshalIndent(manifestDoc, "", "  ")
	if err != nil {
		return fmt.Errorf("robot src_v2 publish: marshal manifest failed: %w", err)
	}
	manifestOut = append(manifestOut, '\n')
	if err := os.WriteFile(manifestAssetPath, manifestOut, 0o644); err != nil {
		return fmt.Errorf("robot src_v2 publish: write manifest asset failed: %w", err)
	}
	if err := os.WriteFile(manifestVersionedAssetPath, manifestOut, 0o644); err != nil {
		return fmt.Errorf("robot src_v2 publish: write versioned manifest asset failed: %w", err)
	}
	assetPathByName[manifestAssetName] = manifestAssetPath
	assetPathByName[manifestVersionedAssetName] = manifestVersionedAssetPath
	manifestDigest, err := fileSHA256(manifestVersionedAssetPath)
	if err != nil {
		return fmt.Errorf("robot src_v2 publish: manifest sha failed: %w", err)
	}
	channelAssetName := "robot_src_v2_channel.json"
	channelAssetPath := filepath.Join(outDir, channelAssetName)
	channelDoc := manifestChannelDoc{
		SchemaVersion:  "v1",
		Name:           "robot-src_v2",
		Channel:        "latest",
		Repo:           strings.TrimSpace(repo),
		ReleaseVersion: strings.TrimSpace(version),
		ManifestURL:    fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", strings.TrimSpace(repo), strings.TrimSpace(version), manifestVersionedAssetName),
		ManifestSHA256: manifestDigest,
		PublishedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	channelRaw, err := json.MarshalIndent(channelDoc, "", "  ")
	if err != nil {
		return fmt.Errorf("robot src_v2 publish: marshal channel asset failed: %w", err)
	}
	channelRaw = append(channelRaw, '\n')
	if err := os.WriteFile(channelAssetPath, channelRaw, 0o644); err != nil {
		return fmt.Errorf("robot src_v2 publish: write channel asset failed: %w", err)
	}
	assetPathByName[channelAssetName] = channelAssetPath

	if len(assetPathByName) == 0 {
		return fmt.Errorf("robot src_v2 publish: no release assets were built")
	}

	needsUpload := make([]string, 0, len(assetPathByName))
	for name, localPath := range assetPathByName {
		remoteDigest, ok := existing[name]
		if !ok {
			needsUpload = append(needsUpload, name)
			continue
		}
		localDigest, derr := fileSHA256(localPath)
		if derr != nil {
			return fmt.Errorf("robot src_v2 publish: digest failed for %s: %w", name, derr)
		}
		remoteDigest = strings.TrimSpace(strings.TrimPrefix(remoteDigest, "sha256:"))
		if remoteDigest == "" || !strings.EqualFold(remoteDigest, localDigest) {
			needsUpload = append(needsUpload, name)
		}
	}
	sort.Strings(needsUpload)
	if len(needsUpload) == 0 {
		logs.Info("robot src_v2 publish: release %s already has all required assets with matching digests; skipping upload", version)
		return nil
	}

	gh, err := resolveGHCli()
	if err != nil {
		return err
	}
	assetPaths := make([]string, 0, len(needsUpload))
	for _, name := range needsUpload {
		assetPaths = append(assetPaths, assetPathByName[name])
	}
	if !exists {
		args := []string{"release", "create", version, "--repo", repo, "--title", "Robot src_v2 " + version, "--notes", "Automated robot src_v2 publish " + version}
		args = append(args, assetPaths...)
		cmd := exec.Command(gh, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
		logs.Info("robot src_v2 publish: created release %s with %d assets", version, len(assetPaths))
		return nil
	}

	args := []string{"release", "upload", version, "--repo", repo, "--clobber"}
	args = append(args, assetPaths...)
	cmd := exec.Command(gh, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	logs.Info("robot src_v2 publish: uploaded %d changed/missing assets to %s", len(assetPaths), version)
	return nil
}

func resolvePublishTargets(target string, all bool) ([]buildTarget, error) {
	if all {
		return []buildTarget{
			{GOOS: "linux", GOARCH: "amd64"},
			{GOOS: "linux", GOARCH: "arm64"},
			{GOOS: "darwin", GOARCH: "amd64"},
			{GOOS: "darwin", GOARCH: "arm64"},
			{GOOS: "windows", GOARCH: "amd64"},
		}, nil
	}
	v := strings.TrimSpace(strings.ToLower(target))
	if v == "" {
		v = "linux-arm64"
	}
	parts := strings.Split(v, "-")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return nil, fmt.Errorf("invalid --target %q (expected <goos>-<goarch>, e.g. linux-arm64)", target)
	}
	return []buildTarget{{GOOS: strings.TrimSpace(parts[0]), GOARCH: strings.TrimSpace(parts[1])}}, nil
}

func resolveRobotPublishVersion(repoRoot, requested string) (string, error) {
	if strings.TrimSpace(requested) != "" {
		return strings.TrimSpace(requested), nil
	}
	if v := strings.TrimSpace(os.Getenv("ROBOT_SRC_V2_PUBLISH_VERSION")); v != "" {
		return v, nil
	}
	tagCmd := exec.Command("git", "describe", "--tags", "--exact-match")
	tagCmd.Dir = repoRoot
	if out, err := tagCmd.CombinedOutput(); err == nil {
		v := strings.TrimSpace(string(out))
		if v != "" {
			return v, nil
		}
	}
	shaCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	shaCmd.Dir = repoRoot
	out, err := shaCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resolve publish version failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	sha := strings.TrimSpace(string(out))
	if sha == "" {
		return "", fmt.Errorf("resolve publish version failed: empty git sha")
	}
	return "robot-src-v2-" + sha, nil
}

func resolveGoBinary() (string, error) {
	candidate := filepath.Join(logs.GetDialtoneEnv(), "go", "bin", "go")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return exec.LookPath("go")
}

func renderReleaseAssetTemplate(raw, goos, goarch string) string {
	v := strings.TrimSpace(raw)
	v = strings.ReplaceAll(v, "${goos}", goos)
	v = strings.ReplaceAll(v, "${goarch}", goarch)
	v = strings.ReplaceAll(v, "<goos>", goos)
	v = strings.ReplaceAll(v, "<goarch>", goarch)
	return v
}

func buildGoBinary(goBin, srcRoot, mainPath, out, goos, goarch string) error {
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	cmd := exec.Command(goBin, "build", "-o", out, mainPath)
	cmd.Dir = srcRoot
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func createTarGzFromDir(outFile, srcDir string) error {
	if err := os.MkdirAll(filepath.Dir(outFile), 0o755); err != nil {
		return err
	}
	f, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer f.Close()
	gzw := gzip.NewWriter(f)
	defer gzw.Close()
	tw := tar.NewWriter(gzw)
	defer tw.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		h, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		h.Name = rel
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(tw, in)
		return err
	})
}

func githubReleaseAssets(repo, version string) (map[string]string, bool, error) {
	gh, err := resolveGHCli()
	if err != nil {
		return nil, false, err
	}
	cmd := exec.Command(gh, "release", "view", version, "--repo", repo, "--json", "tagName,assets")
	out, err := cmd.CombinedOutput()
	if err != nil {
		lower := strings.ToLower(string(out))
		if strings.Contains(lower, "not found") || strings.Contains(lower, "no release found") {
			return map[string]string{}, false, nil
		}
		return nil, false, fmt.Errorf("gh release view failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	var rv releaseView
	if err := json.Unmarshal(out, &rv); err != nil {
		return nil, false, err
	}
	m := map[string]string{}
	for _, a := range rv.Assets {
		m[strings.TrimSpace(a.Name)] = strings.TrimSpace(a.Digest)
	}
	return m, true, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func resolveGHCli() (string, error) {
	if p, err := exec.LookPath("gh"); err == nil {
		return p, nil
	}
	candidate := filepath.Join(logs.GetDialtoneEnv(), "gh", "bin", "gh")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return "", fmt.Errorf("gh cli not found; run ./dialtone.sh github src_v1 install")
}

func sanitizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, "/", "-")
	v = strings.ReplaceAll(v, "\\", "-")
	v = strings.ReplaceAll(v, " ", "-")
	if v == "" {
		return time.Now().UTC().Format("20060102-150405")
	}
	return v
}

func runSrcV2Diagnostic(repoRoot string, args []string) error {
	fs := flag.NewFlagSet("robot-src-v2-diagnostic", flag.ContinueOnError)
	host := fs.String("host", strings.TrimSpace(os.Getenv("ROBOT_HOST")), "Robot SSH host")
	port := fs.String("port", "22", "Robot SSH port")
	user := fs.String("user", strings.TrimSpace(os.Getenv("ROBOT_USER")), "Robot SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "Robot SSH password")
	remoteRepo := fs.String("remote-repo", "", "Remote repo root (default: <remote-home>/dialtone)")
	manifest := fs.String("manifest", "src/plugins/robot/src_v2/config/composition.manifest.json", "Remote manifest path (absolute or repo-relative)")
	uiURL := fs.String("ui-url", "", "Robot UI URL for public checks + browser checks (default: https://<robot-hostname>.dialtone.earth)")
	browserNode := fs.String("browser-node", defaultRobotDevBrowserNode(), "Mesh node for remote browser (for example legion, chroma)")
	skipUI := fs.Bool("skip-ui", false, "Skip chromedp UI menu checks")
	publicCheck := fs.Bool("public-check", true, "Verify public UI endpoint is reachable")
	if err := fs.Parse(args); err != nil {
		return err
	}

	required := []string{
		filepath.Join(repoRoot, "bin", "dialtone_autoswap_v1"),
		filepath.Join(repoRoot, "bin", "dialtone_robot_v2"),
		filepath.Join(repoRoot, "bin", "dialtone_camera_v1"),
		filepath.Join(repoRoot, "bin", "dialtone_mavlink_v1"),
		filepath.Join(repoRoot, "bin", "dialtone_repl_v1"),
		filepath.Join(repoRoot, "src", "plugins", "robot", "src_v2", "ui", "dist", "index.html"),
	}
	for _, p := range required {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("diagnostic missing local artifact: %s", p)
		}
	}
	logs.Info("robot src_v2 diagnostic: local artifact check passed")

	if strings.TrimSpace(*host) == "" || strings.TrimSpace(*user) == "" {
		logs.Warn("robot src_v2 diagnostic: no --host/--user provided; skipped remote checks")
		return nil
	}

	targetHost := strings.TrimSpace(*host)
	targetUser := strings.TrimSpace(*user)
	targetPort := strings.TrimSpace(*port)
	targetPass := *pass
	var meshNode *ssh_plugin.MeshNode
	client, node, _, _, meshErr := ssh_plugin.DialMeshNode(targetHost, ssh_plugin.CommandOptions{
		User:     targetUser,
		Port:     targetPort,
		Password: targetPass,
	})
	if meshErr != nil {
		// Fallback to direct host dial for non-mesh targets.
		directClient, err := ssh_plugin.DialSSH(targetHost, targetPort, targetUser, targetPass)
		if err != nil {
			return err
		}
		client = directClient
	} else {
		nodeCopy := node
		meshNode = &nodeCopy
		if strings.TrimSpace(*user) == "" {
			*user = node.User
		}
	}
	defer client.Close()

	remoteHomeOut, err := ssh_plugin.RunSSHCommand(client, "printf '%s' \"$HOME\"")
	if err != nil {
		return fmt.Errorf("remote home lookup failed: %w", err)
	}
	remoteHome := strings.TrimSpace(remoteHomeOut)
	if remoteHome == "" {
		return fmt.Errorf("remote home lookup returned empty value")
	}
	resolvedRemoteRepo := strings.TrimSpace(*remoteRepo)
	if resolvedRemoteRepo == "" {
		resolvedRemoteRepo = filepath.ToSlash(filepath.Join(remoteHome, "dialtone"))
	}
	autoswapRoot := filepath.ToSlash(filepath.Join(remoteHome, ".dialtone", "autoswap"))
	manifestAbs := resolveRemoteManifestPath(resolvedRemoteRepo, strings.TrimSpace(*manifest))
	remoteExecutableExists := func(path string) bool {
		if strings.TrimSpace(path) == "" {
			return false
		}
		_, err := ssh_plugin.RunSSHCommand(client, "test -x "+shellSingleQuote(path))
		return err == nil
	}
	remoteFileExists := func(path string) bool {
		if strings.TrimSpace(path) == "" {
			return false
		}
		_, err := ssh_plugin.RunSSHCommand(client, "test -f "+shellSingleQuote(path))
		return err == nil
	}
	selectExecutable := func(candidates []string) (string, error) {
		for _, c := range candidates {
			c = filepath.ToSlash(strings.TrimSpace(c))
			if remoteExecutableExists(c) {
				return c, nil
			}
		}
		return "", fmt.Errorf("no executable candidate exists: %v", candidates)
	}
	selectFile := func(candidates []string) (string, error) {
		for _, c := range candidates {
			c = filepath.ToSlash(strings.TrimSpace(c))
			if remoteFileExists(c) {
				return c, nil
			}
		}
		return "", fmt.Errorf("no file candidate exists: %v", candidates)
	}

	autoswapBin, err := selectExecutable([]string{
		filepath.Join(resolvedRemoteRepo, "bin", "dialtone_autoswap_v1"),
		filepath.Join(autoswapRoot, "bin", "dialtone_autoswap_v1"),
	})
	if err != nil {
		return fmt.Errorf("diagnostic remote autoswap binary check failed: %w", err)
	}
	if _, err := selectExecutable([]string{
		filepath.Join(resolvedRemoteRepo, "bin", "dialtone_robot_v2"),
		filepath.Join(autoswapRoot, "artifacts", "dialtone_robot_v2"),
	}); err != nil {
		return fmt.Errorf("diagnostic remote robot binary check failed: %w", err)
	}
	if _, err := selectExecutable([]string{
		filepath.Join(resolvedRemoteRepo, "bin", "dialtone_camera_v1"),
		filepath.Join(autoswapRoot, "artifacts", "dialtone_camera_v1"),
	}); err != nil {
		return fmt.Errorf("diagnostic remote camera binary check failed: %w", err)
	}
	if _, err := selectExecutable([]string{
		filepath.Join(resolvedRemoteRepo, "bin", "dialtone_mavlink_v1"),
		filepath.Join(autoswapRoot, "artifacts", "dialtone_mavlink_v1"),
	}); err != nil {
		return fmt.Errorf("diagnostic remote mavlink binary check failed: %w", err)
	}
	if _, err := selectExecutable([]string{
		filepath.Join(resolvedRemoteRepo, "bin", "dialtone_repl_v1"),
		filepath.Join(autoswapRoot, "artifacts", "dialtone_repl_v1"),
	}); err != nil {
		return fmt.Errorf("diagnostic remote repl binary check failed: %w", err)
	}
	if _, err := selectFile([]string{
		filepath.Join(resolvedRemoteRepo, "src", "plugins", "robot", "src_v2", "ui", "dist", "index.html"),
		filepath.Join(autoswapRoot, "artifacts", "robot_src_v2_ui_dist", "index.html"),
	}); err != nil {
		return fmt.Errorf("diagnostic remote ui dist check failed: %w", err)
	}
	if !remoteFileExists(manifestAbs) {
		candidates := []string{
			filepath.ToSlash(filepath.Join(autoswapRoot, "manifests", "robot-src_v2.manifest.json")),
		}
		found := ""
		for _, c := range candidates {
			if remoteFileExists(c) {
				found = c
				break
			}
		}
		if found == "" {
			manifestDir := filepath.ToSlash(filepath.Join(autoswapRoot, "manifests"))
			latestManifestOut, lerr := ssh_plugin.RunSSHCommand(client, "find "+shellSingleQuote(manifestDir)+" -maxdepth 1 -type f -name 'manifest-*.json' -printf '%T@ %p\\n' 2>/dev/null | sort -nr | head -n1 | awk '{print $2}'")
			if lerr == nil {
				latestManifest := strings.TrimSpace(latestManifestOut)
				if latestManifest != "" && remoteFileExists(latestManifest) {
					found = latestManifest
				}
			}
		}
		if found == "" {
			return fmt.Errorf("diagnostic remote manifest check failed: %s", manifestAbs)
		}
		manifestAbs = found
	}
	logs.Info("robot src_v2 diagnostic: remote artifact check passed")

	activeOut, err := ssh_plugin.RunSSHCommand(client, "systemctl --user is-active dialtone_autoswap.service")
	if err != nil {
		return fmt.Errorf("autoswap service active check failed: %w", err)
	}
	if strings.TrimSpace(activeOut) != "active" {
		return fmt.Errorf("autoswap service is not active: %s", strings.TrimSpace(activeOut))
	}

	execOut, err := ssh_plugin.RunSSHCommand(client, "systemctl --user show dialtone_autoswap.service --property=ExecStart --no-pager")
	if err != nil {
		return fmt.Errorf("autoswap service ExecStart check failed: %w", err)
	}
	manifestURL := strings.TrimSpace(extractFlagValue(execOut, "--manifest-url"))
	if !strings.Contains(execOut, "dialtone_autoswap_v1") {
		return fmt.Errorf("autoswap service ExecStart does not reference dialtone_autoswap_v1")
	}
	if !strings.Contains(execOut, manifestAbs) && !strings.Contains(execOut, "--manifest-url") {
		return fmt.Errorf("autoswap service ExecStart does not reference manifest path %s or --manifest-url", manifestAbs)
	}
	logs.Info("robot src_v2 diagnostic: autoswap service is active and uses expected manifest")

	repoRootForList := ""
	if _, err := ssh_plugin.RunSSHCommand(client, "test -d "+shellSingleQuote(resolvedRemoteRepo)); err == nil {
		repoRootForList = resolvedRemoteRepo
	}
	runtimePath := filepath.ToSlash(filepath.Join(remoteHome, ".dialtone", "autoswap", "state", "runtime.json"))
	runtimeRaw, err := ssh_plugin.RunSSHCommand(client, "cat "+shellSingleQuote(runtimePath))
	if err != nil {
		return fmt.Errorf("autoswap runtime state read failed: %w", err)
	}
	var runtimeState struct {
		ManifestPath string `json:"manifest_path"`
		Processes    []struct {
			Name   string `json:"name"`
			PID    int    `json:"pid"`
			Status string `json:"status"`
		} `json:"processes"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(runtimeRaw)), &runtimeState); err != nil {
		return fmt.Errorf("autoswap runtime state parse failed: %w", err)
	}
	if rp := strings.TrimSpace(runtimeState.ManifestPath); rp != "" {
		if remoteFileExists(rp) {
			manifestAbs = filepath.ToSlash(rp)
			logs.Info("robot src_v2 diagnostic: using runtime manifest path from state: %s", manifestAbs)
		}
	} else if manifestURL != "" {
		// When using --manifest-url, the manifest is typically downloaded under autoswap/manifests/.
		// Prefer the explicit runtime manifest path when present; fail only if missing.
		candidates := []string{
			filepath.ToSlash(filepath.Join(autoswapRoot, "manifests", "robot-src_v2.manifest.json")),
			filepath.ToSlash(filepath.Join(autoswapRoot, "manifests")),
		}
		found := ""
		for _, c := range candidates {
			if c == filepath.ToSlash(filepath.Join(autoswapRoot, "manifests")) {
				if latestManifestOut, lerr := ssh_plugin.RunSSHCommand(client, "find "+shellSingleQuote(c)+" -maxdepth 1 -type f -name 'manifest-*.json' -printf '%T@ %p\\n' 2>/dev/null | sort -nr | head -n1 | awk '{print $2}'"); lerr == nil {
					latestManifest := strings.TrimSpace(latestManifestOut)
					if latestManifest != "" && remoteFileExists(latestManifest) {
						found = latestManifest
						break
					}
				}
				continue
			}
			if remoteFileExists(c) {
				found = c
				break
			}
		}
		if found == "" {
			return fmt.Errorf("diagnostic could not resolve active autoswap manifest for --manifest-url mode")
		}
		manifestAbs = found
	}
	listCmd := shellSingleQuote(autoswapBin) + " service --mode list --manifest " + shellSingleQuote(manifestAbs)
	if strings.TrimSpace(repoRootForList) != "" {
		listCmd += " --repo-root " + shellSingleQuote(repoRootForList)
	}
	listOut, err := ssh_plugin.RunSSHCommand(client, listCmd)
	if err != nil {
		return fmt.Errorf("autoswap service --mode list failed: %w", err)
	}
	for _, token := range []string{"runtime", "supervisor"} {
		if !strings.Contains(strings.ToLower(listOut), token) {
			return fmt.Errorf("autoswap list output missing expected token %q", token)
		}
	}
	logs.Info("robot src_v2 diagnostic: autoswap list output looks valid")

	manifestRaw, err := ssh_plugin.RunSSHCommand(client, "cat "+shellSingleQuote(manifestAbs))
	if err != nil {
		return fmt.Errorf("autoswap manifest read failed: %w", err)
	}
	var manifestState struct {
		Runtime struct {
			Processes []struct {
				Name string `json:"name"`
			} `json:"processes"`
		} `json:"runtime"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(manifestRaw)), &manifestState); err != nil {
		return fmt.Errorf("autoswap manifest parse failed: %w", err)
	}
	expectedProc := make(map[string]bool)
	for _, p := range manifestState.Runtime.Processes {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		expectedProc[name] = false
	}
	if len(expectedProc) == 0 {
		return fmt.Errorf("manifest has no runtime.processes entries")
	}
	if manifestURL != "" {
		if strings.TrimSpace(runtimeState.ManifestPath) == "" {
			return fmt.Errorf("active autoswap manifest is empty while service uses --manifest-url")
		}
		if strings.TrimSpace(manifestAbs) == "" || filepath.Clean(runtimeState.ManifestPath) != filepath.Clean(manifestAbs) {
			logs.Info("robot src_v2 diagnostic: active manifest path resolved from runtime state: %s", strings.TrimSpace(runtimeState.ManifestPath))
			manifestAbs = filepath.ToSlash(strings.TrimSpace(runtimeState.ManifestPath))
		}
	}
	if filepath.Clean(runtimeState.ManifestPath) != filepath.Clean(manifestAbs) {
		if strings.Contains(execOut, "--manifest-url") {
			if strings.TrimSpace(runtimeState.ManifestPath) == "" {
				return fmt.Errorf("active autoswap manifest is empty while service uses --manifest-url")
			}
			logs.Info("robot src_v2 diagnostic: active manifest path is %s, using this for checks", strings.TrimSpace(runtimeState.ManifestPath))
		} else {
			return fmt.Errorf("active autoswap manifest mismatch: got=%s expected=%s", runtimeState.ManifestPath, manifestAbs)
		}
	}
	for _, p := range runtimeState.Processes {
		name := strings.TrimSpace(p.Name)
		if _, ok := expectedProc[name]; !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(p.Status), "running") && p.PID > 0 {
			expectedProc[name] = true
		}
	}
	for name, ok := range expectedProc {
		if !ok {
			return fmt.Errorf("autoswap runtime state process is not running: %s", name)
		}
	}
	logs.Info("robot src_v2 diagnostic: autoswap runtime matches manifest processes")

	if manifestURL != "" {
		remoteManifestHashOut, err := ssh_plugin.RunSSHCommand(client, "sha256sum "+shellSingleQuote(manifestAbs)+" | awk '{print $1}'")
		if err != nil {
			return fmt.Errorf("active manifest hash read failed: %w", err)
		}
		remoteManifestHash := strings.TrimSpace(remoteManifestHashOut)
		latestManifestHash, err := fetchResolvedManifestSHA256(manifestURL, 15*time.Second)
		if err != nil {
			return fmt.Errorf("latest manifest fetch failed (%s): %w", manifestURL, err)
		}
		if !strings.EqualFold(remoteManifestHash, latestManifestHash) {
			return fmt.Errorf("active manifest is not latest from manifest-url: active=%s latest=%s", remoteManifestHash, latestManifestHash)
		}
		logs.Info("robot src_v2 diagnostic: active manifest hash matches latest manifest-url")
	}

	pidArgs := make([]string, 0, len(runtimeState.Processes))
	for _, p := range runtimeState.Processes {
		if p.PID <= 0 {
			continue
		}
		pidArgs = append(pidArgs, strconv.Itoa(p.PID))
	}
	if len(pidArgs) == 0 {
		return fmt.Errorf("runtime state has no managed process pids")
	}
	procsOut := ""
	foundAllProcTokens := false
	for attempt := 0; attempt < 6; attempt++ {
		nextOut, perr := ssh_plugin.RunSSHCommand(client, "ps -p "+strings.Join(pidArgs, ",")+" -o pid= -o args= || true")
		if perr != nil {
			return fmt.Errorf("remote process list failed: %w", perr)
		}
		procsOut = nextOut
		foundAllProcTokens = true
		for _, pid := range pidArgs {
			if !strings.Contains(procsOut, pid) {
				foundAllProcTokens = false
				break
			}
		}
		if foundAllProcTokens {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if !foundAllProcTokens {
		return fmt.Errorf("remote process list missing expected managed pids: %s", strings.TrimSpace(procsOut))
	}

	healthOut, err := ssh_plugin.RunSSHCommand(client, "curl -fsS --max-time 5 http://127.0.0.1:18086/health")
	if err != nil {
		return fmt.Errorf("remote /health check failed: %w", err)
	}
	if strings.TrimSpace(healthOut) != "ok" {
		return fmt.Errorf("remote /health expected ok, got %q", strings.TrimSpace(healthOut))
	}
	initOut, err := ssh_plugin.RunSSHCommand(client, "curl -fsS --max-time 5 http://127.0.0.1:18086/api/init")
	if err != nil {
		return fmt.Errorf("remote /api/init check failed: %w", err)
	}
	if !strings.Contains(initOut, "/natsws") {
		return fmt.Errorf("remote /api/init missing /natsws")
	}
	integOut, err := ssh_plugin.RunSSHCommand(client, "curl -fsS --max-time 5 http://127.0.0.1:18086/api/integration-health")
	if err != nil {
		return fmt.Errorf("remote /api/integration-health check failed: %w", err)
	}
	if !strings.Contains(integOut, "\"camera\":{\"status\":\"configured\"}") || !strings.Contains(integOut, "\"mavlink\":{\"status\":\"configured\"}") {
		return fmt.Errorf("remote /api/integration-health missing configured camera/mavlink")
	}
	streamCodeOut, err := ssh_plugin.RunSSHCommand(client, "curl -sS -I --max-time 5 -o /dev/null -w '%{http_code}' http://127.0.0.1:18086/stream")
	if err != nil {
		return fmt.Errorf("remote /stream status check failed: %w", err)
	}
	if strings.TrimSpace(streamCodeOut) != "200" {
		return fmt.Errorf("remote /stream expected HTTP 200, got %s", strings.TrimSpace(streamCodeOut))
	}
	natswsProbe, err := ssh_plugin.RunSSHCommand(client, "python3 - <<'PY'\nimport socket\n\ns = socket.create_connection(('127.0.0.1', 18086), timeout=2)\nreq = (\n    'GET /natsws HTTP/1.1\\r\\n'\n    'Host: 127.0.0.1:18086\\r\\n'\n    'Connection: Upgrade\\r\\n'\n    'Upgrade: websocket\\r\\n'\n    'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\\r\\n'\n    'Sec-WebSocket-Version: 13\\r\\n\\r\\n'\n)\ns.sendall(req.encode())\nresp = s.recv(64)\ns.close()\nprint(resp.decode(errors='replace'))\nPY")
	if err != nil {
		return fmt.Errorf("remote /natsws websocket handshake failed: %w", err)
	}
	natswsStatusLine := strings.TrimSpace(strings.SplitN(strings.TrimSpace(natswsProbe), "\n", 2)[0])
	if !strings.HasPrefix(natswsStatusLine, "HTTP/1.1 101") {
		return fmt.Errorf("remote /natsws expected websocket upgrade 101, got %s", natswsStatusLine)
	}
	cameraSidecarHealth, err := ssh_plugin.RunSSHCommand(client, "curl -fsS --max-time 5 http://127.0.0.1:19090/health")
	if err != nil {
		return fmt.Errorf("remote camera sidecar /health check failed: %w", err)
	}
	if strings.TrimSpace(cameraSidecarHealth) != "ok" {
		return fmt.Errorf("remote camera sidecar /health expected ok, got %q", strings.TrimSpace(cameraSidecarHealth))
	}
	streamProbeOut, err := ssh_plugin.RunSSHCommand(client, "python3 - <<'PY'\nimport urllib.request\nreq=urllib.request.Request('http://127.0.0.1:19090/stream')\nwith urllib.request.urlopen(req, timeout=8) as r:\n    ct=(r.headers.get('Content-Type') or '').lower()\n    chunk=r.read(2048)\nok='multipart/x-mixed-replace' in ct and b'--frame' in chunk\nprint('ok' if ok else 'bad')\nPY")
	if err != nil {
		return fmt.Errorf("remote camera stream payload probe failed: %w", err)
	}
	if strings.TrimSpace(streamProbeOut) != "ok" {
		return fmt.Errorf("remote camera stream payload probe did not return multipart frame boundary")
	}
	mavlinkLiveProbe, err := ssh_plugin.RunSSHCommand(client, "journalctl --user -u dialtone_autoswap.service --since '2 minutes ago' --no-pager | egrep '\\[MAVLINK-RAW\\] (HEARTBEAT|GLOBALPOSITIONINT)' | tail -n 8")
	if err != nil {
		return fmt.Errorf("remote mavlink telemetry liveness check failed: %w", err)
	}
	if !strings.Contains(mavlinkLiveProbe, "MAVLINK-RAW") {
		return fmt.Errorf("remote mavlink telemetry liveness check found no recent MAVLINK-RAW HEARTBEAT/GLOBALPOSITIONINT")
	}
	logs.Info("robot src_v2 diagnostic: remote endpoints passed (/health, /api/init, /api/integration-health, /stream, sidecar camera stream, mavlink telemetry liveness)")

	resolvedUIURL := strings.TrimSpace(*uiURL)
	if resolvedUIURL == "" && *publicCheck {
		publicHost := inferPublicRobotHostname(strings.TrimSpace(*host), meshNode)
		resolvedUIURL = "https://" + publicHost + ".dialtone.earth"
	}
	if !*publicCheck {
		logs.Info("robot src_v2 diagnostic: skipping public UI verification (pass --public-check=true to re-enable)")
		logs.Info("robot src_v2 diagnostic: remote checks completed")
		return nil
	}
	if resolvedUIURL == "" {
		resolvedUIURL = "http://127.0.0.1:18086"
	}
	if !strings.Contains(resolvedUIURL, "://") {
		resolvedUIURL = "https://" + resolvedUIURL
	}
	uiBase := strings.TrimRight(resolvedUIURL, "/")
	publicHealthBody, err := fetchURLText(uiBase+"/health", 10*time.Second)
	if err != nil {
		return fmt.Errorf("public ui /health check failed (%s): %w", uiBase, err)
	}
	if strings.TrimSpace(publicHealthBody) != "ok" {
		return fmt.Errorf("public ui /health expected ok, got %q", strings.TrimSpace(publicHealthBody))
	}
	publicInitBody, err := fetchURLText(uiBase+"/api/init", 10*time.Second)
	if err != nil {
		return fmt.Errorf("public ui /api/init check failed (%s): %w", uiBase, err)
	}
	if !strings.Contains(publicInitBody, "/natsws") {
		return fmt.Errorf("public ui /api/init missing /natsws")
	}
	var publicInit struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(publicInitBody)), &publicInit); err != nil {
		return fmt.Errorf("public ui /api/init json parse failed: %w", err)
	}
	expectedSettingsVersion := strings.TrimSpace(publicInit.Version)
	if expectedSettingsVersion == "" {
		return fmt.Errorf("public ui /api/init returned empty version")
	}
	logs.Info("robot src_v2 diagnostic: public UI passed (%s) expected_version=%s", uiBase, expectedSettingsVersion)

	if !*skipUI {
		if err := runRobotSrcV2MenuDiagnostic(resolvedUIURL, strings.TrimSpace(*browserNode), repoRoot, expectedSettingsVersion); err != nil {
			return err
		}
		logs.Info("robot src_v2 diagnostic: UI menu checks passed (%s)", resolvedUIURL)
	}

	logs.Info("robot src_v2 diagnostic: remote checks completed")
	return nil
}

func inferPublicRobotHostname(host string, node *ssh_plugin.MeshNode) string {
	candidates := []string{strings.TrimSpace(host)}
	if node != nil {
		candidates = append(candidates, strings.TrimSpace(node.Host))
		for _, a := range node.Aliases {
			candidates = append(candidates, strings.TrimSpace(a))
		}
	}
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		c = strings.Trim(c, ".")
		if strings.Contains(c, ".") {
			if isIPv4Address(c) {
				continue
			}
			if strings.HasSuffix(c, ".shad-artichoke.ts.net") {
				base := strings.TrimSuffix(c, ".shad-artichoke.ts.net")
				base = strings.Trim(base, ".")
				if base != "" {
					return base
				}
			}
			parts := strings.Split(c, ".")
			if len(parts) > 0 && parts[0] != "" {
				return parts[0]
			}
		}
		if c != "rover" {
			return c
		}
	}
	return "rover-1"
}

func isIPv4Address(raw string) bool {
	parts := strings.Split(strings.TrimSpace(raw), ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return false
		}
		if n < 0 || n > 255 {
			return false
		}
	}
	return true
}

func fetchURLText(rawURL string, timeout time.Duration) (string, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(rawURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func fetchResolvedManifestSHA256(rawURL string, timeout time.Duration) (string, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodGet, cacheBustedLatestReleaseURL(rawURL), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	trimmed := strings.TrimSpace(string(body))
	var probe map[string]any
	if err := json.Unmarshal([]byte(trimmed), &probe); err == nil {
		if manifestURL, ok := probe["manifest_url"].(string); ok && strings.TrimSpace(manifestURL) != "" && probe["runtime"] == nil {
			if manifestSHA, ok := probe["manifest_sha256"].(string); ok {
				manifestSHA = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(manifestSHA), "sha256:"))
				if manifestSHA != "" {
					return manifestSHA, nil
				}
			}
			return fetchResolvedManifestSHA256(strings.TrimSpace(manifestURL), timeout)
		}
	}
	sum := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(sum[:]), nil
}

func cacheBustedLatestReleaseURL(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return rawURL
	}
	if !strings.EqualFold(strings.TrimSpace(u.Hostname()), "github.com") {
		return rawURL
	}
	if !strings.Contains(strings.TrimSpace(u.Path), "/releases/latest/download/") {
		return rawURL
	}
	q := u.Query()
	q.Set("dialtone_ts", strconv.FormatInt(time.Now().UnixNano(), 10))
	u.RawQuery = q.Encode()
	return u.String()
}

func extractFlagValue(execStart, flagName string) string {
	fields := strings.Fields(execStart)
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] != flagName {
			continue
		}
		v := strings.TrimSpace(fields[i+1])
		v = strings.Trim(v, "\"';")
		return v
	}
	return ""
}

func runRobotSrcV2MenuDiagnostic(uiURL, browserNode, repoRoot, expectedSettingsVersion string) error {
	reg := test_plugin.NewRegistry()
	urlBase := strings.TrimRight(strings.TrimSpace(uiURL), "/")
	if urlBase == "" {
		return fmt.Errorf("ui url is empty")
	}
	reg.Add(test_plugin.Step{
		Name:    "robot-src-v2-diagnostic-ui-menu",
		Timeout: 45 * time.Second,
		RunWithContext: func(ctx *test_plugin.StepContext) (test_plugin.StepRunResult, error) {
			opts := test_plugin.BrowserOptions{
				Headless:   true,
				GPU:        true,
				Role:       "test",
				RemoteNode: strings.TrimSpace(browserNode),
				URL:        "about:blank",
			}
			ctx.Infof("[ACTION] ensure browser role=%s remote_node=%s url=%s", opts.Role, opts.RemoteNode, opts.URL)
			if _, err := ctx.EnsureBrowser(opts); err != nil {
				return test_plugin.StepRunResult{}, err
			}
			ctx.Infof("[ACTION] browser ready")
			if err := ctx.RunBrowser(chromedp.Navigate(urlBase + "/#hero")); err != nil {
				return test_plugin.StepRunResult{}, fmt.Errorf("navigate robot ui: %w", err)
			}
			ctx.Infof("[ACTION] navigated to robot ui")
			if err := ctx.WaitForAriaLabel("Hero Section", 8*time.Second); err != nil {
				return test_plugin.StepRunResult{}, err
			}
			ctx.Infof("[ACTION] hero section visible")
			if err := ctx.WaitForAriaLabelAttrEquals("Hero Section", "data-active", "true", 8*time.Second); err != nil {
				return test_plugin.StepRunResult{}, err
			}
			ctx.Infof("[ACTION] hero section active")
			if err := ctx.WaitForAriaLabel("Toggle Global Menu", 8*time.Second); err != nil {
				return test_plugin.StepRunResult{}, err
			}
			ctx.Infof("[ACTION] menu toggle visible")
			if err := ctx.ClickAriaLabel("Toggle Global Menu"); err != nil {
				return test_plugin.StepRunResult{}, err
			}
			ctx.Infof("[ACTION] menu opened")
			if err := ctx.WaitForAriaLabel("Navigate Settings", 8*time.Second); err != nil {
				return test_plugin.StepRunResult{}, err
			}
			ctx.Infof("[ACTION] settings nav visible")
			if err := ctx.ClickAriaLabel("Navigate Settings"); err != nil {
				return test_plugin.StepRunResult{}, err
			}
			ctx.Infof("[ACTION] settings nav clicked")
			if err := ctx.WaitForAriaLabel("Settings Section", 8*time.Second); err != nil {
				return test_plugin.StepRunResult{}, err
			}
			ctx.Infof("[ACTION] settings section visible")
			if err := ctx.WaitForAriaLabelAttrEquals("Settings Section", "data-active", "true", 8*time.Second); err != nil {
				return test_plugin.StepRunResult{}, err
			}
			ctx.Infof("[ACTION] settings section active")
			expectedPrefix := "version:" + strings.TrimSpace(expectedSettingsVersion)
			if err := waitForBrowserJSCondition(
				ctx,
				12*time.Second,
				fmt.Sprintf(`(() => {
				  const section = document.querySelector("[aria-label='Settings Section']");
				  const byAria = document.querySelector("button[aria-label='Robot Version Button']");
				  const byText = section ? Array.from(section.querySelectorAll("button")).find((b) => /^version:\S+/i.test((b.textContent || "").trim())) : null;
				  const btn = byAria || byText;
				  if (!(btn instanceof HTMLButtonElement)) return false;
				  const text = (btn.textContent || "").trim();
				  return text.startsWith(%q);
				})()`, expectedPrefix),
				"settings section version button did not converge to backend version",
			); err != nil {
				var debugInfo string
				_ = ctx.RunBrowser(chromedp.Evaluate(`(() => {
				  const section = document.querySelector("[aria-label='Settings Section']");
				  const active = section ? section.getAttribute("data-active") : "";
				  const buttons = Array.from(document.querySelectorAll("[aria-label='Settings Section'] button")).map((b) => ({
				    text: (b.textContent || "").trim(),
				    aria: b.getAttribute("aria-label") || ""
				  }));
				  const allButtons = Array.from(document.querySelectorAll("button")).slice(0, 12).map((b) => ({
				    text: (b.textContent || "").trim(),
				    aria: b.getAttribute("aria-label") || ""
				  }));
				  return JSON.stringify({ active, buttons, allButtons });
				})()`, &debugInfo))
				return test_plugin.StepRunResult{}, fmt.Errorf("%w; debug=%s", err, strings.TrimSpace(debugInfo))
			}
			var settingsVersionText string
			if err := ctx.RunBrowser(chromedp.Evaluate(`(() => {
			  const section = document.querySelector("[aria-label='Settings Section']");
			  const byAria = document.querySelector("button[aria-label='Robot Version Button']");
			  const byText = section ? Array.from(section.querySelectorAll("button")).find((b) => /^version:\S+/i.test((b.textContent || "").trim())) : null;
			  const btn = byAria || byText;
			  if (!(btn instanceof HTMLButtonElement)) return "";
			  return (btn.textContent || "").trim();
			})()`, &settingsVersionText)); err != nil {
				return test_plugin.StepRunResult{}, err
			}
			settingsVersionText = strings.TrimSpace(settingsVersionText)
			if settingsVersionText == "" {
				return test_plugin.StepRunResult{}, fmt.Errorf("settings version button text is empty")
			}
			if !strings.HasPrefix(settingsVersionText, expectedPrefix) {
				return test_plugin.StepRunResult{}, fmt.Errorf("settings version button mismatch: got=%q expected_prefix=%q", settingsVersionText, expectedPrefix)
			}
			ctx.Infof("[ACTION] settings version button text: %s", settingsVersionText)
			return test_plugin.StepRunResult{Report: "diagnostic settings version button passed"}, nil
		},
	})
	return reg.Run(test_plugin.SuiteOptions{
		Version:       "robot-src-v2-diagnostic-ui",
		RepoRoot:      repoRoot,
		ReportPath:    "plugins/robot/src_v2/test/DIAGNOSTIC_TEST.md",
		NATSURL:       "nats://127.0.0.1:4222",
		NATSSubject:   "logs.test.robot-src-v2-diagnostic-ui",
		AutoStartNATS: true,
	})
}

func waitForBrowserJSCondition(ctx *test_plugin.StepContext, timeout time.Duration, expr string, timeoutMsg string) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var ok bool
		if err := ctx.RunBrowser(chromedp.Evaluate(expr, &ok)); err == nil && ok {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	if strings.TrimSpace(timeoutMsg) == "" {
		timeoutMsg = "browser condition timed out"
	}
	return fmt.Errorf("%s", timeoutMsg)
}

func resolveRemoteManifestPath(remoteRepo, manifest string) string {
	m := strings.TrimSpace(manifest)
	if m == "" {
		return filepath.ToSlash(filepath.Join(remoteRepo, "src", "plugins", "robot", "src_v2", "config", "composition.manifest.json"))
	}
	if strings.HasPrefix(m, "/") {
		return filepath.ToSlash(filepath.Clean(m))
	}
	if strings.HasPrefix(m, "src/") {
		return filepath.ToSlash(filepath.Join(remoteRepo, m))
	}
	if strings.HasPrefix(m, "plugins/") {
		return filepath.ToSlash(filepath.Join(remoteRepo, "src", m))
	}
	return filepath.ToSlash(filepath.Join(remoteRepo, m))
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(strings.TrimSpace(s), "'", "'\\''") + "'"
}

func runDialtone(repoRoot string, args ...string) error {
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), args...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

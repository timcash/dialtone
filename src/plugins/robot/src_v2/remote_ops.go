package robotv2

import (
	cloudflarev1 "dialtone/dev/plugins/cloudflare/src_v1/go"
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	ssh_plugin "dialtone/dev/plugins/ssh/src_v1/go"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

const (
	defaultRobotMeshAlias   = "rover"
	defaultRobotRelayName   = "rover-1"
	defaultRobotPublicUIURL = "https://rover-1.dialtone.earth"
)

type robotRemoteTarget struct {
	Node    *ssh_plugin.MeshNode
	User    string
	SSHOpts ssh_plugin.CommandOptions
}

func runSrcV2Relay(repoRoot string, args []string) error {
	fs := flag.NewFlagSet("robot-src-v2-relay", flag.ContinueOnError)
	subdomain := fs.String("subdomain", "", "Cloudflare relay subdomain (default: DIALTONE_DOMAIN/DIALTONE_HOSTNAME/"+defaultRobotRelayName+")")
	robotUIURL := fs.String("robot-ui-url", "", "Robot UI URL target for relay (overrides --host/--port)")
	name := fs.String("name", "", "Deprecated alias for --subdomain")
	url := fs.String("url", "", "Deprecated alias for --robot-ui-url")
	host := fs.String("host", defaultRobotRelayName, "Robot host for relay target")
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

	relayName := chooseNonEmpty(
		strings.TrimSpace(*subdomain),
		strings.TrimSpace(*name),
		strings.TrimSpace(configv1.LookupEnvString("DIALTONE_DOMAIN")),
		strings.TrimSpace(configv1.LookupEnvString("DIALTONE_HOSTNAME")),
		defaultRobotRelayName,
	)

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
		serviceName, err := configureRobotRelayService(repoRoot, relayName, targetURL)
		if err != nil {
			return err
		}
		logs.Info("robot src_v2 relay service active: %s", serviceName)
	} else if err := runDialtone(repoRoot, "cloudflare", "src_v1", "robot", "--name", relayName, "--url", targetURL); err != nil {
		return err
	}
	logs.Info("robot src_v2 relay active: https://%s.dialtone.earth -> %s", relayName, targetURL)
	return nil
}

func configureRobotRelayService(repoRoot, name, targetURL string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = chooseNonEmpty(
			strings.TrimSpace(configv1.LookupEnvString("DIALTONE_DOMAIN")),
			strings.TrimSpace(configv1.LookupEnvString("DIALTONE_HOSTNAME")),
			defaultRobotRelayName,
		)
	}
	token := resolveRobotRelayTunnelToken(name)
	if token == "" {
		return "", fmt.Errorf("missing Cloudflare tunnel token for %s (set CF_TUNNEL_TOKEN_%s or CF_TUNNEL_TOKEN)", name, strings.ToUpper(strings.ReplaceAll(name, "-", "_")))
	}
	cfBin := resolveCloudflaredPath(repoRoot)
	if cfBin == "" {
		return "", fmt.Errorf("cloudflared binary not found (expected in DIALTONE_ENV/cloudflare or PATH)")
	}
	serviceName := fmt.Sprintf("dialtone-proxy-%s.service", name)
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	serviceDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		return "", err
	}
	servicePath := filepath.Join(serviceDir, serviceName)
	serviceContent := fmt.Sprintf(`[Unit]
Description=Dialtone Robot Relay for %s
After=network.target

[Service]
Type=simple
ExecStart=%s tunnel --no-autoupdate run --token %s --url %s
Restart=always
RestartSec=2

[Install]
WantedBy=default.target
`, name, cfBin, token, targetURL)
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0o644); err != nil {
		return "", err
	}
	if err := runSystemctlUser("daemon-reload"); err != nil {
		return "", err
	}
	if err := runSystemctlUser("enable", serviceName); err != nil {
		return "", err
	}
	if err := runSystemctlUser("restart", serviceName); err != nil {
		if err := runSystemctlUser("start", serviceName); err != nil {
			return "", err
		}
	}
	return serviceName, nil
}

func resolveRobotRelayTunnelToken(name string) string {
	return cloudflarev1.ResolveTunnelToken(strings.TrimSpace(name), "")
}

func resolveCloudflaredPath(repoRoot string) string {
	rt, err := configv1.ResolveRuntime(repoRoot)
	if err != nil {
		return ""
	}
	envRoot := rt.DialtoneEnv
	candidate := filepath.Join(envRoot, "cloudflare", "cloudflared")
	if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
		return candidate
	}
	if p, err := exec.LookPath("cloudflared"); err == nil {
		return p
	}
	return ""
}

func runSystemctlUser(args ...string) error {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func chooseNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func runSrcV2Clean(args []string) error {
	fs := flag.NewFlagSet("robot-src-v2-clean", flag.ContinueOnError)
	host := fs.String("host", "", "Robot mesh host to clean")
	user := fs.String("user", "", "SSH user override (defaults to mesh user)")
	port := fs.String("port", "", "SSH port override (defaults to mesh port)")
	password := fs.String("pass", "", "SSH password (optional; key auth preferred)")
	keepAutoswap := fs.Bool("keep-autoswap", false, "Remove everything except autoswap service/binary")
	restartAutoswap := fs.Bool("restart-autoswap", false, "When --keep-autoswap, restart autoswap after cleanup")
	if err := fs.Parse(args); err != nil {
		return err
	}
	targetHost := strings.TrimSpace(*host)
	if targetHost == "" {
		return fmt.Errorf("clean requires --host")
	}
	node, err := ssh_plugin.ResolveMeshNode(targetHost)
	if err != nil {
		return fmt.Errorf("clean requires a mesh node alias/hostname for --host: %w", err)
	}
	if node.OS == "windows" {
		return fmt.Errorf("clean currently supports linux/macos targets only; got windows node %q", node.Name)
	}
	targetUser := chooseNonEmpty(strings.TrimSpace(*user), strings.TrimSpace(node.User))
	opts := ssh_plugin.CommandOptions{
		User:     targetUser,
		Port:     strings.TrimSpace(*port),
		Password: strings.TrimSpace(*password),
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
	for key, want := range required {
		got := strings.TrimSpace(verifyMap[key])
		if got != want {
			return fmt.Errorf("remote clean verification failed on %s: %s=%q (want %q)", node.Name, key, got, want)
		}
	}
	logs.Info("robot src_v2 clean completed on %s", node.Name)
	logs.Info("%s", strings.TrimSpace(verifyOut))
	return nil
}

func runSrcV2Rollout(repoRoot string, args []string) error {
	fs := flag.NewFlagSet("robot-src-v2-rollout", flag.ContinueOnError)
	host := fs.String("host", defaultRobotMeshAlias, "Robot mesh host for rollout")
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
	requireNix := fs.Bool("require-nix", false, "Fail if nix is not installed on the robot host")
	if err := fs.Parse(args); err != nil {
		return err
	}

	targetInfo, err := resolveRequiredRobotMeshTarget("rollout", *host, *user, *port, *pass, defaultRobotMeshAlias)
	if err != nil {
		return err
	}

	nixProbe := "if command -v nix >/dev/null 2>&1; then nix --extra-experimental-features 'nix-command flakes' --version 2>/dev/null || nix --version; else echo MISSING; fi"
	nixOut, err := ssh_plugin.RunNodeCommand(targetInfo.Node.Name, nixProbe, targetInfo.SSHOpts)
	if err != nil {
		return fmt.Errorf("rollout nix probe failed on %s: %w", targetInfo.Node.Name, err)
	}
	nixOut = strings.TrimSpace(nixOut)
	if strings.EqualFold(nixOut, "MISSING") {
		if *requireNix {
			return fmt.Errorf("rollout requires nix on %s, but nix is not installed", targetInfo.Node.Name)
		}
		logs.Warn("robot src_v2 rollout: nix not installed on %s; continuing with autoswap release artifacts only", targetInfo.Node.Name)
	} else {
		logs.Info("robot src_v2 rollout: remote nix available on %s (%s)", targetInfo.Node.Name, nixOut)
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
		deployArgs := []string{"autoswap", "src_v1", "deploy", "--host", targetInfo.Node.Name, "--user", targetInfo.User, "--repo", strings.TrimSpace(*repo), "--service"}
		if targetInfo.SSHOpts.Port != "" {
			deployArgs = append(deployArgs, "--port", targetInfo.SSHOpts.Port)
		}
		if targetInfo.SSHOpts.Password != "" {
			deployArgs = append(deployArgs, "--pass", targetInfo.SSHOpts.Password)
		}
		if err := runDialtone(repoRoot, deployArgs...); err != nil {
			return err
		}
	}

	if !*skipUpdate {
		updateArgs := []string{"autoswap", "src_v1", "update", "--host", targetInfo.Node.Name, "--user", targetInfo.User}
		if targetInfo.SSHOpts.Port != "" {
			updateArgs = append(updateArgs, "--port", targetInfo.SSHOpts.Port)
		}
		if targetInfo.SSHOpts.Password != "" {
			updateArgs = append(updateArgs, "--pass", targetInfo.SSHOpts.Password)
		}
		if err := runDialtone(repoRoot, updateArgs...); err != nil {
			return err
		}
	}

	if !*skipDiagnostic {
		diagArgs := []string{"--host", targetInfo.Node.Name, "--user", targetInfo.User}
		if targetInfo.SSHOpts.Port != "" {
			diagArgs = append(diagArgs, "--port", targetInfo.SSHOpts.Port)
		}
		if targetInfo.SSHOpts.Password != "" {
			diagArgs = append(diagArgs, "--pass", targetInfo.SSHOpts.Password)
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

	logs.Info("robot src_v2 rollout completed host=%s user=%s", targetInfo.Node.Name, targetInfo.User)
	return nil
}

func runSrcV2NixDiagnostic(repoRoot string, args []string) error {
	fs := flag.NewFlagSet("robot-src-v2-nix-diagnostic", flag.ContinueOnError)
	host := fs.String("host", defaultRobotMeshAlias, "Robot mesh host for nix diagnostic")
	port := fs.String("port", "", "SSH port override")
	user := fs.String("user", "", "SSH user override")
	pass := fs.String("pass", "", "SSH password override")
	remoteRepo := fs.String("remote-repo", "", "Remote repo root (default: first mesh repo candidate or ~/dialtone)")
	syncFlake := fs.Bool("sync-flake", true, "Sync local flake.nix and dialtone.sh to remote repo before diagnostics")
	if err := fs.Parse(args); err != nil {
		return err
	}

	targetInfo, err := resolveRequiredRobotMeshTarget("nix-diagnostic", *host, *user, *port, *pass, defaultRobotMeshAlias)
	if err != nil {
		return err
	}
	targetRepo := resolveRobotRemoteRepoRoot(*remoteRepo, targetInfo.Node, targetInfo.User)

	if *syncFlake {
		if _, err := ssh_plugin.RunNodeCommand(targetInfo.Node.Name, "mkdir -p "+shellSingleQuote(targetRepo), targetInfo.SSHOpts); err != nil {
			return fmt.Errorf("nix-diagnostic prepare remote repo failed: %w", err)
		}
		if err := ssh_plugin.UploadNodeFile(targetInfo.Node.Name, filepath.Join(repoRoot, "flake.nix"), filepath.ToSlash(filepath.Join(targetRepo, "flake.nix")), targetInfo.SSHOpts); err != nil {
			return fmt.Errorf("nix-diagnostic sync flake.nix failed: %w", err)
		}
		if err := ssh_plugin.UploadNodeFile(targetInfo.Node.Name, filepath.Join(repoRoot, "dialtone.sh"), filepath.ToSlash(filepath.Join(targetRepo, "dialtone.sh")), targetInfo.SSHOpts); err != nil {
			return fmt.Errorf("nix-diagnostic sync dialtone.sh failed: %w", err)
		}
		if _, err := ssh_plugin.RunNodeCommand(targetInfo.Node.Name, "chmod +x "+shellSingleQuote(filepath.ToSlash(filepath.Join(targetRepo, "dialtone.sh"))), targetInfo.SSHOpts); err != nil {
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
		out, err := ssh_plugin.RunNodeCommand(targetInfo.Node.Name, check.cmd, targetInfo.SSHOpts)
		if err != nil {
			return fmt.Errorf("nix-diagnostic %s failed: %w", check.name, err)
		}
		logs.Info("robot src_v2 nix-diagnostic: %s ok: %s", check.name, strings.TrimSpace(firstLine(out)))
	}
	logs.Info("robot src_v2 nix-diagnostic completed host=%s repo=%s", targetInfo.Node.Name, targetRepo)
	return nil
}

func runSrcV2NixGC(args []string) error {
	fs := flag.NewFlagSet("robot-src-v2-nix-gc", flag.ContinueOnError)
	host := fs.String("host", defaultRobotMeshAlias, "Robot mesh host for nix garbage collection")
	port := fs.String("port", "", "SSH port override")
	user := fs.String("user", "", "SSH user override")
	pass := fs.String("pass", "", "SSH password override")
	if err := fs.Parse(args); err != nil {
		return err
	}

	targetInfo, err := resolveRequiredRobotMeshTarget("nix-gc", *host, *user, *port, *pass, defaultRobotMeshAlias)
	if err != nil {
		return err
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
	out, err := ssh_plugin.RunNodeCommand(targetInfo.Node.Name, cmd, targetInfo.SSHOpts)
	if err != nil {
		return fmt.Errorf("nix-gc failed on %s: %w", targetInfo.Node.Name, err)
	}
	logs.Raw("%s", strings.TrimSpace(out))
	logs.Info("robot src_v2 nix-gc completed host=%s user=%s", targetInfo.Node.Name, targetInfo.User)
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

func resolveRequiredRobotMeshTarget(action, host, user, port, pass, defaultHost string) (robotRemoteTarget, error) {
	var target robotRemoteTarget
	targetHost := strings.TrimSpace(host)
	if targetHost == "" {
		targetHost = strings.TrimSpace(defaultHost)
	}
	if targetHost == "" {
		return target, fmt.Errorf("%s requires --host", action)
	}
	node, err := ssh_plugin.ResolveMeshNode(targetHost)
	if err != nil {
		return target, fmt.Errorf("%s requires a mesh node alias/hostname for --host: %w", action, err)
	}
	targetUser := chooseNonEmpty(strings.TrimSpace(user), strings.TrimSpace(node.User))
	if targetUser == "" {
		return target, fmt.Errorf("%s requires --user or a mesh node with a default user", action)
	}
	target.Node = &node
	target.User = targetUser
	target.SSHOpts = ssh_plugin.CommandOptions{
		User:     targetUser,
		Port:     strings.TrimSpace(port),
		Password: strings.TrimSpace(pass),
	}
	return target, nil
}

func resolveRobotRemoteRepoRoot(explicit string, node *ssh_plugin.MeshNode, user string) string {
	targetRepo := strings.TrimSpace(explicit)
	if targetRepo != "" {
		return targetRepo
	}
	if node != nil && len(node.RepoCandidates) > 0 {
		if candidate := strings.TrimSpace(node.RepoCandidates[0]); candidate != "" {
			return candidate
		}
	}
	return filepath.ToSlash(filepath.Join("/home", user, "dialtone"))
}

func runRobotSyncCode(repoRoot string, args []string) error {
	return runDialtone(repoRoot, append([]string{"ssh", "src_v1", "sync-code"}, args...)...)
}

func runRobotSyncWatch(repoRoot string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("sync-watch requires start|stop|status")
	}
	switch args[0] {
	case "start":
		rest := append([]string{"ssh", "src_v1", "sync-code", "--service"}, args[1:]...)
		return runDialtone(repoRoot, rest...)
	case "stop":
		return runDialtone(repoRoot, "ssh", "src_v1", "sync-code", "--service-stop")
	case "status":
		return runDialtone(repoRoot, "ssh", "src_v1", "sync-code", "--service-status")
	default:
		return fmt.Errorf("sync-watch requires start|stop|status")
	}
}

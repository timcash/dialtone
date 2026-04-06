package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

const syncCodeServiceName = "dialtone-ssh-sync-code.service"

func InstallSyncCodeService(opts SyncCodeOptions, interval time.Duration) error {
	if strings.TrimSpace(opts.Node) == "" {
		return fmt.Errorf("--host is required with --service")
	}
	repoRoot, err := findDialtoneRepoRoot()
	if err != nil {
		return err
	}
	source := strings.TrimSpace(opts.Source)
	if source == "" {
		source = repoRoot
	}
	source = strings.TrimRight(source, "/")
	if source == "" {
		return fmt.Errorf("source path is empty")
	}
	if _, err := os.Stat(source); err != nil {
		return fmt.Errorf("source path missing: %s", source)
	}
	replIndexInfof("ssh sync-code service: installing %s interval=%s", syncCodeSummary(opts.Node, source, opts.Dest, opts.Delete), strings.TrimSpace(interval.String()))

	unitDir, err := systemdUserDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		return fmt.Errorf("create systemd user dir: %w", err)
	}
	unitPath := filepath.Join(unitDir, syncCodeServiceName)

	execCmd := buildSyncCodeLoopExec(repoRoot, SyncCodeOptions{
		Node:     opts.Node,
		Source:   source,
		Dest:     opts.Dest,
		Delete:   opts.Delete,
		Excludes: opts.Excludes,
	}, interval)

	unit := strings.Join([]string{
		"[Unit]",
		"Description=Dialtone SSH sync-code loop",
		"After=network-online.target",
		"",
		"[Service]",
		"Type=simple",
		"ExecStart=" + execCmd,
		"Restart=always",
		"RestartSec=2",
		"",
		"[Install]",
		"WantedBy=default.target",
		"",
	}, "\n")

	if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
		return fmt.Errorf("write systemd service file: %w", err)
	}
	if err := runSystemctlUser("daemon-reload"); err != nil {
		return err
	}
	if err := runSystemctlUser("enable", "--now", syncCodeServiceName); err != nil {
		return err
	}
	logs.Info("Installed/started user service: %s", unitPath)
	replIndexInfof("ssh sync-code service: installed %s interval=%s", syncCodeSummary(opts.Node, source, opts.Dest, opts.Delete), strings.TrimSpace(interval.String()))
	return StatusSyncCodeService()
}

func StopSyncCodeService() error {
	replIndexInfof("ssh sync-code service: stopping %s", syncCodeServiceName)
	_ = runSystemctlUser("stop", syncCodeServiceName)
	_ = runSystemctlUser("disable", syncCodeServiceName)
	if err := runSystemctlUser("reset-failed", syncCodeServiceName); err != nil {
		logs.Warn("reset-failed warning: %v", err)
	}
	logs.Info("Stopped/disabled user service: %s", syncCodeServiceName)
	replIndexInfof("ssh sync-code service: stopped %s", syncCodeServiceName)
	return nil
}

func StatusSyncCodeService() error {
	activeText, activeErr := runSystemctlUserOutput("is-active", syncCodeServiceName)
	enabledText, enabledErr := runSystemctlUserOutput("is-enabled", syncCodeServiceName)
	replIndexInfof("ssh sync-code service: %s", summarizeSyncCodeServiceState(activeText, enabledText, activeErr, enabledErr))
	out, err := runSystemctlUserOutput("status", "--no-pager", syncCodeServiceName)
	if strings.TrimSpace(out) != "" {
		logs.Raw("%s", strings.TrimSpace(out))
	}
	if err != nil {
		return fmt.Errorf("systemd status failed: %w", err)
	}
	return nil
}

func buildSyncCodeLoopExec(repoRoot string, opts SyncCodeOptions, interval time.Duration) string {
	args := []string{
		"./dialtone.sh", "ssh", "src_v1", "sync-code",
		"--host", strings.TrimSpace(opts.Node),
		"--src", strings.TrimSpace(opts.Source),
	}
	if d := strings.TrimSpace(opts.Dest); d != "" {
		args = append(args, "--dest", d)
	}
	if opts.Delete {
		args = append(args, "--delete")
	}
	for _, ex := range opts.Excludes {
		ex = strings.TrimSpace(ex)
		if ex == "" {
			continue
		}
		args = append(args, "--exclude", ex)
	}

	// Intentionally use the public Dialtone entrypoint so each sync loop is
	// injected through REPL and produces shared task/index output on NATS.
	quoted := make([]string, 0, len(args))
	for _, a := range args {
		quoted = append(quoted, shellQuoteSyncService(a))
	}
	cmd := strings.Join(quoted, " ")
	loop := fmt.Sprintf("cd %s && while true; do %s; sleep %s; done", shellQuoteSyncService(repoRoot), cmd, shellQuoteSyncService(interval.String()))
	return "/bin/bash -lc " + shellQuoteSyncService(loop)
}

func shellQuoteSyncService(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, `'`, `'\''`)
	return "'" + v + "'"
}

func findDialtoneRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	cur := cwd
	for {
		if _, err := os.Stat(filepath.Join(cur, "dialtone.sh")); err == nil {
			return cur, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return "", fmt.Errorf("unable to find repo root containing dialtone.sh from %s", cwd)
}

func systemdUserDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(home) == "" {
		return "", fmt.Errorf("home directory not found")
	}
	return filepath.Join(home, ".config", "systemd", "user"), nil
}

func runSystemctlUser(args ...string) error {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		logs.Raw("%s", strings.TrimSpace(string(out)))
	}
	if err != nil {
		return fmt.Errorf("systemctl %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func runSystemctlUserOutput(args ...string) (string, error) {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func summarizeSyncCodeServiceState(activeText, enabledText string, activeErr, enabledErr error) string {
	active := normalizeSystemctlState(activeText)
	enabled := normalizeSystemctlState(enabledText)
	if active == "" && enabled == "" && (activeErr != nil || enabledErr != nil) {
		return "not installed"
	}
	parts := make([]string, 0, 2)
	if active != "" {
		parts = append(parts, "active="+active)
	}
	if enabled != "" {
		parts = append(parts, "enabled="+enabled)
	}
	if len(parts) == 0 {
		return "status unavailable"
	}
	return strings.Join(parts, " ")
}

func normalizeSystemctlState(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	line := strings.SplitN(raw, "\n", 2)[0]
	line = strings.TrimSpace(line)
	if strings.EqualFold(line, "Unit "+syncCodeServiceName+" could not be found.") {
		return ""
	}
	return line
}

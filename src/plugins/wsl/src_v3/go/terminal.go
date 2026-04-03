package wslv3

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

func StartInstance(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("instance name is required")
	}

	if _, err := wslExecWithTimeout(30*time.Second, "-d", name, "-u", "root", "--", "sh", "-lc", "mkdir -p /run && touch /run/dialtone-start-probe"); err != nil {
		return err
	}

	p := NewWslPlugin("")
	if err := p.startKeepAlive(name); err != nil {
		return err
	}

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		instances, err := p.listInstances()
		if err == nil {
			for _, inst := range instances {
				if strings.EqualFold(strings.TrimSpace(inst.Name), name) && strings.EqualFold(strings.TrimSpace(inst.State), "Running") {
					return nil
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("instance %s did not reach Running state after start", name)
}

func OpenTerminal(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("instance name is required")
	}
	if err := StartInstance(name); err != nil {
		return err
	}

	cfg := resolveTerminalBootstrapConfig()

	wslExe, err := resolveWSLExecutable()
	if err != nil {
		return err
	}
	wslProgram := toWindowsPath(wslExe)
	if err := startTerminalChromeWarmup(name, wslExe, cfg); err != nil {
		return err
	}
	script := terminalBootstrapScriptWithConfig(name, cfg)
	launcherPath, err := writeTerminalLauncherScript(name, wslProgram, script)
	if err != nil {
		return err
	}
	cmdExe, err := resolveCmdExecutable()
	if err != nil {
		return err
	}
	cmdProgram := toWindowsPath(cmdExe)
	powershellExe, err := resolveWindowsExecutable("powershell.exe")
	if err != nil {
		return err
	}
	launcherScript := fmt.Sprintf("Start-Process -FilePath %s -WindowStyle Normal -ArgumentList @(%s)",
		psSingleQuote(cmdProgram),
		strings.Join([]string{
			psSingleQuote("/k"),
			psSingleQuote(launcherPath),
		}, ", "),
	)
	cmd := exec.Command(powershellExe, "-NoProfile", "-Command", launcherScript)
	return cmd.Run()
}

func terminalWindowTitle(name string) string {
	return fmt.Sprintf("Dialtone WSL - %s", strings.TrimSpace(name))
}

func terminalBootstrapScript(name string) string {
	return terminalBootstrapScriptWithConfig(name, resolveTerminalBootstrapConfig())
}

type terminalBootstrapConfig struct {
	RepoRoot      string
	ChromeHost    string
	ChromeRole    string
	ChromeEnabled bool
	TerminalTMUX  string
	CADEnabled    bool
	CADTMUX       string
	CADPort       int
}

func resolveTerminalBootstrapConfig() terminalBootstrapConfig {
	return terminalBootstrapConfig{
		RepoRoot:      resolveTerminalRepoRoot(),
		ChromeHost:    resolveTerminalChromeHost(),
		ChromeRole:    resolveTerminalChromeRole(),
		ChromeEnabled: resolveTerminalBool("DIALTONE_WSL_TERMINAL_CHROME_ENABLED", true),
		TerminalTMUX:  resolveTerminalTMUXSession(),
		CADEnabled:    resolveTerminalBool("DIALTONE_WSL_TERMINAL_CAD_ENABLED", true),
		CADTMUX:       resolveTerminalCADTMUXSession(),
		CADPort:       resolveTerminalCADPort(),
	}
}

func resolveTerminalRepoRoot() string {
	if repoRoot := strings.TrimSpace(configv1.LookupEnvString("DIALTONE_WSL_TERMINAL_REPO_ROOT")); repoRoot != "" {
		return normalizeTerminalRepoRoot(repoRoot)
	}
	if envFile := strings.TrimSpace(configv1.ResolveEnvFilePath("")); envFile != "" {
		if repoRoot := strings.TrimSpace(configv1.EnvFileString(envFile, "DIALTONE_REPO_ROOT")); repoRoot != "" {
			return normalizeTerminalRepoRoot(repoRoot)
		}
	}
	return "/home/user/dialtone"
}

func resolveTerminalChromeHost() string {
	if host := strings.TrimSpace(configv1.LookupEnvString("DIALTONE_WSL_TERMINAL_CHROME_HOST")); host != "" {
		return host
	}
	return "legion"
}

func resolveTerminalChromeRole() string {
	if role := strings.TrimSpace(configv1.LookupEnvString("DIALTONE_WSL_TERMINAL_CHROME_ROLE")); role != "" {
		return role
	}
	return "dev"
}

func resolveTerminalTMUXSession() string {
	if session := strings.TrimSpace(configv1.LookupEnvString("DIALTONE_WSL_TERMINAL_TMUX_SESSION")); session != "" {
		return session
	}
	return "dialtone"
}

func resolveTerminalCADTMUXSession() string {
	if session := strings.TrimSpace(configv1.LookupEnvString("DIALTONE_WSL_TERMINAL_CAD_TMUX_SESSION")); session != "" {
		return session
	}
	return "dialtone-cad"
}

func resolveTerminalCADPort() int {
	if raw := strings.TrimSpace(configv1.LookupEnvString("DIALTONE_WSL_TERMINAL_CAD_PORT")); raw != "" {
		if port, err := strconv.Atoi(raw); err == nil && port > 0 {
			return port
		}
	}
	return 8081
}

func resolveTerminalBool(name string, defaultValue bool) bool {
	switch strings.ToLower(strings.TrimSpace(configv1.LookupEnvString(name))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultValue
	}
}

func normalizeTerminalRepoRoot(repoRoot string) string {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return "/home/user/dialtone"
	}
	if strings.HasPrefix(repoRoot, "/") {
		return repoRoot
	}
	return windowsPathToWSLPath(repoRoot)
}

func terminalBootstrapScriptWithConfig(name string, cfg terminalBootstrapConfig) string {
	repoRoot := strings.TrimSpace(cfg.RepoRoot)
	if repoRoot == "" {
		repoRoot = "/home/user/dialtone"
	}
	chromeHost := strings.TrimSpace(cfg.ChromeHost)
	if chromeHost == "" {
		chromeHost = "legion"
	}
	chromeRole := strings.TrimSpace(cfg.ChromeRole)
	if chromeRole == "" {
		chromeRole = "dev"
	}
	cadSession := strings.TrimSpace(cfg.CADTMUX)
	if cadSession == "" {
		cadSession = "dialtone-cad"
	}
	cadPort := cfg.CADPort
	if cadPort <= 0 {
		cadPort = 8081
	}

	lines := []string{
		"export TERM=${TERM:-xterm-256color}",
		"mkdir -p \"$HOME/.dialtone/logs\"",
		fmt.Sprintf("repo_root=%s", shellSingleQuote(repoRoot)),
		fmt.Sprintf("chrome_host=%s", shellSingleQuote(chromeHost)),
		fmt.Sprintf("chrome_role=%s", shellSingleQuote(chromeRole)),
		fmt.Sprintf("cad_session=%s", shellSingleQuote(cadSession)),
		fmt.Sprintf("cad_port=%d", cadPort),
		"if [ -d \"$repo_root\" ]; then cd \"$repo_root\"; fi",
		fmt.Sprintf("printf '\\033]0;%%s\\a' %s", shellSingleQuote(terminalWindowTitle(name))),
		fmt.Sprintf("printf '\\033[1;32mDialtone WSL terminal\\033[0m for %%s\\n' %s", shellSingleQuote(name)),
		"printf 'Repo: %s\\n' \"$PWD\"",
		fmt.Sprintf("printf 'Distro: %%s\\n' %s", shellSingleQuote(name)),
	}
	if cfg.CADEnabled {
		lines = append(lines,
			fmt.Sprintf("cad_cmd=%s", shellSingleQuote(fmt.Sprintf("exec ./dialtone.sh cad src_v1 serve --port %d", cadPort))),
			"cad_ready=0",
			fmt.Sprintf("if command -v curl >/dev/null 2>&1; then if curl -fsS http://127.0.0.1:%d/health >/dev/null 2>&1; then cad_ready=1; fi; elif command -v wget >/dev/null 2>&1; then if wget -qO- http://127.0.0.1:%d/health >/dev/null 2>&1; then cad_ready=1; fi; fi", cadPort, cadPort),
		)
		lines = append(lines, "printf 'CAD session: %s (http://127.0.0.1:%s)\\n' \"$cad_session\" \"$cad_port\"")
	}
	lines = append(lines,
		fmt.Sprintf("printf '%%s\\n' %s", shellSingleQuote("Run ./dialtone.sh to enter the dialtone> repl.")),
		fmt.Sprintf("printf '%%s\\n' %s", shellSingleQuote("Type Linux commands directly at the prompt below. Type exit to close this terminal.")),
	)
	if cfg.CADEnabled {
		lines = append(lines,
			fmt.Sprintf("printf '%%s\\n' %s", shellSingleQuote("CAD stays alive in a dedicated tmux session so repeated terminal opens can reuse the same server.")),
			fmt.Sprintf("printf '%%s\\n' %s", shellSingleQuote(fmt.Sprintf("Health check: curl -fsS http://127.0.0.1:%d/health", cadPort))),
			"printf 'Inspect CAD session with: tmux attach -t %s\\n' \"$cad_session\"",
			fmt.Sprintf("printf 'From Windows: .\\\\dialtone.ps1 tmux status -Session %%s -Distro %%s -Cwd %%s\\n' \"$cad_session\" %s \"$repo_root\"", shellSingleQuote(name)),
			"if [ \"$cad_ready\" -eq 1 ]; then printf 'CAD already ready on http://127.0.0.1:%s.\\n' \"$cad_port\"; elif command -v tmux >/dev/null 2>&1; then tmux kill-session -t \"$cad_session\" >/dev/null 2>&1 || true; tmux new-session -d -s \"$cad_session\" -c \"$repo_root\" \"$cad_cmd\" >/dev/null 2>&1 && printf 'CAD warmup started in tmux session %s.\\n' \"$cad_session\" || printf 'Failed to start CAD tmux session %s.\\n' \"$cad_session\"; else printf 'tmux not available; skipping CAD warmup.\\n'; fi",
		)
	}
	if cfg.ChromeEnabled {
		lines = append(lines,
			fmt.Sprintf("printf 'Chrome warmup target: %%s role=%%s\\n' \"$chrome_host\" \"$chrome_role\""),
			fmt.Sprintf("printf 'Chrome warmup log: %%s\\n\\n' %s", shellSingleQuote(terminalChromeWarmupLogPath(chromeHost, chromeRole))),
		)
	}
	lines = append(lines,
		fmt.Sprintf("printf '%%s\\n\\n' %s", shellSingleQuote(fmt.Sprintf("Recommended next step: run ./dialtone.sh and then use /chrome src_v3 status --host %s --role %s inside the REPL.", chromeHost, chromeRole))),
		"printf '%s\\n\\n' 'Terminal is ready in the repo root. Type commands directly at the prompt below.'",
		"if command -v bash >/dev/null 2>&1; then exec bash -li; fi",
		"if command -v zsh >/dev/null 2>&1; then exec zsh -li; fi",
		"exec sh -li",
	)
	return strings.Join(lines, "; ")
}

func terminalChromeWarmupLogName(host, role string) string {
	return "wsl-terminal-chrome-" + terminalPathToken(host) + "-" + terminalPathToken(role) + ".log"
}

func terminalChromeWarmupLogPath(host, role string) string {
	return "$HOME/.dialtone/logs/" + terminalChromeWarmupLogName(host, role)
}

func terminalChromeWarmupStampName(host, role string) string {
	return "wsl-terminal-chrome-" + terminalPathToken(host) + "-" + terminalPathToken(role) + ".stamp"
}

func terminalChromeWarmupStampPath(host, role string) string {
	return "$HOME/.dialtone/logs/" + terminalChromeWarmupStampName(host, role)
}

func terminalChromeWarmupScript(cfg terminalBootstrapConfig) string {
	return strings.Join([]string{
		"mkdir -p \"$HOME/.dialtone/logs\"",
		fmt.Sprintf("warmup_log=$HOME/.dialtone/logs/%s", terminalChromeWarmupLogName(cfg.ChromeHost, cfg.ChromeRole)),
		fmt.Sprintf("warmup_stamp=$HOME/.dialtone/logs/%s", terminalChromeWarmupStampName(cfg.ChromeHost, cfg.ChromeRole)),
		"warmup_now=$(date +%s 2>/dev/null || echo 0)",
		"warmup_last=0",
		"if [ -f \"$warmup_stamp\" ]; then warmup_last=$(cat \"$warmup_stamp\" 2>/dev/null || echo 0); fi",
		"if [ \"$warmup_now\" -gt 0 ] && [ \"$warmup_last\" -gt 0 ] && [ $((warmup_now - warmup_last)) -lt 30 ]; then exit 0; fi",
		"if [ \"$warmup_now\" -gt 0 ]; then printf '%s' \"$warmup_now\" > \"$warmup_stamp\"; fi",
		fmt.Sprintf("cd %s", shellSingleQuote(cfg.RepoRoot)),
		fmt.Sprintf("./dialtone.sh chrome src_v3 deploy --host %s --role %s --service >> \"$warmup_log\" 2>&1",
			shellSingleQuote(cfg.ChromeHost),
			shellSingleQuote(cfg.ChromeRole),
		),
	}, "; ")
}

func startTerminalChromeWarmup(name, wslExe string, cfg terminalBootstrapConfig) error {
	if !cfg.ChromeEnabled {
		return nil
	}
	if strings.TrimSpace(cfg.RepoRoot) == "" {
		return fmt.Errorf("terminal chrome warmup repo root is required")
	}
	cmd := exec.Command(wslExe,
		"-d", name,
		"--cd", cfg.RepoRoot,
		"--",
		"sh", "-lc", terminalChromeWarmupScript(cfg),
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Start()
}

func windowsPathToWSLPath(path string) string {
	path = strings.TrimSpace(path)
	if len(path) < 3 || path[1] != ':' {
		return path
	}
	drive := strings.ToLower(path[:1])
	rest := strings.ReplaceAll(path[2:], `\`, "/")
	rest = strings.TrimPrefix(rest, "/")
	return "/mnt/" + drive + "/" + rest
}

func terminalPathToken(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return "default"
	}
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-_")
	if out == "" {
		return "default"
	}
	return out
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func psSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func cmdDoubleQuote(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func writeTerminalLauncherScript(name, wslProgram, bootstrapScript string) (string, error) {
	tempDir := os.TempDir()

	shFile, err := os.CreateTemp(tempDir, "dialtone-wsl-terminal-*.sh")
	if err != nil {
		return "", err
	}
	shPath := shFile.Name()
	shContent := "#!/bin/sh\n" + bootstrapScript + "\n"
	if _, err := shFile.WriteString(shContent); err != nil {
		_ = shFile.Close()
		return "", err
	}
	if err := shFile.Close(); err != nil {
		return "", err
	}

	linuxShellPath := windowsPathToWSLPath(shPath)
	cmdFile, err := os.CreateTemp(tempDir, "dialtone-wsl-terminal-*.cmd")
	if err != nil {
		return "", err
	}
	cmdPath := cmdFile.Name()
	lines := []string{
		"@echo off",
		"title " + terminalWindowTitle(name),
		fmt.Sprintf("%s -d %s --cd %s -- sh %s",
			cmdDoubleQuote(wslProgram),
			cmdDoubleQuote(name),
			cmdDoubleQuote("~"),
			cmdDoubleQuote(linuxShellPath),
		),
	}
	if _, err := cmdFile.WriteString(strings.Join(lines, "\r\n") + "\r\n"); err != nil {
		_ = cmdFile.Close()
		return "", err
	}
	if err := cmdFile.Close(); err != nil {
		return "", err
	}
	return cmdPath, nil
}

func resolveWindowsRepoRoot() string {
	if rt, err := configv1.ResolveRuntime(""); err == nil {
		if repoRoot := strings.TrimSpace(rt.RepoRoot); repoRoot != "" && !strings.HasPrefix(repoRoot, "/") {
			return repoRoot
		}
	}
	if repoRoot := strings.TrimSpace(configv1.LookupEnvString("DIALTONE_REPO_ROOT")); repoRoot != "" && !strings.HasPrefix(repoRoot, "/") {
		return repoRoot
	}
	if wd, err := os.Getwd(); err == nil {
		candidates := []string{wd, filepath.Dir(wd)}
		for _, candidate := range candidates {
			if _, err := os.Stat(filepath.Join(candidate, "dialtone.ps1")); err == nil {
				return candidate
			}
		}
	}
	return ""
}

func resolveDialtonePS1() (string, error) {
	candidates := []string{}
	if repoRoot := resolveWindowsRepoRoot(); repoRoot != "" {
		candidates = append(candidates, filepath.Join(repoRoot, "dialtone.ps1"))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(wd, "dialtone.ps1"),
			filepath.Join(filepath.Dir(wd), "dialtone.ps1"),
		)
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("dialtone.ps1 not found in repo root")
}

func runWSLTMUX(session, distro, cwd, action string, commandArgs ...string) error {
	scriptPath, err := resolveDialtonePS1()
	if err != nil {
		return err
	}
	powershellExe, err := resolveWindowsExecutable("powershell.exe")
	if err != nil {
		return err
	}
	args := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptPath, "tmux", action, "-Session", session}
	if distro != "" {
		args = append(args, "-Distro", distro)
	}
	if cwd != "" {
		args = append(args, "-Cwd", cwd)
	}
	args = append(args, commandArgs...)
	out, err := exec.Command(powershellExe, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("dialtone.ps1 tmux %s failed: %w (%s)", action, err, strings.TrimSpace(cleanWSLOutput(out)))
	}
	return nil
}

func runWSLShellWithTimeout(name, cwd string, timeout time.Duration, script string) ([]byte, error) {
	args := []string{"-d", name}
	if strings.TrimSpace(cwd) != "" {
		args = append(args, "--cd", cwd)
	}
	args = append(args, "--", "sh", "-lc", script)
	return wslExecWithTimeout(timeout, args...)
}

func cadHealthCheckScript(cfg terminalBootstrapConfig) string {
	return fmt.Sprintf("if command -v curl >/dev/null 2>&1; then curl -fsS http://127.0.0.1:%d/health >/dev/null; elif command -v wget >/dev/null 2>&1; then wget -qO- http://127.0.0.1:%d/health >/dev/null; else python3 - <<'PY'\nimport sys, urllib.request\ntry:\n    body = urllib.request.urlopen('http://127.0.0.1:%d/health', timeout=2).read().decode().strip()\nexcept Exception:\n    sys.exit(1)\nsys.exit(0 if body == 'ok' else 1)\nPY\nfi",
		cfg.CADPort,
		cfg.CADPort,
		cfg.CADPort,
	)
}

func cadStartupScript(cfg terminalBootstrapConfig) string {
	return fmt.Sprintf("cd %s && printf 'Starting CAD via public Dialtone entrypoint on 127.0.0.1:%d...\\n'; exec ./dialtone.sh cad src_v1 serve --port %d",
		shellSingleQuote(cfg.RepoRoot),
		cfg.CADPort,
		cfg.CADPort,
	)
}

func resolveCmdExecutable() (string, error) {
	candidates := []string{
		"cmd.exe",
		"/mnt/c/WINDOWS/system32/cmd.exe",
		"/mnt/c/Windows/System32/cmd.exe",
		`C:\WINDOWS\system32\cmd.exe`,
		`C:\Windows\System32\cmd.exe`,
	}
	for _, candidate := range candidates {
		if strings.Contains(candidate, string(os.PathSeparator)) || strings.Contains(candidate, ":") {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
			continue
		}
		if resolved, err := exec.LookPath(candidate); err == nil {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("cmd.exe not found on PATH or known Windows locations")
}

func resolveWindowsExecutable(name string) (string, error) {
	if resolved, err := exec.LookPath(name); err == nil {
		return resolved, nil
	}

	userProfile := strings.TrimSpace(os.Getenv("USERPROFILE"))
	if userProfile != "" {
		candidate := filepath.Join(userProfile, "AppData", "Local", "Microsoft", "WindowsApps", name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	cmdExe, err := resolveCmdExecutable()
	if err != nil {
		return "", err
	}
	out, err := exec.Command(cmdExe, "/c", "where", name).CombinedOutput()
	if err == nil {
		for _, line := range strings.Split(cleanWSLOutput(out), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				return line, nil
			}
		}
	}
	return "", fmt.Errorf("%s not found on PATH or known Windows locations", name)
}

package wslv3

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	wslExe, err := resolveWSLExecutable()
	if err != nil {
		return err
	}
	wslProgram := toWindowsPath(wslExe)
	script := terminalBootstrapScript(name)
	powershellExe, err := resolveWindowsExecutable("powershell.exe")
	if err != nil {
		return err
	}
	launcherScript := fmt.Sprintf("Start-Process -FilePath %s -WindowStyle Normal -ArgumentList @(%s)",
		psSingleQuote(wslProgram),
		strings.Join([]string{
			psSingleQuote("-d"),
			psSingleQuote(name),
			psSingleQuote("--cd"),
			psSingleQuote("~"),
			psSingleQuote("--"),
			psSingleQuote("sh"),
			psSingleQuote("-lc"),
			psSingleQuote(script),
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
	RepoRoot   string
	ChromeHost string
	ChromeRole string
}

func resolveTerminalBootstrapConfig() terminalBootstrapConfig {
	return terminalBootstrapConfig{
		RepoRoot:   resolveTerminalRepoRoot(),
		ChromeHost: resolveTerminalChromeHost(),
		ChromeRole: resolveTerminalChromeRole(),
	}
}

func resolveTerminalRepoRoot() string {
	if repoRoot := strings.TrimSpace(configv1.LookupEnvString("DIALTONE_WSL_TERMINAL_REPO_ROOT")); repoRoot != "" {
		return repoRoot
	}
	if envFile := strings.TrimSpace(configv1.ResolveEnvFilePath("")); envFile != "" {
		if repoRoot := strings.TrimSpace(configv1.EnvFileString(envFile, "DIALTONE_REPO_ROOT")); strings.HasPrefix(repoRoot, "/") {
			return repoRoot
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
	warmupCmd := fmt.Sprintf("cd %s && ./dialtone.sh chrome src_v3 deploy --host %s --role %s --service",
		shellSingleQuote(repoRoot),
		shellSingleQuote(chromeHost),
		shellSingleQuote(chromeRole),
	)
	lines := []string{
		"export TERM=${TERM:-xterm-256color}",
		"mkdir -p \"$HOME/.dialtone/logs\"",
		fmt.Sprintf("repo_root=%s", shellSingleQuote(repoRoot)),
		fmt.Sprintf("chrome_host=%s", shellSingleQuote(chromeHost)),
		fmt.Sprintf("chrome_role=%s", shellSingleQuote(chromeRole)),
		fmt.Sprintf("warmup_log=$HOME/.dialtone/logs/%s", terminalChromeWarmupLogName(chromeHost, chromeRole)),
		"if [ -d \"$repo_root\" ]; then cd \"$repo_root\"; fi",
		fmt.Sprintf("printf '\\033]0;%%s\\a' %s", shellSingleQuote(terminalWindowTitle(name))),
		fmt.Sprintf("printf '\\033[1;32mDialtone WSL terminal\\033[0m for %%s\\n' %s", shellSingleQuote(name)),
		"printf 'Repo: %s\\n' \"$PWD\"",
		fmt.Sprintf("printf '%%s\\n' %s", shellSingleQuote("Run ./dialtone.sh to enter the dialtone> repl.")),
		fmt.Sprintf("printf '%%s\\n' %s", shellSingleQuote("Type Linux commands directly at the prompt below. Type exit to close this terminal.")),
		fmt.Sprintf("if [ -x ./dialtone.sh ]; then nohup sh -lc %s >> \"$warmup_log\" 2>&1 & printf 'Chrome warmup queued on %%s role=%%s. Log: %%s\\n\\n' \"$chrome_host\" \"$chrome_role\" \"$warmup_log\"; else printf 'dialtone.sh not found in %%s; skipping Chrome warmup.\\n\\n' \"$PWD\"; fi",
			shellSingleQuote(warmupCmd),
		),
		fmt.Sprintf("printf '%%s\\n\\n' %s", shellSingleQuote("Recommended next step: run ./dialtone.sh and then use /chrome src_v3 status --host legion --role dev inside the REPL.")),
		"if command -v bash >/dev/null 2>&1; then exec bash -li; fi",
		"if command -v zsh >/dev/null 2>&1; then exec zsh -li; fi",
		"exec sh -li",
	}
	return strings.Join(lines, "; ")
}

func terminalChromeWarmupLogName(host, role string) string {
	return "wsl-terminal-chrome-" + terminalPathToken(host) + "-" + terminalPathToken(role) + ".log"
}

func terminalChromeWarmupLogPath(host, role string) string {
	return "$HOME/.dialtone/logs/" + terminalChromeWarmupLogName(host, role)
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

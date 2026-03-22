package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func runBootstrap(args []string) error {
	opts := parseBootstrapArgs(args)

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	envPath, _ := resolveFilePath(repoRoot, opts.envFile, filepath.Join("env", ".env"))
	cfg, err := resolveConfig(opts.hostname, opts.stateDir)
	if err != nil {
		return err
	}

	if opts.preferNative {
		nativeRunning, nativeTailnet, _ := detectNativeTailscale(cfg.Tailnet)
		if strings.TrimSpace(nativeTailnet) != "" {
			cfg.Tailnet = nativeTailnet
		}
		if nativeRunning {
			fmt.Printf("tsnet bootstrap: native tailscale daemon detected for %s\n", cfg.Hostname)
			if !opts.skipACL {
				if err := ensureDialtoneACL(cfg); err != nil {
					fmt.Printf("warning: tsnet ACL check failed: %v\n", err)
				}
			}
			if opts.noKeepalive {
				return nil
			}
			return nil
		}
	}

	if err := ensureAuthKey(cfg, envPath); err != nil {
		return err
	}

	if !opts.skipACL {
		if err := ensureDialtoneACL(cfg); err != nil {
			return err
		}
	}

	if opts.noKeepalive {
		return nil
	}

	return ensureKeepalive(repoRoot, cfg, envPath, opts.stateDir)
}

type bootstrapConfig struct {
	hostname     string
	envFile      string
	stateDir     string
	noKeepalive  bool
	preferNative bool
	skipACL      bool
}

func parseBootstrapArgs(args []string) bootstrapConfig {
	cfg := bootstrapConfig{
		hostname:     sanitizeHost(os.Getenv("DIALTONE_HOSTNAME")),
		envFile:      os.Getenv("DIALTONE_ENV_FILE"),
		stateDir:     "",
		preferNative: true,
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--host":
			if i+1 < len(args) {
				cfg.hostname = sanitizeHost(args[i+1])
				i++
			}
		case "--env":
			if i+1 < len(args) {
				cfg.envFile = args[i+1]
				i++
			}
		case "--env-file":
			if i+1 < len(args) {
				cfg.envFile = args[i+1]
				i++
			}
		case "--state-dir":
			if i+1 < len(args) {
				cfg.stateDir = strings.TrimSpace(args[i+1])
				i++
			}
		case "--no-keepalive":
			cfg.noKeepalive = true
		case "--prefer-native":
			cfg.preferNative = true
		case "--skip-native":
			cfg.preferNative = false
		case "--skip-acl":
			cfg.skipACL = true
		case "--skip-acl-check":
			cfg.skipACL = true
		}
	}
	return cfg
}

func detectNativeTailscale(expectedTailnet string) (running bool, detectedTailnet string, err error) {
	cmdPath, lookErr := exec.LookPath("tailscale")
	if lookErr != nil {
		return false, "", nil
	}
	out, err := exec.Command(cmdPath, "status", "--json").Output()
	if err != nil {
		return false, "", nil
	}

	var status map[string]any
	if err := json.Unmarshal(out, &status); err != nil {
		return false, "", nil
	}

	runningState, _ := status["BackendState"].(string)
	if strings.ToLower(strings.TrimSpace(runningState)) != "running" {
		return false, "", nil
	}

	detected := sanitizeHost(strings.TrimSpace(expectedTailnet))
	if tailnet := parseTailnetFromStatusJSON(out); tailnet != "" {
		detected = sanitizeHost(tailnet)
	}
	return true, detected, nil
}

func ensureKeepalive(repoRoot string, cfg tsnetConfig, envPath, stateDir string) error {
	if strings.TrimSpace(os.Getenv(cfg.AuthKeyEnv)) == "" {
		return errors.New("TS_AUTHKEY required for tsnet keepalive")
	}
	hostname := strings.TrimSpace(cfg.Hostname)
	if hostname == "" {
		hostname = sanitizeHost(os.Getenv("DIALTONE_HOSTNAME"))
	}
	if stateDir == "" {
		stateDir = cfg.StateDir
	}
	if !filepath.IsAbs(stateDir) {
		stateDir = filepath.Join(repoRoot, stateDir)
	}

	stateRoot := defaultDialtoneStateDir()
	pidFile := filepath.Join(stateRoot, "run", fmt.Sprintf("tsnet-keepalive-%s.pid", hostname))
	logFile := filepath.Join(stateRoot, "logs", fmt.Sprintf("tsnet-keepalive-%s.log", hostname))
	_ = os.MkdirAll(filepath.Dir(pidFile), 0o755)
	_ = os.MkdirAll(filepath.Dir(logFile), 0o755)
	_ = os.MkdirAll(stateDir, 0o755)

	if existing, err := os.ReadFile(pidFile); err == nil {
		pid := strings.TrimSpace(string(existing))
		if pid != "" && isAlivePID(pid) {
			return nil
		}
		_ = os.Remove(pidFile)
	}
	envFileContentPath := strings.TrimSpace(envPath)
	if envFileContentPath == "" {
		envFileContentPath = filepath.Join(repoRoot, "env", ".env")
	}

	cmd, err := makeKeepaliveCommand(repoRoot, stateDir, hostname, envFileContentPath, cfg)
	if err != nil {
		return err
	}
	logHandle, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer logHandle.Close()
	cmd.Stdout = logHandle
	cmd.Stderr = logHandle

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start tsnet keepalive failed: %w", err)
	}
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0o644); err != nil {
		return err
	}
	return nil
}

func makeKeepaliveCommand(repoRoot, stateDir, hostname, envFile string, cfg tsnetConfig) (*exec.Cmd, error) {
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}

	envs := withoutTSAPI()
	envs = append(envs,
		fmt.Sprintf("DIALTONE_TSNET_STATE_DIR=%s", stateDir),
		fmt.Sprintf("DIALTONE_HOSTNAME=%s", hostname),
		fmt.Sprintf("%s=%s", cfg.AuthKeyEnv, os.Getenv(cfg.AuthKeyEnv)),
	)
	envs = append(envs, "DIALTONE_ENV_FILE="+envFile)

	cliDir := filepath.Join(repoRoot, "src", "mods", "tsnet", "v1", "cli")
	args := []string{"run", cliDir, "keepalive", "--state-dir", stateDir, "--host", hostname}
	cmdArgs := make([]string, 0, len(args)+1)
	cmdArgs = append(cmdArgs, goBin)
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("nohup", cmdArgs...)
	cmd.Dir = repoRoot
	cmd.Env = envs
	return cmd, nil
}

func withoutTSAPI() []string {
	filtered := make([]string, 0, len(os.Environ()))
	for _, raw := range os.Environ() {
		key, _, ok := strings.Cut(raw, "=")
		if !ok {
			continue
		}
		switch key {
		case "TS_API_KEY", "TAILSCALE_API_KEY":
			continue
		}
		filtered = append(filtered, raw)
	}
	return filtered
}

func isAlivePID(pidText string) bool {
	pid, err := strconv.Atoi(strings.TrimSpace(pidText))
	if err != nil || pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

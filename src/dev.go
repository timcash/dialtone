package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	repl "dialtone/dev/plugins/repl/src_v1/go/repl"

	"github.com/joho/godotenv"
)

var logFile *os.File

func findRepoRootFromPath(start string) (string, error) {
	cwd := start
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	cwd, _ = filepath.Abs(cwd)
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}
	return "", os.ErrNotExist
}

func prependPathEntries(entries ...string) {
	current := strings.TrimSpace(os.Getenv("PATH"))
	parts := []string{}
	seen := map[string]struct{}{}
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if _, err := os.Stat(e); err != nil {
			continue
		}
		if _, ok := seen[e]; !ok {
			parts = append(parts, e)
			seen[e] = struct{}{}
		}
	}
	for _, p := range strings.Split(current, string(os.PathListSeparator)) {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; !ok {
			parts = append(parts, p)
			seen[p] = struct{}{}
		}
	}
	_ = os.Setenv("PATH", strings.Join(parts, string(os.PathListSeparator)))
}

func bootstrapDialtoneRuntimeEnv() {
	cwd, _ := os.Getwd()
	repoRoot := strings.TrimSpace(os.Getenv("DIALTONE_REPO_ROOT"))
	if repoRoot == "" {
		if found, err := findRepoRootFromPath(cwd); err == nil {
			repoRoot = found
		}
	}
	if repoRoot != "" {
		repoRoot, _ = filepath.Abs(repoRoot)
		_ = os.Setenv("DIALTONE_REPO_ROOT", repoRoot)
		_ = os.Setenv("DIALTONE_SRC_ROOT", filepath.Join(repoRoot, "src"))
		if strings.TrimSpace(os.Getenv("DIALTONE_ENV_FILE")) == "" {
			_ = os.Setenv("DIALTONE_ENV_FILE", filepath.Join(repoRoot, "env", ".env"))
		}
	}

	depsDir := GetDialtoneEnv()
	if depsDir != "" {
		goBinDir := filepath.Join(depsDir, "go", "bin")
		bunBinDir := filepath.Join(depsDir, "bun", "bin")
		prependPathEntries(goBinDir, bunBinDir)

		goBin := filepath.Join(goBinDir, "go")
		if _, err := os.Stat(goBin); err == nil {
			_ = os.Setenv("DIALTONE_GO_BIN", goBin)
		}
		bunBin := filepath.Join(bunBinDir, "bun")
		if _, err := os.Stat(bunBin); err == nil {
			_ = os.Setenv("DIALTONE_BUN_BIN", bunBin)
		}
	}
}

func maybeReexecInNixShell() {
	if v := strings.TrimSpace(strings.ToLower(os.Getenv("DIALTONE_USE_NIX"))); v == "0" || v == "false" || v == "no" || v == "off" {
		return
	}
	if strings.TrimSpace(os.Getenv("IN_NIX_SHELL")) != "" {
		return
	}
	repoRoot := strings.TrimSpace(os.Getenv("DIALTONE_REPO_ROOT"))
	if repoRoot == "" {
		if found, err := findRepoRootFromPath(""); err == nil {
			repoRoot = found
		}
	}
	if strings.TrimSpace(repoRoot) == "" {
		return
	}
	flakePath := filepath.Join(repoRoot, "flake.nix")
	if _, err := os.Stat(flakePath); err != nil {
		return
	}
	if _, err := exec.LookPath("nix"); err != nil {
		return
	}
	script := filepath.Join(repoRoot, "dialtone.sh")
	args := append([]string{"--extra-experimental-features", "nix-command flakes", "develop", "path:" + repoRoot, "--command", script}, os.Args[1:]...)
	cmd := exec.Command("nix", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		logs.Error("failed to enter nix shell: %v", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func initLogger() {
	mirrorStdout := strings.TrimSpace(os.Getenv("DIALTONE_LOG_STDOUT")) == "1"
	// No-arg invocation is interactive entry; always surface logs to stdout.
	if !mirrorStdout && len(os.Args) < 2 {
		mirrorStdout = true
	}
	if mirrorStdout {
		logs.SetOutput(os.Stdout)
	} else {
		logs.SetOutput(io.Discard)
	}

	cwd, _ := os.Getwd()
	repoRoot := cwd
	if filepath.Base(cwd) == "src" {
		repoRoot = filepath.Dir(cwd)
	}
	path := filepath.Join(repoRoot, "dialtone.log")
	var err error
	logFile, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		// Silent failure for logger init to avoid fmt dependency
	}
}

func logLine(category, msg string) {
	logs.Info("[%s] %s", category, msg)
}

// LoadConfig loads environment variables from a custom file or defaults to .env
func LoadConfig() {
	cwd, _ := os.Getwd()
	repoRoot := cwd
	if filepath.Base(cwd) == "src" {
		repoRoot = filepath.Dir(cwd)
	}

	// Try dialtone.json first
	jsonPath := filepath.Join(repoRoot, "env", "dialtone.json")
	if fileExists(jsonPath) {
		data, err := os.ReadFile(jsonPath)
		if err == nil {
			var config map[string]string
			if err := json.Unmarshal(data, &config); err == nil {
				for k, v := range config {
					if os.Getenv(k) == "" {
						os.Setenv(k, v)
					}
				}
				logLine("CONFIG", "Loaded from "+jsonPath)
			}
		}
	}

	envFile := os.Getenv("DIALTONE_ENV_FILE")
	if envFile == "" {
		envFile = "env/.env"
	}

	// Try to find the env file by looking up from current dir
	envPath := envFile
	if !filepath.IsAbs(envPath) {
		envPath = filepath.Join(repoRoot, envPath)
	}

	if fileExists(envPath) {
		if err := godotenv.Load(envPath); err != nil {
			logLine("CONFIG", "Warning: godotenv.Load failed: "+envPath)
		}
	}
}

// GetDialtoneEnv returns the directory where dependencies are installed.
func GetDialtoneEnv() string {
	env := os.Getenv("DIALTONE_ENV")
	if env != "" {
		if strings.HasPrefix(env, "~") {
			home, _ := os.UserHomeDir()
			env = filepath.Join(home, env[1:])
		}
		absEnv, _ := filepath.Abs(env)
		return absEnv
	}
	// Fallback to default
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dialtone_env")
}

type Requirement struct {
	Tool    string
	Version string
}

type MissingInstall struct {
	Tool    string
	Command string
	Why     string
}

func EnsureRequirements(reqs []Requirement) error {
	for _, req := range reqs {
		if err := EnsureRequirement(req); err != nil {
			return err
		}
	}
	return nil
}

func EnsureRequirement(req Requirement) error {
	switch req.Tool {
	case "go":
		return ensureGoRequirement(req.Version)
	case "bun":
		return ensureBunRequirement(req.Version)
	default:
		return logs.Errorf("unsupported install requirement tool: %s", req.Tool)
	}
}

func ensureGoRequirement(version string) error {
	depsDir := GetDialtoneEnv()
	goBinName := "go"
	if runtime.GOOS == "windows" {
		goBinName = "go.exe"
	}
	goBin := filepath.Join(depsDir, "go", "bin", goBinName)
	if _, err := os.Stat(goBin); os.IsNotExist(err) {
		logs.Info("[install] Go missing; running ./dialtone.sh go src_v1 install")
		cmd := exec.Command("./dialtone.sh", "go", "src_v1", "install")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return logs.Errorf("failed to install Go: %w", err)
		}
	}

	if version == "" {
		return nil
	}

	out, err := exec.Command(goBin, "version").CombinedOutput()
	if err != nil {
		return logs.Errorf("failed checking go version: %w", err)
	}
	want := "go" + version
	if !strings.Contains(string(out), want) {
		return logs.Errorf("go version mismatch: want %s, got %s", want, strings.TrimSpace(string(out)))
	}
	return nil
}

func ensureBunRequirement(version string) error {
	depsDir := GetDialtoneEnv()
	bunBinName := "bun"
	if runtime.GOOS == "windows" {
		bunBinName = "bun.exe"
	}
	bunBin := filepath.Join(depsDir, "bun", "bin", bunBinName)
	if _, err := os.Stat(bunBin); os.IsNotExist(err) {
		logs.Info("[install] Bun missing; installing via ./dialtone.sh bun src_v1 install")
		cmd := exec.Command("./dialtone.sh", "bun", "src_v1", "install")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return logs.Errorf("failed to install Bun: %w", err)
		}
	}

	if version == "" || version == "latest" {
		return nil
	}

	out, err := exec.Command(bunBin, "--version").CombinedOutput()
	if err != nil {
		return logs.Errorf("failed checking bun version: %w", err)
	}
	got := strings.TrimSpace(string(out))
	if got != version {
		return logs.Errorf("bun version mismatch: want %s, got %s", version, got)
	}
	return nil
}

func main() {
	bootstrapDialtoneRuntimeEnv()
	maybeReexecInNixShell()
	initLogger()
	LoadConfig()
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()

	if len(os.Args) < 2 {
		missing := detectMissingForREPL()
		if len(missing) > 0 {
			logs.Info("DIALTONE> REPL prerequisites are missing.")
			for _, m := range missing {
				logs.Info("DIALTONE> - %s: %s", m.Tool, m.Why)
				logs.Info("DIALTONE>   install: %s", m.Command)
			}
			logs.Info("DIALTONE> After installing, run ./dialtone.sh again.")
			return
		}
		if err := startDefaultMultiplayerREPL(); err != nil {
			logs.Error("REPL error: %v", err)
			os.Exit(1)
		}
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "help", "-h", "--help":
		printDevUsage()
	case "branch":
		runBranch(args)
	case "plugins":
		listPlugins()
	case "mods":
		if err := runMods(args); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			logs.Error("mods command failed: %v", err)
			os.Exit(1)
		}
	case "dev":
		if len(args) > 0 && args[0] == "install" {
			runDevInstall()
			return
		}
		logs.Error("Unknown dev command: %v", args)
	default:
		if err := runPluginScaffold(command, args); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			logs.Error("Orchestrator error: %v", err)
			os.Exit(1)
		}
	}
}

func detectMissingForREPL() []MissingInstall {
	missing := []MissingInstall{}
	envDir := GetDialtoneEnv()
	goBinName := "go"
	bunBinName := "bun"
	installPrefix := "./dialtone.sh"
	if runtime.GOOS == "windows" {
		goBinName = "go.exe"
		bunBinName = "bun.exe"
		installPrefix = ".\\dialtone.ps1"
	}
	goBin := filepath.Join(envDir, "go", "bin", goBinName)
	if _, err := os.Stat(goBin); err != nil {
		missing = append(missing, MissingInstall{
			Tool:    "Go runtime",
			Command: installPrefix + " go src_v1 install",
			Why:     "required to run plugin scaffolds and subtones",
		})
	}
	bunBin := filepath.Join(envDir, "bun", "bin", bunBinName)
	if _, err := os.Stat(bunBin); err != nil {
		missing = append(missing, MissingInstall{
			Tool:    "Bun runtime",
			Command: installPrefix + " bun src_v1 install",
			Why:     "required for plugins that build or run JS/TS UIs",
		})
	}
	return missing
}

func runDevInstall() {
	missing := detectMissingForREPL()
	if len(missing) == 0 {
		logs.Info("Managed runtimes already installed.")
		logs.Info("Ready. You can now run plugin commands (install/build/test) via DIALTONE.")
		return
	}

	needGo := false
	needBun := false
	for _, m := range missing {
		switch m.Tool {
		case "Go runtime":
			needGo = true
		case "Bun runtime":
			needBun = true
		}
	}

	if needGo {
		logs.Info("Installing managed Go runtime...")
		if err := runPluginScaffold("go", []string{"src_v1", "install", "--latest"}); err != nil {
			logs.Error("Go install failed: %v", err)
			return
		}
	}
	if needBun {
		logs.Info("Installing managed Bun runtime...")
		if err := runPluginScaffold("bun", []string{"src_v1", "install"}); err != nil {
			logs.Error("Bun install failed: %v", err)
			return
		}
	}

	logs.Info("Bootstrap complete. Initializing dev.go scaffold...")
	logs.Info("Ready. You can now run plugin commands (install/build/test) via DIALTONE.")
}

func printDevUsage() {
	script := "./dialtone.sh"
	if runtime.GOOS == "windows" {
		script = ".\\dialtone.cmd"
	}
	logs.Info("Usage: %s <command> [options]", script)
	logs.Info("Global flags:")
	logs.Info("  --stdout             Mirror logs to stdout (logs still publish to NATS)")
	logs.Info("")
	logs.Info("Dev orchestrator commands:")
	logs.Info("  plugins              List available plugin scaffolds")
	logs.Info("  branch <name>        Create or checkout a feature branch")
	logs.Info("  help                 Show this help")
	logs.Info("")
	logs.Info("Plugin routing:")
	logs.Info("  <plugin> <args...>   Run <plugin>/{scaffold|cli}/main.go in src/plugins or src/mods (or scaffold.sh/cli.sh)")
	logs.Info("")
	logs.Info("Examples:")
	logs.Info("  ./dialtone.sh go install --latest")
	logs.Info("  ./dialtone.sh go exec version")
	logs.Info("  ./dialtone.sh robot install src_v1")
	logs.Info("  ./dialtone.sh dag install src_v3")
	logs.Info("  ./dialtone.sh mods help")
	logs.Info("  ./dialtone.sh gemini run --task task.md")
}

func listPlugins() {
	roots := []string{"mods", "plugins"}
	seen := map[string]struct{}{}
	logs.Info("Available commands with scaffold:")
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if _, exists := seen[name]; exists {
				continue
			}
			goScaffold := filepath.Join(root, name, "scaffold", "main.go")
			cliScaffold := filepath.Join(root, name, "cli", "main.go")
			shScaffold := filepath.Join(root, name, "scaffold.sh")
			cliShell := filepath.Join(root, name, "cli.sh")
			if fileExists(goScaffold) || fileExists(shScaffold) {
				logs.Info("  - %s (%s)", name, root)
				seen[name] = struct{}{}
				continue
			}
			if fileExists(cliScaffold) || fileExists(cliShell) {
				logs.Info("  - %s (%s)", name, root)
				seen[name] = struct{}{}
			}
		}
	}
}

func runPluginScaffold(plugin string, args []string) error {
	roots := []string{"plugins", "mods"}
	pluginDir := ""
	var fallbackDir string
	for _, root := range roots {
		candidate := filepath.Join(root, plugin)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			goScaffold := filepath.Join(candidate, "scaffold", "main.go")
			cliScaffold := filepath.Join(candidate, "cli", "main.go")
			shScaffold := filepath.Join(candidate, "scaffold.sh")
			if fileExists(goScaffold) || fileExists(cliScaffold) || fileExists(shScaffold) {
				pluginDir = candidate
				break
			}
			if fallbackDir == "" {
				fallbackDir = candidate
			}
		}
	}
	if pluginDir == "" && fallbackDir != "" {
		pluginDir = fallbackDir
	}
	if pluginDir == "" {
		return logs.Errorf("unknown plugin: %s", plugin)
	}

	goScaffold := filepath.Join(pluginDir, "scaffold", "main.go")
	cliScaffold := filepath.Join(pluginDir, "cli", "main.go")
	if fileExists(goScaffold) {
		var cmd *exec.Cmd
		if fileExists(filepath.Join(pluginDir, "go.mod")) {
			cmd = exec.Command("go", append([]string{"run", "./scaffold/main.go"}, args...)...)
			cmd.Dir = pluginDir
		} else {
			cmd = exec.Command("go", append([]string{"run", "./" + filepath.ToSlash(goScaffold)}, args...)...)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	}

	if fileExists(cliScaffold) {
		var cmd *exec.Cmd
		if fileExists(filepath.Join(pluginDir, "go.mod")) {
			cmd = exec.Command("go", append([]string{"run", "./cli/main.go"}, args...)...)
			cmd.Dir = pluginDir
		} else {
			cmd = exec.Command("go", append([]string{"run", "./" + filepath.ToSlash(cliScaffold)}, args...)...)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	}

	shScaffold := filepath.Join(pluginDir, "scaffold.sh")
	if fileExists(shScaffold) {
		cmd := exec.Command("bash", append([]string{shScaffold}, args...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	}

	return logs.Errorf("plugin %s has no scaffold/cli main.go or scaffold.sh/cli.sh in candidate roots", plugin)
}

func runMods(args []string) error {
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	cmd := exec.Command(goBin, append([]string{"run", "./mods/mod/v1/main.go"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func runBranch(args []string) {
	if len(args) == 0 {
		logs.Error("Usage: ./dialtone.sh branch <name>")
		os.Exit(1)
	}

	branchName := args[0]
	check := exec.Command("git", "branch", "--list", branchName)
	output, err := check.Output()
	if err != nil {
		logs.Error("Failed to check branches: %v", err)
		os.Exit(1)
	}

	var cmd *exec.Cmd
	if strings.TrimSpace(string(output)) != "" {
		logs.Info("Branch '%s' exists, checking out...", branchName)
		cmd = exec.Command("git", "checkout", branchName)
	} else {
		logs.Info("Creating new branch '%s'...", branchName)
		cmd = exec.Command("git", "checkout", "-b", branchName)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logs.Error("Git operation failed: %v", err)
		os.Exit(1)
	}

	logs.Info("Now on branch: %s", branchName)
}

func startDefaultMultiplayerREPL() error {
	room := strings.TrimSpace(os.Getenv("DIALTONE_REPL_ROOM"))
	if room == "" {
		room = "index"
	}
	clientURL := strings.TrimSpace(os.Getenv("DIALTONE_REPL_NATS_URL"))
	if clientURL == "" {
		clientURL = "nats://127.0.0.1:4222"
	}
	leaderURL := strings.TrimSpace(os.Getenv("DIALTONE_REPL_LEADER_NATS_URL"))
	if leaderURL == "" {
		leaderURL = "nats://0.0.0.0:4222"
	}

	joinArgs := []string{"--nats-url", clientURL, room}
	if endpointReachable(clientURL, 700*time.Millisecond) {
		return repl.RunJoin(joinArgs)
	}
	if !replAutostartEnabled() {
		return fmt.Errorf("no REPL daemon detected on %s (autostart disabled). start daemon with: ./dialtone.sh repl src_v1 service --mode run --room %s", clientURL, room)
	}

	logs.Info("DIALTONE> No REPL leader detected on %s; starting leader for room %s", clientURL, room)
	cmd, err := startLocalLeaderProcess(leaderURL, room)
	if err != nil {
		return err
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	if !waitForEndpoint(clientURL, 6*time.Second) {
		return fmt.Errorf("leader started but nats endpoint did not become reachable: %s", clientURL)
	}
	return repl.RunJoin(joinArgs)
}

func replAutostartEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("DIALTONE_REPL_AUTOSTART")))
	switch v {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func startLocalLeaderProcess(natsURL, room string) (*exec.Cmd, error) {
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	cmd := exec.Command(goBin, "run", "./plugins/repl/scaffold/main.go", "src_v1", "leader", "--embedded-nats", "--nats-url", natsURL, "--room", room)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil
	cmd.Dir, _ = os.Getwd()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func waitForEndpoint(natsURL string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if endpointReachable(natsURL, 500*time.Millisecond) {
			return true
		}
		time.Sleep(150 * time.Millisecond)
	}
	return false
}

func endpointReachable(natsURL string, timeout time.Duration) bool {
	u, err := url.Parse(strings.TrimSpace(natsURL))
	if err != nil {
		return false
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "4222"
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

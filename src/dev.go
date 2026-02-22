package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	repl "dialtone/dev/plugins/repl/src_v1/go/repl"

	"github.com/joho/godotenv"
)

var logFile *os.File

func initLogger() {
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
	envFile := os.Getenv("DIALTONE_ENV_FILE")
	if envFile == "" {
		envFile = "env/.env"
	}

	// Try to find the env file by looking up from current dir
	cwd, _ := os.Getwd()
	repoRoot := cwd
	if filepath.Base(cwd) == "src" {
		repoRoot = filepath.Dir(cwd)
	}
	envPath := filepath.Join(repoRoot, envFile)

	if err := godotenv.Load(envPath); err != nil {
		logLine("CONFIG", "Warning: godotenv.Load failed: "+envPath)
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
	goBin := filepath.Join(depsDir, "go", "bin", "go")
	if _, err := os.Stat(goBin); os.IsNotExist(err) {
		logs.Info("[install] Go missing; running ./dialtone.sh go install")
		cmd := exec.Command("./dialtone.sh", "go", "install")
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
	bunBin := filepath.Join(depsDir, "bun", "bin", "bun")
	if _, err := os.Stat(bunBin); os.IsNotExist(err) {
		logs.Info("[install] Bun missing; installing via ./dialtone.sh bun install")
		cmd := exec.Command("./dialtone.sh", "bun", "install")
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
		if err := repl.Start(logLine); err != nil {
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
	goBin := filepath.Join(envDir, "go", "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		missing = append(missing, MissingInstall{
			Tool:    "Go runtime",
			Command: "./dialtone.sh go src_v1 install",
			Why:     "required to run plugin scaffolds and subtones",
		})
	}
	bunBin := filepath.Join(envDir, "bun", "bin", "bun")
	if _, err := os.Stat(bunBin); err != nil {
		missing = append(missing, MissingInstall{
			Tool:    "Bun runtime",
			Command: "./dialtone.sh bun src_v1 install",
			Why:     "required for plugins that build or run JS/TS UIs",
		})
	}
	return missing
}

func runDevInstall() {
	logs.Info("Installing latest Go runtime for managed ./dialtone.sh go commands...")
	cwd, _ := os.Getwd()
	installer := filepath.Join(cwd, "plugins/go/install.sh")
	if !fileExists(installer) {
		logs.Error("Installer missing: %s", installer)
		return
	}

	cmd := exec.Command("bash", installer, "--latest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logs.Error("Install failed.")
		return
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
	logs.Info("")
	logs.Info("Dev orchestrator commands:")
	logs.Info("  plugins              List available plugin scaffolds")
	logs.Info("  branch <name>        Create or checkout a feature branch")
	logs.Info("  help                 Show this help")
	logs.Info("")
	logs.Info("Plugin routing:")
	logs.Info("  <plugin> <args...>   Run src/plugins/<plugin>/scaffold/main.go (or scaffold.sh)")
	logs.Info("")
	logs.Info("Examples:")
	logs.Info("  ./dialtone.sh go install --latest")
	logs.Info("  ./dialtone.sh go exec version")
	logs.Info("  ./dialtone.sh robot install src_v1")
	logs.Info("  ./dialtone.sh dag install src_v3")
	logs.Info("  ./dialtone.sh gemini run --task task.md")
}

func listPlugins() {
	root := "plugins"
	entries, err := os.ReadDir(root)
	if err != nil {
		logs.Error("Failed to read plugins directory: %v", err)
		return
	}
	logs.Info("Available plugins with scaffold:")
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		goScaffold := filepath.Join(root, name, "scaffold", "main.go")
		shScaffold := filepath.Join(root, name, "scaffold.sh")
		if fileExists(goScaffold) || fileExists(shScaffold) {
			logs.Info("  - %s", name)
		}
	}
}

func runPluginScaffold(plugin string, args []string) error {
	pluginDir := filepath.Join("plugins", plugin)
	info, err := os.Stat(pluginDir)
	if err != nil || !info.IsDir() {
		return logs.Errorf("unknown plugin: %s", plugin)
	}

	goScaffold := filepath.Join(pluginDir, "scaffold", "main.go")
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

	shScaffold := filepath.Join(pluginDir, "scaffold.sh")
	if fileExists(shScaffold) {
		cmd := exec.Command("bash", append([]string{shScaffold}, args...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	}

	return logs.Errorf("plugin %s has no scaffold/main.go or scaffold.sh", plugin)
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

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"dialtone/dev/plugins/proc/src_v1/go/proc"

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
		fmt.Printf("Warning: failed to open dialtone.log: %v\n", err)
	}
}

func logLine(category, msg string) {
	if logFile == nil {
		return
	}
	ts := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	fmt.Fprintf(logFile, "[%s | INFO | %s] %s\n", ts, category, msg)
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
		logLine("CONFIG", fmt.Sprintf("Warning: godotenv.Load(%s) failed: %v", envPath, err))
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
		return fmt.Errorf("unsupported install requirement tool: %s", req.Tool)
	}
}

func ensureGoRequirement(version string) error {
	depsDir := GetDialtoneEnv()
	goBin := filepath.Join(depsDir, "go", "bin", "go")
	if _, err := os.Stat(goBin); os.IsNotExist(err) {
		fmt.Printf("[install] Go missing; running ./dialtone.sh go install\n")
		cmd := exec.Command("./dialtone.sh", "go", "install")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install Go: %w", err)
		}
	}

	if version == "" {
		return nil
	}

	out, err := exec.Command(goBin, "version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed checking go version: %w", err)
	}
	want := "go" + version
	if !strings.Contains(string(out), want) {
		return fmt.Errorf("go version mismatch: want %s, got %s", want, strings.TrimSpace(string(out)))
	}
	return nil
}

func ensureBunRequirement(version string) error {
	depsDir := GetDialtoneEnv()
	bunBin := filepath.Join(depsDir, "bun", "bin", "bun")
	if _, err := os.Stat(bunBin); os.IsNotExist(err) {
		fmt.Printf("[install] Bun missing; installing via ./dialtone.sh bun install\n")
		cmd := exec.Command("./dialtone.sh", "bun", "install")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install Bun: %w", err)
		}
	}

	if version == "" || version == "latest" {
		return nil
	}

	out, err := exec.Command(bunBin, "--version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed checking bun version: %w", err)
	}
	got := strings.TrimSpace(string(out))
	if got != version {
		return fmt.Errorf("bun version mismatch: want %s, got %s", version, got)
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
		startREPL()
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
		fmt.Printf("Unknown dev command: %v\n", args)
	default:
		if err := runPluginScaffold(command, args); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			fmt.Printf("Orchestrator error: %v\n", err)
			os.Exit(1)
		}
	}
}

func runDevInstall() {
	output := func(msg string) {
		fmt.Println(msg)
		logLine("EXEC", msg)
	}

	output("Installing latest Go runtime for managed ./dialtone.sh go commands...")
	cwd, _ := os.Getwd()
	// Since we are in 'src', installer is at plugins/go/install.sh
	installer := filepath.Join(cwd, "plugins/go/install.sh")
	if !fileExists(installer) {
		output("Installer missing: " + installer)
		return
	}

	cmd := exec.Command("bash", installer, "--latest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		output("Install failed.")
		return
	}

	output("Bootstrap complete. Initializing dev.go scaffold...")
	output("Ready. You can now run plugin commands (install/build/test) via DIALTONE.")
}

func isTTY() bool {
	fi, _ := os.Stdin.Stat()
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func startREPL() {
	say := func(msg string) {
		fmt.Println("DIALTONE> " + msg)
		logLine("REPL", "DIALTONE> "+msg)
	}

	say("Virtual Librarian online.")
	say("Type 'help' for commands, or 'exit' to quit.")

	scanner := bufio.NewScanner(os.Stdin)
	tty := isTTY()

	for {
		fmt.Print("USER-1> ")
		if !scanner.Scan() {
			say("Session closed.")
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if !tty {
			fmt.Println(line)
		}

		logLine("REPL", "USER-1> "+line)

		if line == "exit" || line == "quit" {
			say("Goodbye.")
			break
		}

		if line == "help" {
			printREPLHelp()
			continue
		}

		if line == "ps" {
			proc.ListProcesses()
			continue
		}

		// Handle @DIALTONE or @dialtone.sh prefix (optional now)
		cmdStr := line
		if strings.HasPrefix(line, "@DIALTONE ") {
			cmdStr = line[len("@DIALTONE "):]
		} else if strings.HasPrefix(line, "@dialtone.sh ") {
			cmdStr = line[len("@dialtone.sh "):]
		}

		args := strings.Fields(cmdStr)
		if len(args) == 0 {
			continue
		}

		cmdName := args[0]
		if len(args) > 1 {
			cmdName += " " + args[1]
		}

		if strings.Join(args, " ") == "proc test src_v1" {
			proc.RunTestSrcV1()
			continue
		}

		isBackground := false
		if len(args) > 0 && args[len(args)-1] == "&" {
			isBackground = true
			args = args[:len(args)-1]
			cmdName = strings.TrimSuffix(cmdName, " &")
		}

		say(fmt.Sprintf("Request received. Spawning subtone for %s...", cmdName))
		if isBackground {
			go proc.RunSubtone(args)
		} else {
			proc.RunSubtone(args)
		}
	}
}

func printREPLHelp() {
	content := `Help

### Bootstrap
` + "`" + `@DIALTONE dev install` + "`" + `
Install latest Go and bootstrap dev.go command scaffold

### Plugins
` + "`" + `robot install src_v1` + "`" + `
Install robot src_v1 dependencies

` + "`" + `dag install src_v3` + "`" + `
Install dag src_v3 dependencies

### System
` + "`" + `ps` + "`" + `
List active subtones

` + "`" + `<any command>` + "`" + `
Forward to @./dialtone.sh <command>`

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if i == 0 {
			fmt.Println("DIALTONE> " + line)
			logLine("REPL", "DIALTONE> "+line)
		} else {
			fmt.Println(line)
			logLine("REPL", line)
		}
	}
}

func printDevUsage() {
	script := "./dialtone.sh"
	if runtime.GOOS == "windows" {
		script = ".\\dialtone.cmd"
	}
	fmt.Printf("Usage: %s <command> [options]\n", script)
	fmt.Println()
	fmt.Println("Dev orchestrator commands:")
	fmt.Println("  plugins              List available plugin scaffolds")
	fmt.Println("  branch <name>        Create or checkout a feature branch")
	fmt.Println("  help                 Show this help")
	fmt.Println()
	fmt.Println("Plugin routing:")
	fmt.Println("  <plugin> <args...>   Run src/plugins/<plugin>/scaffold/main.go (or scaffold.sh)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  ./dialtone.sh go install --latest")
	fmt.Println("  ./dialtone.sh go exec version")
	fmt.Println("  ./dialtone.sh robot install src_v1")
	fmt.Println("  ./dialtone.sh dag install src_v3")
}

func listPlugins() {
	root := "plugins"
	entries, err := os.ReadDir(root)
	if err != nil {
		fmt.Printf("Failed to read plugins directory: %v\n", err)
		return
	}
	fmt.Println("Available plugins with scaffold:")
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		goScaffold := filepath.Join(root, name, "scaffold", "main.go")
		shScaffold := filepath.Join(root, name, "scaffold.sh")
		if fileExists(goScaffold) || fileExists(shScaffold) {
			fmt.Printf("  - %s\n", name)
		}
	}
}

func runPluginScaffold(plugin string, args []string) error {
	pluginDir := filepath.Join("plugins", plugin)
	info, err := os.Stat(pluginDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("unknown plugin: %s", plugin)
	}

	goScaffold := filepath.Join(pluginDir, "scaffold", "main.go")
	if fileExists(goScaffold) {
		var cmd *exec.Cmd
		if fileExists(filepath.Join(pluginDir, "go.mod")) {
			// Plugin has its own module, run from plugin dir
			cmd = exec.Command("go", append([]string{"run", "./scaffold/main.go"}, args...)...)
			cmd.Dir = pluginDir
		} else {
			// Plugin is part of main module, run from 'src'
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

	return fmt.Errorf("plugin %s has no scaffold/main.go or scaffold.sh", plugin)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func runBranch(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ./dialtone.sh branch <name>")
		os.Exit(1)
	}

	branchName := args[0]
	check := exec.Command("git", "branch", "--list", branchName)
	output, err := check.Output()
	if err != nil {
		fmt.Printf("Failed to check branches: %v\n", err)
		os.Exit(1)
	}

	var cmd *exec.Cmd
	if strings.TrimSpace(string(output)) != "" {
		fmt.Printf("Branch '%s' exists, checking out...\n", branchName)
		cmd = exec.Command("git", "checkout", branchName)
	} else {
		fmt.Printf("Creating new branch '%s'...\n", branchName)
		cmd = exec.Command("git", "checkout", "-b", branchName)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Git operation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Now on branch: %s\n", branchName)
}

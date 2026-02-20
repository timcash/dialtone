package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
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

func main() {
	initLogger()
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

		// Handle @DIALTONE or @dialtone.sh prefix
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
		say(fmt.Sprintf("Request received. Spawning subtone for %s...", cmdName))
		runSubtone(args)
	}
}

func printREPLHelp() {
	content := `Help

### Bootstrap
` + "`" + `@DIALTONE dev install` + "`" + `
Install latest Go and bootstrap dev.go command scaffold

### Plugins
` + "`" + `@DIALTONE robot install src_v1` + "`" + `
Install robot src_v1 dependencies

### System
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

func runSubtone(args []string) {
	cwd, _ := os.Getwd()
	repoRoot := cwd
	if filepath.Base(cwd) == "src" {
		repoRoot = filepath.Dir(cwd)
	}
	dialtoneSh := filepath.Join(repoRoot, "dialtone.sh")

	cmd := exec.Command(dialtoneSh, args...)
	cmd.Dir = repoRoot

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		fmt.Printf("DIALTONE> Failed to start subtone: %v\n", err)
		logLine("REPL", fmt.Sprintf("Failed to start subtone: %v", err))
		return
	}

	pid := cmd.Process.Pid
	fmt.Printf("DIALTONE> Spawning subtone subprocess via PID %d...\n", pid)
	fmt.Printf("DIALTONE> Streaming stdout/stderr from subtone PID %d.\n", pid)
	logLine("REPL", fmt.Sprintf("Spawning subtone subprocess via PID %d...", pid))

	// Combine stdout and stderr
	reader := io.MultiReader(stdout, stderr)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		// If the line already starts with DIALTONE>, strip it to avoid double prefixing
		// when streaming subtone output that was produced by another dev.go instance.
		displayLine := line
		if strings.HasPrefix(line, "DIALTONE> ") {
			displayLine = line[len("DIALTONE> "):]
		}
		prefix := fmt.Sprintf("DIALTONE:%d> ", pid)
		fmt.Printf("%s%s\n", prefix, displayLine)
		logLine("REPL", prefix+displayLine)
	}

	err := cmd.Wait()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	fmt.Printf("DIALTONE> Process %d exited with code %d.\n", pid, exitCode)
	logLine("REPL", fmt.Sprintf("Process %d exited with code %d.", pid, exitCode))
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
		cmd := exec.Command("go", append([]string{"run", "./scaffold/main.go"}, args...)...)
		cmd.Dir = pluginDir
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

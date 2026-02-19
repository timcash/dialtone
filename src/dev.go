package dialtone

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ExecuteDev routes commands to plugin scaffolds.
func ExecuteDev() {
	if len(os.Args) < 2 {
		printDevUsage()
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "help", "-h", "--help":
		printDevUsage()
		return
	case "branch":
		runBranch(args)
		return
	case "plugins":
		listPlugins()
		return
	default:
		if err := runPluginScaffold(command, args); err != nil {
			fmt.Printf("Orchestrator error: %v\n", err)
			os.Exit(1)
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

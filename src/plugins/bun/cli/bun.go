package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"dialtone/dev/plugins/logs/src_v1/go"
)

func RunBun(args []string) {
	if len(args) == 0 {
		printUsage()
		return
	}

	command := args[0]
	switch command {
	case "help", "-h", "--help":
		printUsage()
	case "install":
		runInstall(args[1:])
	case "exec":
		runExec(args[1:])
	case "version":
		runVersion()
	default:
		// Default to exec if command is unknown? 
		// Actually, let's just use exec for everything else if it looks like a bun arg
		runExec(args)
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh bun <command> [args]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  install              Install dependencies (bun install)")
	fmt.Println("  exec [--cwd <dir>]   Run bun command in directory")
	fmt.Println("  version              Print bun version")
}

func runInstall(args []string) {
	depsDir := logs.GetDialtoneEnv()
	bunBin := filepath.Join(depsDir, "bun", "bin", "bun")
	
	cwd, bunArgs := extractCwd(args)
	
	cmd := exec.Command(bunBin, append([]string{"install"}, bunArgs...)...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

func runVersion() {
	depsDir := logs.GetDialtoneEnv()
	bunBin := filepath.Join(depsDir, "bun", "bin", "bun")
	cmd := exec.Command(bunBin, "--version")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func runExec(args []string) {
	if len(args) == 0 {
		logs.Fatal("Usage: ./dialtone.sh bun exec <args...>")
	}

	cwd, bunArgs := extractCwd(args)
	if len(bunArgs) == 0 {
		logs.Fatal("Usage: ./dialtone.sh bun exec <args...>")
	}

	depsDir := logs.GetDialtoneEnv()
	bunBin := filepath.Join(depsDir, "bun", "bin", "bun")
	if _, err := os.Stat(bunBin); os.IsNotExist(err) {
		logs.Fatal("Bun toolchain not found at %s. Run './dialtone.sh install' first.", bunBin)
	}

	// Prepend managed bun and node bins so spawned scripts resolve managed tooling first.
	newPath := filepath.Join(depsDir, "bun", "bin") + string(os.PathListSeparator) + filepath.Join(depsDir, "node", "bin") + string(os.PathListSeparator) + os.Getenv("PATH")
	_ = os.Setenv("PATH", newPath)

	cmd := exec.Command(bunBin, bunArgs...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
		logs.Fatal("Bun command failed: %v", err)
	}
}

func extractCwd(args []string) (string, []string) {
	var cwd string
	filtered := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--cwd" {
			if i+1 >= len(args) {
				logs.Fatal("Missing value for --cwd")
			}
			cwd = args[i+1]
			i++
			continue
		}
		if len(arg) > len("--cwd=") && arg[:len("--cwd=")] == "--cwd=" {
			cwd = arg[len("--cwd="):]
			continue
		}
		filtered = append(filtered, arg)
	}
	return cwd, filtered
}

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {
	case "help", "-h", "--help":
		printUsage()
	case "install":
		runInstall(os.Args[2:])
	case "exec", "run":
		runExec(os.Args[2:])
	case "version":
		runExec([]string{"version"})
	default:
		fmt.Printf("Unknown go scaffold command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh go <command> [args]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  install [--latest]   Install managed Go runtime")
	fmt.Println("  exec <args...>       Run managed go command")
	fmt.Println("  run <args...>        Alias for exec")
	fmt.Println("  version              Print managed go version")
}

func runInstall(args []string) {
	pluginDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Failed to resolve go plugin directory: %v\n", err)
		os.Exit(1)
	}
	installer := filepath.Join(pluginDir, "install.sh")
	cmd := exec.Command("bash", append([]string{installer}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

func runExec(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ./dialtone.sh go exec <args...>")
		os.Exit(1)
	}

	dialtoneEnv := os.Getenv("DIALTONE_ENV")
	if dialtoneEnv == "" {
		fmt.Println("DIALTONE_ENV is not set")
		os.Exit(1)
	}

	goBin := filepath.Join(dialtoneEnv, "go", "bin", "go")
	cmd := exec.Command(goBin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Printf("go command failed: %v\n", err)
		os.Exit(1)
	}
}

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func Run(args []string) error {
	if len(args) < 1 {
		printUsage()
		return fmt.Errorf("no command provided")
	}

	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "dev":
		return runNpm("dev", cmdArgs)
	case "build":
		return runNpm("build", cmdArgs)
	case "install":
		return runNpm("install", cmdArgs)
	case "mock-data":
		RunMockData(cmdArgs)
		return nil
	case "kill":
		runKill()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown ui command: %s", command)
	}
}

func printUsage() {
	fmt.Println("Usage: dialtone ui <command> [args]")
	fmt.Println("\nCommands:")
	fmt.Println("  dev       Run the dev server (npm run dev)")
	fmt.Println("  build     Build the web UI (npm run build)")
	fmt.Println("  install   Install dependencies (npm install)")
	fmt.Println("  mock-data Start mock data server for testing")
	fmt.Println("  kill      Kill running UI processes (dev, mock-data)")
}

func runKill() {
	fmt.Println(">> Killing UI processes...")

	// 1. Process Name Kill
	exec.Command("pkill", "-f", "vite").Run()
	exec.Command("pkill", "-f", "dialtone ui mock-data").Run()
	exec.Command("pkill", "-f", "dialtone ui dev").Run()

	// 2. Port Kill (Force cleanup of ports)
	ports := []string{"4222", "4223", "8080", "5173", "5174"}
	for _, p := range ports {
		// fuser -k -n tcp PORT
		exec.Command("fuser", "-k", "-n", "tcp", p).Run()
	}

	fmt.Println(">> UI processes terminated.")
}

func runNpm(script string, args []string) error {
	// Locate npm in DIALTONE_ENV
	envDir := os.Getenv("DIALTONE_ENV")
	if envDir == "" {
		return fmt.Errorf("DIALTONE_ENV not set")
	}

	envDir, _ = filepath.Abs(envDir)

	npmPath := filepath.Join(envDir, "node", "bin", "npm")
	nodePath := filepath.Join(envDir, "node", "bin")

	if _, err := os.Stat(npmPath); os.IsNotExist(err) {
		fmt.Printf("WARNING: npm not found at %s. Falling back to system npm.\n", npmPath)
		npmPath = "npm"
	} else {
		currentPath := os.Getenv("PATH")
		os.Setenv("PATH", nodePath+string(os.PathListSeparator)+currentPath)
	}

	nodeBin := filepath.Join(nodePath, "node")
	webDir := filepath.Join("src", "core", "web")

	var cmd *exec.Cmd
	if script == "install" {
		cmd = exec.Command(nodeBin, npmPath, "install")
		cmd.Args = append(cmd.Args, args...)
	} else {
		cmd = exec.Command(nodeBin, npmPath, "run", script, "--")
		cmd.Args = append(cmd.Args, args...)
	}

	cmd.Dir = webDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()

	fmt.Printf(">> Running: %s %s run %s (in %s)\n", nodeBin, npmPath, script, webDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ui command failed: %w", err)
	}
	return nil
}

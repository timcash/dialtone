package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func Run(args []string) {
	if len(args) < 1 {
		printUsage()
		return
	}

	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "dev":
		runNpm("dev", cmdArgs)
	case "build":
		runNpm("build", cmdArgs)
	case "install":
		runNpm("install", cmdArgs)
	case "mock-data":
		RunMockData(cmdArgs)
	case "kill":
		runKill()
	default:
		fmt.Printf("Unknown ui command: %s\n", command)
		printUsage()
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

func runNpm(script string, args []string) {
	// Locate npm in DIALTONE_ENV
	envDir := os.Getenv("DIALTONE_ENV")
	if envDir == "" {
		fmt.Println("ERROR: DIALTONE_ENV environment variable is not set.")
		fmt.Println("Please ensure you are running via 'dialtone.sh' and have a valid .env configuration.")
		os.Exit(1)
	}

	envDir, _ = filepath.Abs(envDir)

	npmPath := filepath.Join(envDir, "node", "bin", "npm")
	nodePath := filepath.Join(envDir, "node", "bin")

	// If npm binary doesn't exist at the calculated path, try PATH or error
	if _, err := os.Stat(npmPath); os.IsNotExist(err) {
		fmt.Printf("WARNING: npm not found at %s. Falling back to system npm.\n", npmPath)
		npmPath = "npm"
	} else {
		// Update PATH to include node/bin so npm can find node
		currentPath := os.Getenv("PATH")
		os.Setenv("PATH", nodePath+string(os.PathListSeparator)+currentPath)
	}

	// Locate node binary
	nodeBin := filepath.Join(nodePath, "node")

	// Web directory
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
	// Inherit environment (PATH is already updated)
	cmd.Env = os.Environ()

	fmt.Printf(">> Running: %s %s run %s (in %s)\n", nodeBin, npmPath, script, webDir)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error running ui command: %v\n", err)
		os.Exit(1)
	}
}

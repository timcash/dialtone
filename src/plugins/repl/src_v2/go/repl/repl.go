package repl

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dialtone/dev/plugins/proc/src_v1/go/proc"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

type DialtoneConfig struct {
	EnvRoot   string           `json:"DIALTONE_ENV,omitempty"`
	RepoRoot  string           `json:"DIALTONE_REPO_ROOT,omitempty"`
	UseNix    string           `json:"DIALTONE_USE_NIX,omitempty"`
	MeshNodes []sshv1.MeshNode `json:"mesh_nodes,omitempty"`
}

func RunREPLV2(args []string) error {
	repoRoot := os.Getenv("DIALTONE_REPO_ROOT")
	if repoRoot == "" {
		return fmt.Errorf("DIALTONE_REPO_ROOT is not set")
	}

	isTest := false
	for _, arg := range args {
		if arg == "--test" || arg == "-test" {
			isTest = true
			break
		}
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "USER-1"
	}
	prompt := hostname + "> "

	fmt.Println("DIALTONE> REPL v2 (Autonomous Mode) starting...")
	fmt.Println("DIALTONE> Type /help to see available slash commands.")
	
	configPath := filepath.Join(repoRoot, "env", "dialtone.json")
	dialtoneSh := filepath.Join(repoRoot, "dialtone.sh")

	// --- Interactive Loop ---
	scanner := bufio.NewScanner(os.Stdin)
	
	if isTest {
		fmt.Println("DIALTONE> [TEST] Running automated onboarding sequence...")
		commands := []string{
			"/mesh add wsl wsl.shad-artichoke.ts.net user",
			"/ssh wsl whoami",
			"exit",
		}
		for _, cmd := range commands {
			fmt.Printf("%s%s\n", prompt, cmd)
			if err := handleInput(cmd, configPath, dialtoneSh); err != nil {
				fmt.Printf("DIALTONE> [ERROR] %v\n", err)
			}
		}
		return nil
	}

	for {
		fmt.Print(prompt)
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			break
		}

		if err := handleInput(line, configPath, dialtoneSh); err != nil {
			fmt.Printf("DIALTONE> [ERROR] %v\n", err)
		}
	}

	return nil
}

func handleInput(line string, configPath string, dialtoneSh string) error {
	if !strings.HasPrefix(line, "/") {
		// Pass-through to dialtone.sh for standard commands
		executeCommand(line, dialtoneSh)
		return nil
	}

	parts := strings.Fields(line)
	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "/help":
		printHelp()
	case "/env":
		return handleEnv(args, configPath)
	case "/mesh":
		return handleMesh(args, configPath)
	case "/ssh":
		return handleSSH(args, dialtoneSh, configPath)
	case "/status":
		return printStatus(configPath)
	case "/ps":
		printManagedProcesses()
	default:
		return fmt.Errorf("unknown slash command: %s", cmd)
	}
	return nil
}

func handleEnv(args []string, path string) error {
	if len(args) < 3 || args[0] != "set" {
		return fmt.Errorf("usage: /env set <key> <value>")
	}
	config, _ := loadConfig(path)
	key, val := args[1], args[2]
	
	switch key {
	case "DIALTONE_ENV": config.EnvRoot = val
	case "DIALTONE_REPO_ROOT": config.RepoRoot = val
	case "DIALTONE_USE_NIX": config.UseNix = val
	default: return fmt.Errorf("unknown env key: %s", key)
	}
	
	return saveConfig(path, config)
}

func handleMesh(args []string, path string) error {
	if len(args) < 4 || args[0] != "add" {
		return fmt.Errorf("usage: /mesh add <name> <host> <user>")
	}
	config, _ := loadConfig(path)
	name, host, user := args[1], args[2], args[3]
	
	// Check if already exists
	for i, n := range config.MeshNodes {
		if n.Name == name {
			config.MeshNodes[i].Host = host
			config.MeshNodes[i].User = user
			fmt.Printf("DIALTONE> Updated existing mesh node: %s\n", name)
			return saveConfig(path, config)
		}
	}

	newNode := sshv1.MeshNode{
		Name:    name,
		Host:    host,
		User:    user,
		Port:    "22",
		OS:      "linux",
		Aliases: []string{name},
		HostCandidates: []string{host},
		RoutePreference: []string{"tailscale", "private"},
	}
	config.MeshNodes = append(config.MeshNodes, newNode)
	fmt.Printf("DIALTONE> Added new mesh node: %s\n", name)
	return saveConfig(path, config)
}

func handleSSH(args []string, bin string, configPath string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: /ssh <node> <command>")
	}
	node := args[0]
	cmd := strings.Join(args[1:], " ")
	
	// Map /ssh to plugin command
	pluginCmd := fmt.Sprintf("ssh src_v1 run --host %s --cmd %q", node, cmd)
	
	// Ensure the plugin knows where the config is
	os.Setenv("DIALTONE_MESH_CONFIG", configPath)
	executeCommand(pluginCmd, bin)
	return nil
}

func loadConfig(path string) (DialtoneConfig, error) {
	var config DialtoneConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(data, &config)
	return config, err
}

func saveConfig(path string, config DialtoneConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	err = os.WriteFile(path, data, 0644)
	if err == nil {
		fmt.Printf("DIALTONE> [OK] Configuration saved to %s\n", path)
	}
	return err
}

func printHelp() {
	fmt.Println("DIALTONE> Available commands:")
	fmt.Println("  /env set <K> <V>       Set environment variable in dialtone.json")
	fmt.Println("  /mesh add <N> <H> <U>  Add/Update a mesh node configuration")
	fmt.Println("  /ssh <node> <cmd>      Run a command on a mesh node via SSH plugin")
	fmt.Println("  /status                Show current configuration and health")
	fmt.Println("  /ps                    List managed subtones")
	fmt.Println("  exit                   Quit REPL")
}

func printStatus(path string) error {
	config, err := loadConfig(path)
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}
	fmt.Printf("DIALTONE> Configuration: %s\n", path)
	fmt.Printf("  DIALTONE_ENV:       %s\n", config.EnvRoot)
	fmt.Printf("  DIALTONE_REPO_ROOT:  %s\n", config.RepoRoot)
	fmt.Printf("  Mesh Nodes (%d):\n", len(config.MeshNodes))
	for _, n := range config.MeshNodes {
		fmt.Printf("    - %s (%s@%s)\n", n.Name, n.User, n.Host)
	}
	return nil
}

func printManagedProcesses() {
	procs := proc.ListManagedProcesses()
	if len(procs) == 0 {
		fmt.Println("DIALTONE> No active subtones.")
		return
	}
	fmt.Println("DIALTONE> Active Subtones:")
	fmt.Printf("%-8s %-8s %-10s %-8s %s\n", "PID", "UPTIME", "CPU%", "PORTS", "COMMAND")
	for _, p := range procs {
		fmt.Printf("%-8d %-8s %-10.1f %-8d %s\n", p.PID, p.StartedAgo, p.CPUPercent, p.PortCount, p.Command)
	}
}

func executeCommand(line string, bin string) {
	args := strings.Fields(line)
	cmd := exec.Command(bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Printf("DIALTONE> [ERROR] command failed: %v\n", err)
	}
}

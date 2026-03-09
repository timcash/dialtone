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
	EnvRoot   string            `json:"DIALTONE_ENV,omitempty"`
	RepoRoot  string            `json:"DIALTONE_REPO_ROOT,omitempty"`
	UseNix    string            `json:"DIALTONE_USE_NIX,omitempty"`
	Session   map[string]string `json:"session,omitempty"`
	MeshNodes []sshv1.MeshNode  `json:"mesh_nodes,omitempty"`
}

func RunREPLV2(args []string) error {
	repoRoot := os.Getenv("DIALTONE_REPO_ROOT")
	if repoRoot == "" {
		return fmt.Errorf("DIALTONE_REPO_ROOT is not set")
	}

	isTest := false
	isLLM := false
	for _, arg := range args {
		if arg == "--test" {
			isTest = true
		}
		if arg == "--llm" {
			isLLM = true
		}
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "USER-1"
	}
	prompt := hostname + "> "

	fmt.Println("DIALTONE> REPL v2 (Autonomous Mode) starting...")

	configPath := filepath.Join(repoRoot, "env", "dialtone.json")
	dialtoneSh := filepath.Join(repoRoot, "dialtone.sh")

	// --- 1. Automated Test Onboarding ---
	if isTest {
		fmt.Println("DIALTONE> [TEST] Starting automated onboarding sequence...")
		testCmds := []string{
			"/mesh add wsl wsl.shad-artichoke.ts.net user",
			"/ssh wsl whoami",
			"exit",
		}
		for _, cmd := range testCmds {
			fmt.Printf("%s%s\n", prompt, cmd)
			if shouldExit(cmd) {
				return nil
			}
			if err := handleInput(cmd, configPath, dialtoneSh); err != nil {
				fmt.Printf("DIALTONE> [ERROR] %v\n", err)
			}
		}
		return nil
	}

	// --- 2. LLM Mode (Programmatic Control) ---
	if isLLM {
		foundCmds := false
		for _, arg := range args {
			// Skip flags and internal routing keywords
			if arg == "--llm" || arg == "repl" || arg == "src_v2" || arg == "run" {
				continue
			}

			foundCmds = true
			fmt.Printf("%s%s\n", prompt, arg)
			if shouldExit(arg) {
				return nil
			}
			if err := handleInput(arg, configPath, dialtoneSh); err != nil {
				fmt.Printf("DIALTONE> [ERROR] %v\n", err)
			}
		}
		if foundCmds {
			return nil
		}
		fmt.Println("DIALTONE> LLM Mode active. Listening for input...")
	}

	// --- 3. Interactive Loop ---
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print(prompt)
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if shouldExit(line) {
			break
		}

		if err := handleInput(line, configPath, dialtoneSh); err != nil {
			fmt.Printf("DIALTONE> [ERROR] %v\n", err)
		}
	}

	return nil
}

func shouldExit(line string) bool {
	clean := strings.TrimSpace(line)
	return clean == "exit" || clean == "quit" || clean == "/exit" || clean == "/quit"
}

func handleInput(line string, configPath string, dialtoneSh string) error {
	if !strings.HasPrefix(line, "/") {
		return executeCommand(line, dialtoneSh)
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
	case "/session":
		return handleSession(args, configPath)
	case "/exit", "/quit":
		return nil
	default:
		return fmt.Errorf("unknown slash command: %s", cmd)
	}
	return nil
}

func handleSession(args []string, path string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: /session <set|get|clear> [key] [value]")
	}
	config, _ := loadConfig(path)
	if config.Session == nil {
		config.Session = make(map[string]string)
	}

	switch args[0] {
	case "set":
		if len(args) < 3 {
			return fmt.Errorf("usage: /session set <key> <value>")
		}
		config.Session[args[1]] = args[2]
		fmt.Printf("DIALTONE> Session state updated: %s=%s\n", args[1], args[2])
	case "get":
		if len(args) < 2 {
			return fmt.Errorf("usage: /session get <key>")
		}
		fmt.Printf("DIALTONE> Session %s: %s\n", args[1], config.Session[args[1]])
		return nil // Don't save
	case "clear":
		config.Session = make(map[string]string)
		fmt.Println("DIALTONE> Session cleared.")
	default:
		return fmt.Errorf("unknown session action: %s", args[0])
	}
	return saveConfig(path, config)
}

func handleEnv(args []string, path string) error {
	if len(args) < 3 || args[0] != "set" {
		return fmt.Errorf("usage: /env set <key> <value>")
	}
	config, _ := loadConfig(path)
	key, val := args[1], args[2]
	switch key {
	case "DIALTONE_ENV":
		config.EnvRoot = val
	case "DIALTONE_REPO_ROOT":
		config.RepoRoot = val
	case "DIALTONE_USE_NIX":
		config.UseNix = val
	default:
		return fmt.Errorf("unknown env key: %s", key)
	}
	return saveConfig(path, config)
}

func handleMesh(args []string, path string) error {
	if len(args) < 4 || args[0] != "add" {
		return fmt.Errorf("usage: /mesh add <name> <host> <user>")
	}
	config, _ := loadConfig(path)
	name, host, user := args[1], args[2], args[3]
	for i, n := range config.MeshNodes {
		if n.Name == name {
			config.MeshNodes[i].Host = host
			config.MeshNodes[i].User = user
			fmt.Printf("DIALTONE> Updated mesh node: %s\n", name)
			return saveConfig(path, config)
		}
	}
	newNode := sshv1.MeshNode{
		Name: name, Host: host, User: user, Port: "22", OS: "linux",
		Aliases: []string{name}, HostCandidates: []string{host},
		RoutePreference: []string{"tailscale", "private"},
	}
	config.MeshNodes = append(config.MeshNodes, newNode)
	fmt.Printf("DIALTONE> Added mesh node: %s\n", name)
	return saveConfig(path, config)
}

func handleSSH(args []string, bin string, configPath string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: /ssh <node> <command>")
	}
	node := args[0]
	cmd := strings.Join(args[1:], " ")
	pluginCmd := fmt.Sprintf("ssh src_v1 run --host %s --cmd %q", node, cmd)
	os.Setenv("DIALTONE_MESH_CONFIG", configPath)
	return executeCommand(pluginCmd, bin)
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
	fmt.Println("  /env set <K> <V>       Set variable in dialtone.json")
	fmt.Println("  /mesh add <N> <H> <U>  Add/Update mesh node")
	fmt.Println("  /ssh <node> <cmd>      Run SSH command")
	fmt.Println("  /session <set|get>     Manage session state")
	fmt.Println("  /status                Show config status")
	fmt.Println("  /ps                    List subtones")
	fmt.Println("  exit                   Quit REPL")
}

func printStatus(path string) error {
	config, err := loadConfig(path)
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}
	fmt.Printf("DIALTONE> Configuration: %s\n", path)
	fmt.Printf("  Mesh Nodes (%d):\n", len(config.MeshNodes))
	for _, n := range config.MeshNodes {
		fmt.Printf("    - %s (%s@%s)\n", n.Name, n.User, n.Host)
	}
	if len(config.Session) > 0 {
		fmt.Printf("  Session State:\n")
		for k, v := range config.Session {
			fmt.Printf("    - %s: %s\n", k, v)
		}
	}
	return nil
}

func executeCommand(line string, bin string) error {
	if shouldExit(line) {
		return nil
	}
	args := strings.Fields(line)
	if len(args) == 0 {
		return nil
	}

	cmd := exec.Command(bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func printManagedProcesses() {
	procs := proc.ListManagedProcesses()
	if len(procs) == 0 {
		fmt.Println("DIALTONE> No active subtones.")
		return
	}
	fmt.Println("DIALTONE> Active Subtones:")
	fmt.Printf("%-8s %-10s %s\n", "PID", "CPU%", "COMMAND")
	for _, p := range procs {
		fmt.Printf("%-8d %-10.1f %s\n", p.PID, p.CPUPercent, p.Command)
	}
}

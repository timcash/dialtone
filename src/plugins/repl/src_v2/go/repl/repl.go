package repl

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dialtone/dev/plugins/proc/src_v1/go/proc"
)

func RunREPLV2(args []string) error {
	repoRoot := os.Getenv("DIALTONE_REPO_ROOT")
	if repoRoot == "" {
		return fmt.Errorf("DIALTONE_REPO_ROOT is not set")
	}

	isTest := false
	for _, arg := range args {
		if arg == "--test" {
			isTest = true
		}
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "USER-1"
	}
	prompt := hostname + "> "

	fmt.Println("DIALTONE> REPL v2 starting...")
	
	// --- 1. Guided Setup & Verification ---
	verifyFile := func(path, description string) bool {
		fullPath := filepath.Join(repoRoot, path)
		if _, err := os.Stat(fullPath); err == nil {
			fmt.Printf("DIALTONE> [OK] %s found: %s\n", description, path)
			return true
		}
		fmt.Printf("DIALTONE> [MISSING] %s not found: %s\n", description, path)
		return false
	}

	verifyFile("env/.env", ".env file")
	verifyFile("env/mesh.json", "mesh.json file")
	verifyFile("env/ssh_config", "ssh_config file")

	// --- 2. SSH connectivity check to wsl ---
	fmt.Println("DIALTONE> Verifying SSH connectivity to wsl...")
	sshCmd := exec.Command("ssh", "-F", filepath.Join(repoRoot, "env", "ssh_config"), "wsl", "whoami")
	sshCmd.Stderr = os.Stderr
	if out, err := sshCmd.Output(); err == nil {
		fmt.Printf("DIALTONE> [OK] SSH connection to wsl successful (user: %s)\n", strings.TrimSpace(string(out)))
	} else {
		fmt.Printf("DIALTONE> [FAIL] SSH connection to wsl failed: %v\n", err)
	}

	if isTest {
		fmt.Println("DIALTONE> Test mode active. Exiting after verification.")
		return nil
	}

	// --- 3. Print Available Commands ---
	fmt.Println("\nAvailable Dialtone commands (from REPL):")
	fmt.Println("  ssh src_v1 run --host <node> --cmd <command>")
	fmt.Println("  chrome v1 status")
	fmt.Println("  ps (List active processes)")
	fmt.Println("  help (Full help)")
	fmt.Println("  exit (Quit REPL)\n")

	// --- 4. Interactive Loop ---
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
		if line == "exit" || line == "quit" {
			break
		}
		if line == "help" {
			fmt.Println("Type any dialtone command directly, or 'exit' to quit.")
			continue
		}
		if line == "ps" {
			printManagedProcesses()
			continue
		}

		executeCommand(line)
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

func executeCommand(line string) {
	args := strings.Fields(line)
	// We call back into dialtone.sh for simplicity or run dev.go
	// But let's assume we can run any dialtone command.
	repoRoot := os.Getenv("DIALTONE_REPO_ROOT")
	dialtoneSh := filepath.Join(repoRoot, "dialtone.sh")

	// We'll pass the command through dialtone.sh to reuse its env setup
	cmd := exec.Command(dialtoneSh, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Printf("DIALTONE> [ERROR] command failed: %v\n", err)
	}
}

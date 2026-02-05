package cli

import (
	"dialtone/cli/src/core/config"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	swarm_test "dialtone/cli/src/plugins/swarm/test"
)

func RunSwarm(args []string) {
	if len(args) < 1 {
		printSwarmUsage()
		return
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "help", "-h", "--help":
		printSwarmUsage()
	case "install":
		runSwarmInstall(subArgs)
	case "build":
		runSwarmBuild(subArgs)
	case "test":
		runSwarmTest(subArgs)
	case "test-e2e":
		runSwarmE2E(subArgs)
	case "start":
		runSwarmStart(subArgs)
	case "stop":
		runSwarmStop(subArgs)
	case "list":
		runSwarmList(subArgs)
	case "status":
		runSwarmStatus(subArgs)
	case "dashboard":
		runSwarmDashboard(subArgs)
	default:
		// Assume the first argument is the topic if no other subcommand matches
		runSwarmPear(args)
	}
}

func runSwarmDashboard(args []string) {
	fmt.Println("[swarm] Opening dashboard...")
	appDir := filepath.Join("src", "plugins", "swarm", "app")
	pearBin := getPearBin()

	// Pear run . will now use dashboard.html as defined in package.json gui.main
	cmd := exec.Command(pearBin, "run", ".")
	cmd.Dir = appDir
	if err := cmd.Run(); err != nil {
		fmt.Printf("[swarm] Dashboard failed: %v\n", err)
	}
}

func runSwarmInstall(args []string) {
	fmt.Println("[swarm] Installing dependencies...")
	appDir := filepath.Join("src", "plugins", "swarm", "app")

	npmBin := "npm"
	envPath := config.GetDialtoneEnv()
	if envPath != "" {
		npmBin = filepath.Join(envPath, "node", "bin", "npm")
	}

	cmd := exec.Command(npmBin, "install")
	cmd.Dir = appDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("[swarm] npm install failed: %v\n", err)
		os.Exit(1)
	}
}

func runSwarmBuild(args []string) {
	fmt.Println("[swarm] Building plugin (no-op for now)...")
}

func runSwarmTest(args []string) {
	if err := swarm_test.RunMultiPeerConnection(); err != nil {
		fmt.Printf("[swarm] Test failed: %v\n", err)
		os.Exit(1)
	}
}

func runSwarmStart(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: ./dialtone.sh swarm start <topic>")
		return
	}
	topic := args[0]
	fmt.Printf("[swarm] Starting background node for topic: %s\n", topic)

	appDir := filepath.Join("src", "plugins", "swarm", "app")
	pearBin := getPearBin()

	cmd := exec.Command(pearBin, "run", ".", topic)
	cmd.Dir = appDir

	logPath := filepath.Join(appDir, "swarm.log")
	logFile, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		fmt.Printf("[swarm] Failed to start node: %v\n", err)
		return
	}

	node := SwarmNode{
		ID:        fmt.Sprintf("node-%d", cmd.Process.Pid),
		PID:       cmd.Process.Pid,
		Topic:     topic,
		StartTime: time.Now(),
		Status:    "running",
	}
	addNodeToRegistry(node)

	fmt.Printf("[swarm] Node started with PID %d. Logs: %s\n", cmd.Process.Pid, logPath)
}

func runSwarmStop(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: ./dialtone.sh swarm stop <pid>")
		return
	}
	pidStr := args[0]
	var pid int
	fmt.Sscanf(pidStr, "%d", &pid)

	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Printf("[swarm] Process %d not found\n", pid)
		return
	}

	fmt.Printf("[swarm] Stopping node with PID %d...\n", pid)
	if err := process.Kill(); err != nil {
		fmt.Printf("[swarm] Failed to kill process %d: %v\n", pid, err)
	}

	removeNodeFromRegistry(pid)
	fmt.Println("[swarm] Node stopped and removed from registry.")
}

func reconcileAndCheckNode(n *SwarmNode, realPids map[string]int) (int, string) {
	pid := n.PID
	if realPid, ok := realPids[n.Topic]; ok {
		pid = realPid
	}

	proc, _ := os.FindProcess(pid)
	err := proc.Signal(os.Signal(nil))
	status := "alive"
	if err != nil {
		status = "dead"
	}
	return pid, status
}

func getRealPids() map[string]int {
	home, _ := os.UserHomeDir()
	swarmDir := filepath.Join(home, ".dialtone", "swarm")
	files, _ := os.ReadDir(swarmDir)
	realPids := make(map[string]int)
	for _, f := range files {
		if len(f.Name()) > 7 && f.Name()[:7] == "status_" {
			data, _ := os.ReadFile(filepath.Join(swarmDir, f.Name()))
			var status struct {
				PID   int    `json:"pid"`
				Topic string `json:"topic"`
			}
			if json.Unmarshal(data, &status) == nil {
				realPids[status.Topic] = status.PID
			}
		}
	}
	return realPids
}

func runSwarmList(args []string) {
	registry, err := loadRegistry()
	if err != nil {
		fmt.Printf("[swarm] Failed to load registry: %v\n", err)
		return
	}

	realPids := getRealPids()
	if len(registry.Nodes) == 0 && len(realPids) == 0 {
		fmt.Println("[swarm] No running nodes found.")
		return
	}

	fmt.Printf("%-10s %-10s %-20s %-20s\n", "ID/PID", "STATUS", "TOPIC", "STARTED")
	fmt.Println("----------------------------------------------------------------------")

	updated := false
	for i, n := range registry.Nodes {
		pid, status := reconcileAndCheckNode(&n, realPids)
		if pid != n.PID {
			registry.Nodes[i].PID = pid
			updated = true
		}
		fmt.Printf("%-10d %-10s %-20s %-20s\n", pid, status, n.Topic, n.StartTime.Format("15:04:05"))
	}
	if updated {
		saveRegistry(registry)
	}
}

func runSwarmStatus(args []string) {
	registry, err := loadRegistry()
	if err != nil {
		fmt.Printf("[swarm] Failed to load registry: %v\n", err)
		return
	}

	realPids := getRealPids()
	if len(registry.Nodes) == 0 {
		fmt.Println("[swarm] No running nodes found.")
		return
	}

	fmt.Printf("%-10s %-10s %-20s %-10s %-10s %-20s\n", "ID/PID", "STATUS", "TOPIC", "PEERS", "LATENCY", "STARTED")
	fmt.Println("----------------------------------------------------------------------------------------------------")
	for i, n := range registry.Nodes {
		pid, status := reconcileAndCheckNode(&n, realPids)
		if pid != n.PID {
			registry.Nodes[i].PID = pid
		}

		home, _ := os.UserHomeDir()
		statusFile := filepath.Join(home, ".dialtone", "swarm", fmt.Sprintf("status_%d.json", pid))

		var peers int
		var latencyStr string = "N/A"

		if data, err := os.ReadFile(statusFile); err == nil {
			var statusData struct {
				Peers     int                `json:"peers"`
				Latencies map[string]float64 `json:"latencies"`
			}
			if json.Unmarshal(data, &statusData) == nil {
				peers = statusData.Peers
				if len(statusData.Latencies) > 0 {
					var sum float64
					for _, l := range statusData.Latencies {
						sum += l
					}
					latencyStr = fmt.Sprintf("%.2fms", sum/float64(len(statusData.Latencies)))
				}
			}
		}

		fmt.Printf("%-10d %-10s %-20s %-10d %-10s %-20s\n", pid, status, n.Topic, peers, latencyStr, n.StartTime.Format("15:04:05"))
	}
}

func getPearBin() string {
	pearBin := "pear"
	envPath := config.GetDialtoneEnv()
	if envPath != "" {
		testPear := filepath.Join(envPath, "node", "bin", "pear")
		if _, err := os.Stat(testPear); err == nil {
			pearBin = testPear
		}
	}
	return pearBin
}

func printSwarmUsage() {
	fmt.Println("Usage: ./dialtone.sh swarm <subcommand> [args]")
	fmt.Println("\nSubcommands:")
	fmt.Println("  start <topic>    Start a background node for a topic")
	fmt.Println("  stop <pid>       Stop a background node by PID")
	fmt.Println("  list             List all running swarm nodes")
	fmt.Println("  status           Show detailed status/top-like report")
	fmt.Println("  install          Install swarm dependencies")
	fmt.Println("  test             Run integration tests")
	fmt.Println("  test-e2e         Run consolidated E2E tests with Puppeteer")
	fmt.Println("\nConnects to a Hyperswarm topic using Pear runtime.")
}

func runSwarmE2E(args []string) {
	fmt.Println("[swarm] Running Node + Puppeteer Orchestrated E2E tests...")
	testFile := filepath.Join("src", "plugins", "swarm", "test", "swarm_orchestrator.ts")

	cmd := exec.Command("npx", "tsx", testFile)
	cmd.Dir = "." // Run from root to let dialtone.sh work
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("[swarm] E2E test failed: %v\n", err)
	}
	fmt.Println("[swarm] E2E tests complete.")
}

func runSwarmPear(args []string) {
	topic := args[0]
	fmt.Printf("[swarm] Joining topic: %s\n", topic)

	appDir := filepath.Join("src", "plugins", "swarm", "app")
	pearBin := getPearBin()

	cmd := exec.Command(pearBin, "run", ".", topic)
	cmd.Dir = appDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("[swarm] Pear execution failed: %v\n", err)
		os.Exit(1)
	}
}

package cli

import (
	"bufio"
	"dialtone/cli/src/core/config"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
	case "src":
		runSwarmSrc(subArgs)
	case "install":
		runSwarmInstall(subArgs)
	case "build":
		runSwarmBuild(subArgs)
	case "test":
		runSwarmTest(subArgs)
	case "start":
		runSwarmStart(subArgs)
	case "dev":
		runSwarmDev(subArgs)
	case "stop":
		runSwarmStop(subArgs)
	case "list":
		runSwarmList(subArgs)
	case "status":
		runSwarmStatus(subArgs)
	case "smoke":
		runSwarmSmoke(subArgs)
	case "dashboard":
		runSwarmDashboard(subArgs)
	case "lint":
		runSwarmLint(subArgs)
	case "warm":
		runSwarmWarm(subArgs)
	case "flush":
		runSwarmFlush(subArgs)
	default:
		// Assume the first argument is the topic if no other subcommand matches
		runSwarmPear(args)
	}
}

func runSwarmSmoke(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: ./dialtone.sh swarm smoke <dir>")
		return
	}
	dir := args[0]
	if err := swarm_test.RunSmoke(dir); err != nil {
		fmt.Printf("[swarm] Smoke test failed: %v\n", err)
		os.Exit(1)
	}
}

func runSwarmDashboard(args []string) {
	fmt.Println("[swarm] Starting dashboard HTTP server...")
	appDir := filepath.Join("src", "plugins", "swarm", "app")
	pearBin := getPearBin()

	// Run the app in dashboard mode so it serves http://127.0.0.1:4000
	cmd := exec.Command(pearBin, "run", ".", "dashboard", getRepoRoot())
	cmd.Dir = appDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "DIALTONE_REPO="+getRepoRoot())
	if err := cmd.Run(); err != nil {
		fmt.Printf("[swarm] Dashboard failed: %v\n", err)
	}
}

func getRepoRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func runSwarmInstall(args []string) {
	fmt.Println("[swarm] Installing dependencies into DIALTONE_ENV...")
	appDir := filepath.Join("src", "plugins", "swarm", "app")

	envPath := getDialtoneEnvOrExit()
	envAppDir := filepath.Join(envPath, "plugins", "swarm", "app")

	ensureDir(envAppDir)

	copyFile(filepath.Join(appDir, "package.json"), filepath.Join(envAppDir, "package.json"))
	copyFileIfExists(filepath.Join(appDir, "package-lock.json"), filepath.Join(envAppDir, "package-lock.json"))

	npmBin := resolveNpmBin(envPath)
	cmd := exec.Command(npmBin, "install")
	cmd.Dir = envAppDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("[swarm] npm install failed: %v\n", err)
		os.Exit(1)
	}

	linkNodeModules(appDir, filepath.Join(envAppDir, "node_modules"))
	ensureEnvToolLink(envPath, "pear")
}

func runSwarmWarm(args []string) {
	topicPrefix := "dialtone-test"
	if len(args) > 0 {
		topicPrefix = args[0]
	}
	fmt.Printf("[swarm] Starting persistent warm peer for topic prefix: %s\n", topicPrefix)
	fmt.Println("[swarm] This node holds open both Data and KeySwarm topics to accelerate discovery.")

	appDir := filepath.Join("src", "plugins", "swarm", "app")
	pearBin := getPearBin()

	cmd := exec.Command(pearBin, "run", "warm.js", topicPrefix)
	cmd.Dir = appDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("[swarm] Warm peer failed: %v\n", err)
	}
}

func runSwarmFlush(args []string) {
	fmt.Println("[swarm] Flushing all swarm topics and stopping all nodes...")

	// 1. Kill all nodes in registry
	registry, err := loadRegistry()
	if err == nil {
		for _, n := range registry.Nodes {
			fmt.Printf("[swarm] Killing node %d (%s)...\n", n.PID, n.Topic)
			proc, _ := os.FindProcess(n.PID)
			if proc != nil {
				_ = proc.Kill()
			}
			removeStatusFile(n.PID)
		}
	}

	// 2. Clear registry
	_ = saveRegistry(&NodeRegistry{Nodes: []SwarmNode{}})

	// 3. Wipe warm storage
	home, _ := os.UserHomeDir()
	warmDir := filepath.Join(home, ".dialtone", "swarm", "warm")
	fmt.Printf("[swarm] Wiping warm storage: %s\n", warmDir)
	_ = os.RemoveAll(warmDir)

	fmt.Println("[swarm] Flush complete. System is clean.")
}

func runSwarmBuild(args []string) {
	fmt.Println("[swarm] Building plugin (no-op for now)...")
}

func runSwarmTest(args []string) {
	if err := swarm_test.RunAll(); err != nil {
		fmt.Printf("[swarm] Test failed: %v\n", err)
		os.Exit(1)
	}
}

func runSwarmLint(args []string) {
	fmt.Println("[swarm] Running multi-linter...")

	// 1. Go Linting
	fmt.Println(">> [1/3] Linting Golang code...")
	gofmtCmd := exec.Command("gofmt", "-l", "src/plugins/swarm")
	out, _ := gofmtCmd.Output()
	if len(out) > 0 {
		fmt.Printf("[swarm] Go formatting issues in:\n%s", string(out))
		fmt.Println("[swarm] Run 'gofmt -w src/plugins/swarm' to fix.")
	} else {
		fmt.Println("   [PASS] Go formatting")
	}

	goVetCmd := exec.Command("go", "vet", "./src/plugins/swarm/cli", "./src/plugins/swarm/test")
	goVetCmd.Stdout = os.Stdout
	goVetCmd.Stderr = os.Stderr
	if err := goVetCmd.Run(); err != nil {
		fmt.Printf("[swarm] Go vet failed: %v\n", err)
	} else {
		fmt.Println("   [PASS] Go vet")
	}

	// 2. Prettier Linting (src_vN)
	fmt.Println(">> [2/3] Linting src_vN (Prettier)...")
	srcV2Dir := filepath.Join("src", "plugins", "swarm", "src_v2")
	envPath := getDialtoneEnvOrExit()
	npmBin := resolveNpmBin(envPath)

	// Try to run npx prettier if available
	prettierCmd := exec.Command(npmBin, "exec", "prettier", "--check", ".")
	prettierCmd.Dir = srcV2Dir
	// prettierCmd.Stdout = os.Stdout // Too noisy
	prettierCmd.Stderr = os.Stderr
	if err := prettierCmd.Run(); err != nil {
		fmt.Printf("[swarm] Prettier check failed in src_v2: %v\n", err)
	} else {
		fmt.Println("   [PASS] Prettier (src_v2)")
	}

	// 3. UI Linting (Bun)
	fmt.Println(">> [3/3] Linting UI (Bun)...")
	uiDir := filepath.Join(srcV2Dir, "ui")
	bunBin := resolveBunBin(envPath)

	bunLintCmd := exec.Command(bunBin, "run", "lint")
	bunLintCmd.Dir = uiDir
	bunLintCmd.Stdout = os.Stdout
	bunLintCmd.Stderr = os.Stderr
	if err := bunLintCmd.Run(); err != nil {
		fmt.Printf("[swarm] Bun UI lint failed: %v\n", err)
	} else {
		fmt.Println("   [PASS] Bun UI lint")
	}

	fmt.Println("[swarm] Multi-lint complete.")
}

func runSwarmStart(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: ./dialtone.sh swarm start <topic> [name]")
		return
	}
	topic := args[0]
	name := ""
	if len(args) > 1 {
		name = args[1]
	}
	fmt.Printf("[swarm] Starting background node for topic: %s\n", topic)

	appDir := filepath.Join("src", "plugins", "swarm", "app")
	pearBin := getPearBin()

	cmdArgs := []string{"run", ".", topic}
	if name != "" {
		cmdArgs = append(cmdArgs, name)
	}
	cmd := exec.Command(pearBin, cmdArgs...)
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

func runSwarmDev(args []string) {
	mode := "dashboard"
	name := ""
	if len(args) > 0 && args[0] != "" {
		mode = args[0]
	}
	if len(args) > 1 {
		name = args[1]
	}

	fmt.Printf("[swarm] Starting dev mode (%s)...\n", mode)
	appDir := filepath.Join("src", "plugins", "swarm", "app")
	pearBin := getPearBin()

	cmdArgs := []string{"run", "--dev", "--devtools", ".", mode}
	if mode != "dashboard" && name != "" {
		cmdArgs = append(cmdArgs, name)
	}
	cmd := exec.Command(pearBin, cmdArgs...)
	cmd.Dir = appDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		fmt.Printf("[swarm] Dev mode failed: %v\n", err)
	}
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
	removeStatusFile(pid)
	fmt.Println("[swarm] Node stopped and removed from registry.")
}

func removeStatusFile(pid int) {
	home, _ := os.UserHomeDir()
	statusFile := filepath.Join(home, ".dialtone", "swarm", fmt.Sprintf("status_%d.json", pid))
	_ = os.Remove(statusFile)
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
	envPath := getDialtoneEnvOrExit()
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".cmd"
	}

	candidates := append(pearRuntimeCandidates(), []string{
		filepath.Join(envPath, "node", "bin", "pear"+ext),
		filepath.Join(envPath, "bin", "pear"+ext),
	}...)
	if pearPath := firstExistingPath(candidates); pearPath != "" {
		return pearPath
	}

	if promptInstallPear(envPath) {
		if pearPath := firstExistingPath(candidates); pearPath != "" {
			return pearPath
		}
	}

	fmt.Printf("[swarm] pear not found in DIALTONE_ENV (%s).\n", envPath)
	fmt.Println("[swarm] Please install Pear and re-run (example: add pear to PATH, then run ./dialtone.sh swarm install).")
	os.Exit(1)
	return "pear"
}

func pearRuntimeCandidates() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	pearDir := ""
	switch runtime.GOOS {
	case "darwin":
		pearDir = filepath.Join(homeDir, "Library", "Application Support", "pear")
	case "linux":
		pearDir = filepath.Join(homeDir, ".config", "pear")
	case "windows":
		pearDir = filepath.Join(homeDir, "AppData", "Roaming", "pear")
	default:
		return nil
	}
	runtimeExt := ""
	if runtime.GOOS == "windows" {
		runtimeExt = ".exe"
	}
	host := runtime.GOOS + "-" + runtime.GOARCH
	return []string{
		filepath.Join(pearDir, "bin", "pear"+runtimeExt),
		filepath.Join(pearDir, "current", "by-arch", host, "bin", "pear-runtime"+runtimeExt),
	}
}

func printSwarmUsage() {
	fmt.Println("Usage: ./dialtone.sh swarm <subcommand> [args]")
	fmt.Println("\nSubcommands:")
	fmt.Println("  start <topic> [name]   Start a background node for a topic")
	fmt.Println("  dev [topic|dashboard] [name]  Run Pear dev mode with devtools")
	fmt.Println("  stop <pid>       Stop a background node by PID")
	fmt.Println("  list             List all running swarm nodes")
	fmt.Println("  status           Show detailed status/top-like report")
	fmt.Println("  install          Install swarm dependencies")
	fmt.Println("  test             Run integration tests (chromedp + dashboard)")
	fmt.Println("  warm [prefix]    Start a warm peer to speed up test discovery")
	fmt.Println("  src --n N        Create or validate a srcN template folder")
	fmt.Println("  smoke <dir>      Run smoke tests for a specific directory")
	fmt.Println("\nConnects to a Hyperswarm topic using Pear runtime.")
}

func getDialtoneEnvOrExit() string {
	envPath := config.GetDialtoneEnv()
	if envPath == "" {
		fmt.Println("[swarm] DIALTONE_ENV is not set. Please set it in env/.env or pass --env.")
		os.Exit(1)
	}
	return envPath
}

func resolveNpmBin(envPath string) string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".cmd"
	}
	localNpm := filepath.Join(envPath, "node", "bin", "npm"+ext)
	if _, err := os.Stat(localNpm); err == nil {
		return localNpm
	}
	if path, err := exec.LookPath("npm"); err == nil {
		fmt.Printf("[swarm] WARNING: npm not found in DIALTONE_ENV, using system npm at %s\n", path)
		return path
	}
	fmt.Printf("[swarm] npm not found in DIALTONE_ENV (%s). Run ./dialtone.sh install first.\n", envPath)
	os.Exit(1)
	return "npm"
}

func resolveBunBin(envPath string) string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	candidates := []string{
		filepath.Join(envPath, "bin", "bun"+ext),
		filepath.Join(envPath, "bun", "bin", "bun"+ext),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	if path, err := exec.LookPath("bun"); err == nil {
		fmt.Printf("[swarm] WARNING: bun not found in DIALTONE_ENV, using system bun at %s\n", path)
		return path
	}
	fmt.Printf("[swarm] bun not found in DIALTONE_ENV (%s). Install bun to run swarm tests.\n", envPath)
	os.Exit(1)
	return "bun"
}

func ensureEnvToolLink(envPath, tool string) {
	envBin := filepath.Join(envPath, "bin", tool)
	if _, err := os.Stat(envBin); err == nil {
		return
	}
	toolPath, err := exec.LookPath(tool)
	if err != nil {
		fmt.Printf("[swarm] %s not found in PATH to link into DIALTONE_ENV.\n", tool)
		return
	}
	ensureDir(filepath.Join(envPath, "bin"))
	_ = os.Remove(envBin)
	if err := os.Symlink(toolPath, envBin); err != nil {
		fmt.Printf("[swarm] WARNING: Failed to symlink %s into DIALTONE_ENV: %v\n", tool, err)
	} else {
		fmt.Printf("[swarm] Linked %s into DIALTONE_ENV at %s\n", tool, envBin)
	}
}

func ensureDir(path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		fmt.Printf("[swarm] Failed to create directory %s: %v\n", path, err)
		os.Exit(1)
	}
}

func copyFile(src, dst string) {
	data, err := os.ReadFile(src)
	if err != nil {
		fmt.Printf("[swarm] Failed to read %s: %v\n", src, err)
		os.Exit(1)
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		fmt.Printf("[swarm] Failed to write %s: %v\n", dst, err)
		os.Exit(1)
	}
}

func copyFileIfExists(src, dst string) {
	if _, err := os.Stat(src); err != nil {
		return
	}
	copyFile(src, dst)
}

func linkNodeModules(appDir, envNodeModules string) {
	target := filepath.Join(appDir, "node_modules")
	if _, err := os.Stat(envNodeModules); err != nil {
		fmt.Printf("[swarm] Expected node_modules at %s; install may have failed.\n", envNodeModules)
		return
	}
	if info, err := os.Lstat(target); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			if link, err := os.Readlink(target); err == nil && link == envNodeModules {
				return
			}
		}
		_ = os.RemoveAll(target)
	}
	if err := os.Symlink(envNodeModules, target); err != nil {
		fmt.Printf("[swarm] WARNING: Failed to link node_modules into app dir: %v\n", err)
	}
}

func firstExistingPath(candidates []string) string {
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func promptInstallPear(envPath string) bool {
	fmt.Println("[swarm] Pear not found in DIALTONE_ENV.")
	fmt.Printf("[swarm] Install Pear and ensure it is linked at %s/bin/pear.\n", envPath)
	fmt.Println("[swarm] Example: install Pear on your system, then run ./dialtone.sh swarm install.")
	fmt.Print("[swarm] Press 'y' then Enter to retry after installing, or any other key to abort: ")

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

func runSwarmPear(args []string) {
	topic := args[0]
	name := ""
	if len(args) > 1 {
		name = args[1]
	}
	fmt.Printf("[swarm] Joining topic: %s\n", topic)

	appDir := filepath.Join("src", "plugins", "swarm", "app")
	pearBin := getPearBin()

	cmdArgs := []string{"run", ".", topic}
	if name != "" {
		cmdArgs = append(cmdArgs, name)
	}
	cmd := exec.Command(pearBin, cmdArgs...)
	cmd.Dir = appDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("[swarm] Pear execution failed: %v\n", err)
		os.Exit(1)
	}
}

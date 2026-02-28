package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
	"dialtone/dev/plugins/logs/src_v1/go"
)

// RunChrome handles the 'chrome' command
func RunChrome(args []string) {
	if len(args) == 0 {
		printChromeUsage()
		return
	}

	normalized, warnedOldOrder, err := normalizeChromeArgs(args)
	if err != nil {
		logs.Error("%v", err)
		printChromeUsage()
		return
	}
	if warnedOldOrder {
		logs.Warn("old chrome CLI order is deprecated. Use: ./dialtone.sh chrome src_v1 <command> [args]")
	}
	args = normalized

	switch args[0] {
	case "help", "--help", "-h":
		printChromeUsage()
	case "list":
		listFlags := flag.NewFlagSet("chrome list", flag.ExitOnError)
		headed := listFlags.Bool("headed", false, "Show only headed processes")
		headless := listFlags.Bool("headless", false, "Show only headless processes")
		verbose := listFlags.Bool("verbose", false, "Show full command line report")
		listFlags.BoolVar(verbose, "v", false, "Alias for --verbose")

		for _, arg := range args[1:] {
			if arg == "--help" || arg == "-h" {
				fmt.Println("Usage: ./dialtone.sh chrome src_v1 list [flags]")
				fmt.Println("\nLists all detected Chrome/Edge processes on the system.")
				fmt.Println("\nFlags:")
				listFlags.PrintDefaults()
				return
			}
		}

		listFlags.Parse(args[1:])
		handleList(*headed, *headless, *verbose)
	case "kill":
		killFlags := flag.NewFlagSet("chrome kill", flag.ExitOnError)
		isWindows := killFlags.Bool("windows", false, "Use for WSL 2 host processes")
		totalAll := killFlags.Bool("all", false, "Kill ALL browser processes system-wide")

		for _, arg := range args[1:] {
			if arg == "--help" || arg == "-h" {
				fmt.Println("Usage: ./dialtone.sh chrome src_v1 kill [PID|all] [flags]")
				fmt.Println("\nTerminates Chrome processes. Default behavior is to only kill Dialtone-originated instances.")
				fmt.Println("\nFlags:")
				killFlags.PrintDefaults()
				fmt.Println("\nExamples:")
				fmt.Println("  ./dialtone.sh chrome src_v1 kill all        # Kill only Dialtone-started browsers")
				fmt.Println("  ./dialtone.sh chrome src_v1 kill all --all  # Kill EVERY Chrome process on the PC")
				fmt.Println("  ./dialtone.sh chrome src_v1 kill 1234       # Kill specific process by PID")
				return
			}
		}

		arg := "all"
		if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
			arg = args[1]
			killFlags.Parse(args[2:])
		} else {
			killFlags.Parse(args[1:])
		}
		handleKill(arg, *isWindows, *totalAll)
	case "new":
		newFlags := flag.NewFlagSet("chrome new", flag.ExitOnError)
		port := newFlags.Int("port", 0, "Remote debugging port (0 for auto)")
		gpu := newFlags.Bool("gpu", false, "Enable GPU acceleration")
		headless := newFlags.Bool("headless", false, "Launch in headless mode")
		role := newFlags.String("role", "", "Dialtone role tag (e.g. dev, smoke)")
		reuseExisting := newFlags.Bool("reuse-existing", false, "Attach to existing matching role/headless instance")
		userDataDir := newFlags.String("user-data-dir", "", "Explicit Chrome user data dir")
		debugAddress := newFlags.String("debug-address", "127.0.0.1", "Remote debug bind address")
		debug := newFlags.Bool("debug", false, "Enable verbose logging")

		for _, arg := range args[1:] {
			if arg == "--help" || arg == "-h" {
				fmt.Println("Usage: ./dialtone.sh chrome src_v1 new [URL] [flags]")
				fmt.Println("\nLaunches a new headed Chrome instance linked to Dialtone.")
				fmt.Println("\nFlags:")
				newFlags.PrintDefaults()
				return
			}
		}

		if err := newFlags.Parse(args[1:]); err != nil {
			logs.Fatal("new parse failed: %v", err)
		}

		targetURL := ""
		if rest := newFlags.Args(); len(rest) > 0 {
			targetURL = rest[0]
		}
		handleNew(*port, *gpu, *headless, targetURL, *role, *reuseExisting, *userDataDir, *debugAddress, *debug)
	case "session":
		sessionFlags := flag.NewFlagSet("chrome session", flag.ExitOnError)
		port := sessionFlags.Int("port", 0, "Remote debugging port (0 for auto)")
		gpu := sessionFlags.Bool("gpu", false, "Enable GPU acceleration")
		headless := sessionFlags.Bool("headless", true, "Launch in headless mode")
		role := sessionFlags.String("role", "", "Dialtone role tag (e.g. dev, smoke)")
		reuseExisting := sessionFlags.Bool("reuse-existing", true, "Attach to existing matching role/headless instance")
		userDataDir := sessionFlags.String("user-data-dir", "", "Explicit Chrome user data dir")
		debugAddress := sessionFlags.String("debug-address", "127.0.0.1", "Remote debug bind address")
		url := sessionFlags.String("url", "about:blank", "Initial URL")
		if err := sessionFlags.Parse(args[1:]); err != nil {
			logs.Fatal("session parse failed: %v", err)
		}
		handleSession(*port, *gpu, *headless, *url, *role, *reuseExisting, *userDataDir, *debugAddress)
	case "test":
		if err := runChromeTests(args[1:]); err != nil {
			logs.Fatal("Chrome self-test failed: %v", err)
		}
		logs.Info("Chrome self-test passed")
	case "verify":
		verifyFlags := flag.NewFlagSet("chrome verify", flag.ExitOnError)
		port := verifyFlags.Int("port", chrome.DefaultDebugPort, "Remote debugging port")
		debug := verifyFlags.Bool("debug", false, "Enable verbose logging")

		for _, arg := range args[1:] {
			if arg == "--help" || arg == "-h" {
				fmt.Println("Usage: ./dialtone.sh chrome src_v1 verify [flags]")
				fmt.Println("\nChecks if application is reachable via remote debugging port.")
				fmt.Println("\nFlags:")
				verifyFlags.PrintDefaults()
				return
			}
		}

		verifyFlags.Parse(args[1:])
		verifyChrome(*port, *debug)
	case "install":
		logs.Info("Chrome plugin: No specific dependencies to install (detects local Chrome).")
	case "remote-list":
		handleRemoteListCmd(args[1:])
	case "remote-new":
		handleRemoteNewCmd(args[1:])
	case "remote-probe":
		handleRemoteProbeCmd(args[1:])
	case "remote-relay":
		handleRemoteRelayCmd(args[1:])
	case "remote-doctor":
		handleRemoteDoctorCmd(args[1:])
	case "remote-kill":
		handleRemoteKillCmd(args[1:])
	case "remote-wsl-forward":
		handleRemoteWSLForwardCmd(args[1:])
	default:
		printChromeUsage()
	}
}

func verifyChrome(port int, debug bool) {
	logs.Info("Verifying Chrome/Chromium connectivity (Target Port: %d)...", port)
	if err := chrome.VerifyChrome(port, debug); err != nil {
		logs.Fatal("Chrome verification FAILED: %v", err)
	}
	logs.Info("Chrome verification SUCCESS")
}

func handleList(headedOnly, headlessOnly, verbose bool) {
	logs.Info("Scanning for Chrome/Chromium resources...")
	// We always get all and filter/categorize here
	procs, err := chrome.ListResources(true)
	if err != nil {
		logs.Fatal("Failed to list resources: %v", err)
	}

	if len(procs) == 0 {
		logs.Info("No Chrome processes detected.")
		return
	}

	if verbose {
		fmt.Printf("\n%-8s %-8s %-8s %-10s %-10s %-6s %-10s %-8s %-10s %-8s %-5s %s\n", "PID", "PPID", "HEADLESS", "ORIGIN", "ROLE", "%CPU", "MEM(MB)", "CHILDREN", "PLATFORM", "PORT", "GPU", "COMMAND")
		fmt.Println(strings.Repeat("-", 180))
	} else {
		fmt.Printf("\n%-8s %-8s %-8s %-10s %-10s %-10s %-6s %-10s %-8s %-5s %-10s\n", "PID", "PPID", "HEADLESS", "ORIGIN", "ROLE", "TYPE", "%CPU", "MEM(MB)", "PORT", "GPU", "PLATFORM")
		fmt.Println(strings.Repeat("-", 105))
	}

	count := 0
	for _, p := range procs {
		if headedOnly && p.IsHeadless {
			continue
		}
		if headlessOnly && !p.IsHeadless {
			continue
		}

		platform := "Native"
		if p.IsWindows {
			platform = "Windows"
		}

		headless := "NO"
		if p.IsHeadless {
			headless = "YES"
		}

		gpuStatus := "YES"
		if !p.GPUEnabled {
			gpuStatus = "NO"
		}

		portStr := "-"
		if p.DebugPort > 0 {
			portStr = fmt.Sprintf("%d", p.DebugPort)
		}

		// Determine process type
		procType := "Browser"
		if strings.Contains(p.Command, "--type=renderer") {
			procType = "Renderer"
		} else if strings.Contains(p.Command, "--type=gpu-process") {
			procType = "GPU"
		} else if strings.Contains(p.Command, "--type=utility") {
			procType = "Utility"
		} else if strings.Contains(p.Command, "--type=crashpad-handler") {
			procType = "Crashpad"
		}

		cpuStr := fmt.Sprintf("%.1f", p.CPUPerc)
		if p.CPUPerc == 0 {
			cpuStr = "N/A"
		}

		if verbose {
			cmd := p.Command
			// Clean up common long paths for readability while keeping flags
			cmd = strings.TrimPrefix(cmd, "/mnt/c/Program Files/Google/Chrome/Application/")
			cmd = strings.TrimPrefix(cmd, "/mnt/c/Program Files (x86)/Google/Chrome/Application/")
			cmd = strings.TrimPrefix(cmd, "C:\\Program Files\\Google\\Chrome\\Application\\")

			fmt.Printf("%-8d %-8d %-8s %-10s %-10s %-6s %-10.1f %-8d %-10s %-8s %-5s %s\n",
				p.PID, p.PPID, headless, p.Origin, p.Role, cpuStr, p.MemoryMB, p.ChildCount, platform, portStr, gpuStatus, cmd)
		} else {
			fmt.Printf("%-8d %-8d %-8s %-10s %-10s %-10s %-6s %-10.1f %-8s %-5s %-10s\n",
				p.PID, p.PPID, headless, p.Origin, p.Role, procType, cpuStr, p.MemoryMB, portStr, gpuStatus, platform)
		}
		count++
	}
	fmt.Println()
	logs.Info("Total: %d processes (displayed: %d)", len(procs), count)
	if !verbose {
		logs.Info("Use --verbose to see the full command line report.")
	}
}

func handleKill(arg string, isWindows, totalAll bool) {
	if arg == "all" {
		if totalAll {
			logs.Info("Killing ALL Chrome processes system-wide...")
			if err := chrome.KillAllResources(); err != nil {
				logs.Fatal("Failed to kill all resources: %v", err)
			}
			logs.Info("Successfully killed all Chrome processes")
		} else {
			logs.Info("Killing all Dialtone Chrome instances...")
			if err := chrome.KillDialtoneResources(); err != nil {
				logs.Fatal("Failed to kill Dialtone resources: %v", err)
			}
			logs.Info("Successfully killed Dialtone Chrome processes")
		}
		return
	}

	var pid int
	fmt.Sscanf(arg, "%d", &pid)
	if pid == 0 {
		logs.Fatal("Invalid PID: %s", arg)
	}

	logs.Info("Killing Chrome process PID %d...", pid)

	// Auto-detect isWindows if not provided
	if !isWindows {
		procs, _ := chrome.ListResources(true)
		for _, p := range procs {
			if p.PID == pid {
				isWindows = p.IsWindows
				break
			}
		}
	}

	if err := chrome.KillResource(pid, isWindows); err != nil {
		logs.Fatal("Failed to kill resource: %v", err)
	}
	logs.Info("Successfully killed process %d", pid)
}

func handleNew(port int, gpu bool, headless bool, targetURL, role string, reuseExisting bool, userDataDir string, debugAddress string, debug bool) {
	logs.Info("Launching new %s Chrome instance...", func() string {
		if headless {
			return "headless"
		}
		return "headed"
	}())
	// If port is the default debug port, find a free one to avoid conflicts.
	if port == chrome.DefaultDebugPort {
		port = 0 // app.LaunchChrome will find one
	}

	res, err := chrome.StartSession(chrome.SessionOptions{
		RequestedPort: port,
		GPU:           gpu,
		Headless:      headless,
		TargetURL:     targetURL,
		Role:          role,
		ReuseExisting: reuseExisting,
		UserDataDir:   userDataDir,
		DebugAddress:  debugAddress,
	})
	if err != nil {
		logs.Fatal("Failed to launch Chrome: %v", err)
	}

	fmt.Println("\n🚀 Chrome started successfully!")
	fmt.Printf("%-15s: %d\n", "PID", res.PID)
	fmt.Printf("%-15s: %d\n", "Debug Port", res.Port)
	fmt.Printf("%-15s: %s\n", "WebSocket URL", res.WebSocketURL)
	fmt.Printf("%-15s: %t\n", "Reused", !res.IsNew)
	fmt.Println()
	logs.Info("You can now connect to this instance using the WebSocket URL.")
}

func handleSession(port int, gpu bool, headless bool, targetURL, role string, reuseExisting bool, userDataDir string, debugAddress string) {
	res, err := chrome.StartSession(chrome.SessionOptions{
		RequestedPort: port,
		GPU:           gpu,
		Headless:      headless,
		TargetURL:     targetURL,
		Role:          role,
		ReuseExisting: reuseExisting,
		UserDataDir:   userDataDir,
		DebugAddress:  debugAddress,
	})
	if err != nil {
		logs.Fatal("Failed to create session: %v", err)
	}
	meta := chrome.BuildSessionMetadata(res)
	raw, err := json.Marshal(meta)
	if err != nil {
		logs.Fatal("Failed to encode session metadata: %v", err)
	}
	fmt.Printf("DIALTONE_CHROME_SESSION_JSON=%s\n", string(raw))
}

func printChromeUsage() {
	fmt.Println("Usage: ./dialtone.sh chrome src_v1 <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  verify [--port N]   Verify chrome connectivity")
	fmt.Println("  list [flags]        List detected chrome processes")
	fmt.Println("  new [URL] [flags]   Launch a new Chrome instance")
	fmt.Println("  session [flags]     Launch/reuse and emit machine-readable session metadata")
	fmt.Println("  test                Run chrome plugin self-test (dev/test roles)")
	fmt.Println("  kill [PID|all] [--all] Kill Dialtone processes (default) or all processes")
	fmt.Println("  remote-list [flags] List Chrome processes across mesh nodes")
	fmt.Println("  remote-new [flags]  Start or reuse Chrome on a mesh node with role tag")
	fmt.Println("  remote-probe [flags] Probe debug ports/listeners across mesh nodes")
	fmt.Println("  remote-relay [flags] Start remote TCP relay for debug port exposure")
	fmt.Println("  remote-doctor [flags] Diagnose remote debug reachability/listener issues")
	fmt.Println("  remote-kill [flags] Kill remote Chrome processes by role/origin")
	fmt.Println("  remote-wsl-forward [flags] Configure Windows WSL devtools portproxy/firewall")
	fmt.Println("  install             Install chrome dependencies")
	fmt.Println("\nFlags for list:")
	fmt.Println("  --headed            Filter for headed instances only")
	fmt.Println("  --headless          Filter for headless instances only")
	fmt.Println("  --verbose, -v       Show full command line report")
	fmt.Println("\nFlags for new:")
	fmt.Println("  --gpu               Enable GPU acceleration")
	fmt.Println("  --headless          Enable headless mode")
	fmt.Println("  --role <name>       Tag launched browser role (dev|test)")
	fmt.Println("  --reuse-existing    Reuse existing matching role/headless instance")
	fmt.Println("  --user-data-dir     Set explicit profile directory")
	fmt.Println("  --debug-address     Set remote debug bind address (127.0.0.1 or 0.0.0.0)")
	fmt.Println("\nFlags for kill:")
	fmt.Println("  --all               Kill ALL Chrome/Edge processes system-wide")
	fmt.Println("  --windows           Use with 'kill' for WSL host processes (auto-detected usually)")
	fmt.Println("\nMesh Flags:")
	fmt.Println("  --nodes <csv|all>   Node filter (ex: chroma,darkmac,legion)")
	fmt.Println("  --node <name>       Single node for remote-new/remote-relay")
	fmt.Println("\nGeneral Options:")
	fmt.Printf("  --port %d         Remote debugging port\n", chrome.DefaultDebugPort)
	fmt.Println("  --debug             Enable verbose logging")
	fmt.Println("  --filter <expr>     Test step filter (for chrome test)")
}

func normalizeChromeArgs(args []string) ([]string, bool, error) {
	if len(args) == 0 {
		return nil, false, fmt.Errorf("missing arguments")
	}
	if isHelpArg(args[0]) {
		return []string{"help"}, false, nil
	}
	if strings.HasPrefix(args[0], "src_v") {
		if args[0] != "src_v1" {
			return nil, false, fmt.Errorf("unsupported version %s", args[0])
		}
		if len(args) < 2 {
			return nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh chrome src_v1 <command> [args])")
		}
		return append([]string{args[1]}, args[2:]...), false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		if args[1] != "src_v1" {
			return nil, false, fmt.Errorf("unsupported version %s", args[1])
		}
		return append([]string{args[0]}, args[2:]...), true, nil
	}
	if isHelpArg(args[0]) {
		return []string{"help"}, false, nil
	}
	return nil, false, fmt.Errorf("expected version as first chrome argument (usage: ./dialtone.sh chrome src_v1 <command> [args])")
}

func isHelpArg(s string) bool {
	switch strings.TrimSpace(s) {
	case "help", "--help", "-h":
		return true
	default:
		return false
	}
}

func runChromeTests(args []string) error {
	paths, err := chrome.ResolvePaths("")
	if err != nil {
		return err
	}
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	runArgs := []string{"run", "./plugins/chrome/src_v1/test/cmd/main.go"}
	runArgs = append(runArgs, args...)
	cmd := exec.Command(goBin, runArgs...)
	cmd.Dir = paths.Runtime.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

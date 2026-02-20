package cli

import (
	"flag"
	"fmt"

	"strings"

	"dialtone/dev/plugins/logs/src_v1/go"
	chrome "dialtone/dev/plugins/chrome/app"
	chrome_test "dialtone/dev/plugins/chrome/test"
)

// RunChrome handles the 'chrome' command
func RunChrome(args []string) {
	if len(args) == 0 {
		printChromeUsage()
		return
	}

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
				fmt.Println("Usage: dialtone chrome list [flags]")
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
				fmt.Println("Usage: dialtone chrome kill [PID|all] [flags]")
				fmt.Println("\nTerminates Chrome processes. Default behavior is to only kill Dialtone-originated instances.")
				fmt.Println("\nFlags:")
				killFlags.PrintDefaults()
				fmt.Println("\nExamples:")
				fmt.Println("  dialtone chrome kill all        # Kill only Dialtone-started browsers")
				fmt.Println("  dialtone chrome kill all --all  # Kill EVERY Chrome process on the PC")
				fmt.Println("  dialtone chrome kill 1234       # Kill specific process by PID")
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
		debug := newFlags.Bool("debug", false, "Enable verbose logging")

		for _, arg := range args[1:] {
			if arg == "--help" || arg == "-h" {
				fmt.Println("Usage: dialtone chrome new [URL] [flags]")
				fmt.Println("\nLaunches a new headed Chrome instance linked to Dialtone.")
				fmt.Println("\nFlags:")
				newFlags.PrintDefaults()
				return
			}
		}

		// Pre-process arguments to separate flags from the positional URL
		var flags []string
		var positional []string
		for i := 1; i < len(args); i++ {
			arg := args[i]
			if strings.HasPrefix(arg, "-") {
				flags = append(flags, arg)
				// Handle flags that take values
				if (arg == "--port" || arg == "-port") && i+1 < len(args) {
					flags = append(flags, args[i+1])
					i++
				}
			} else {
				positional = append(positional, arg)
			}
		}

		newFlags.Parse(flags)

		targetURL := ""
		if len(positional) > 0 {
			targetURL = positional[0]
		}
		handleNew(*port, *gpu, *headless, targetURL, *role, *reuseExisting, *debug)
	case "test":
		if err := chrome_test.Run(); err != nil {
			logs.Fatal("Chrome self-test failed: %v", err)
		}
		logs.Info("Chrome self-test passed")
	case "verify":
		verifyFlags := flag.NewFlagSet("chrome verify", flag.ExitOnError)
		port := verifyFlags.Int("port", 9222, "Remote debugging port")
		debug := verifyFlags.Bool("debug", false, "Enable verbose logging")

		for _, arg := range args[1:] {
			if arg == "--help" || arg == "-h" {
				fmt.Println("Usage: dialtone chrome verify [flags]")
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

func handleNew(port int, gpu bool, headless bool, targetURL, role string, reuseExisting, debug bool) {
	logs.Info("Launching new %s Chrome instance...", func() string {
		if headless {
			return "headless"
		}
		return "headed"
	}())
	// If port is the default 9222, let's try to find a free one to avoid conflicts if 9222 is taken
	if port == 9222 {
		port = 0 // app.LaunchChrome will find one
	}

	res, err := chrome.StartSession(chrome.SessionOptions{
		RequestedPort: port,
		GPU:           gpu,
		Headless:      headless,
		TargetURL:     targetURL,
		Role:          role,
		ReuseExisting: reuseExisting,
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

func printChromeUsage() {
	fmt.Println("Usage: dialtone chrome <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  verify [--port N]   Verify chrome connectivity")
	fmt.Println("  list [flags]        List detected chrome processes")
	fmt.Println("  new [URL] [flags]   Launch a new Chrome instance")
	fmt.Println("  test                Run chrome plugin self-test (dev vs smoke roles)")
	fmt.Println("  kill [PID|all] [--all] Kill Dialtone processes (default) or all processes")
	fmt.Println("  install             Install chrome dependencies")
	fmt.Println("\nFlags for list:")
	fmt.Println("  --headed            Filter for headed instances only")
	fmt.Println("  --headless          Filter for headless instances only")
	fmt.Println("  --verbose, -v       Show full command line report")
	fmt.Println("\nFlags for new:")
	fmt.Println("  --gpu               Enable GPU acceleration")
	fmt.Println("  --headless          Enable headless mode")
	fmt.Println("  --role <name>       Tag launched browser role (dev, smoke, etc.)")
	fmt.Println("  --reuse-existing    Reuse existing matching role/headless instance")
	fmt.Println("\nFlags for kill:")
	fmt.Println("  --all               Kill ALL Chrome/Edge processes system-wide")
	fmt.Println("  --windows           Use with 'kill' for WSL host processes (auto-detected usually)")
	fmt.Println("\nGeneral Options:")
	fmt.Println("  --port 9222         Remote debugging port")
	fmt.Println("  --debug             Enable verbose logging")
}

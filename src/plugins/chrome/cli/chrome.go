package cli

import (
	"flag"
	"fmt"

	"strings"

	"dialtone/cli/src/core/logger"
	chrome "dialtone/cli/src/plugins/chrome/app"
)

// RunChrome handles the 'chrome' command
func RunChrome(args []string) {
	if len(args) == 0 {
		printChromeUsage()
		return
	}

	switch args[0] {
	case "list":
		listFlags := flag.NewFlagSet("chrome list", flag.ExitOnError)
		headed := listFlags.Bool("headed", false, "Show only headed processes")
		headless := listFlags.Bool("headless", false, "Show only headless processes")
		verbose := listFlags.Bool("verbose", false, "Show full command line report")
		listFlags.BoolVar(verbose, "v", false, "Alias for --verbose")
		listFlags.Parse(args[1:])
		handleList(*headed, *headless, *verbose)
	case "kill":
		killFlags := flag.NewFlagSet("chrome kill", flag.ExitOnError)
		isWindows := killFlags.Bool("windows", false, "Use for WSL host processes")
		totalAll := killFlags.Bool("all", false, "Kill ALL Chrome/Edge processes system-wide")
		
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
		port := newFlags.Int("port", 0, "Remote debugging port")
		gpu := newFlags.Bool("gpu", false, "Enable GPU acceleration")
		debug := newFlags.Bool("debug", false, "Enable verbose logging")

		// Pre-process arguments to separate flags from the positional URL
		var flags []string
		var positional []string
		for i := 1; i < len(args); i++ {
			arg := args[i]
			if strings.HasPrefix(arg, "-") {
				flags = append(flags, arg)
				// Handle flags that take values (only -port for now)
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
		handleNew(*port, *gpu, targetURL, *debug)
	case "verify":
		verifyFlags := flag.NewFlagSet("chrome verify", flag.ExitOnError)
		port := verifyFlags.Int("port", 9222, "Remote debugging port")
		debug := verifyFlags.Bool("debug", false, "Enable verbose logging")
		verifyFlags.Parse(args[1:])
		verifyChrome(*port, *debug)
	case "install":
		logger.LogInfo("Chrome plugin: No specific dependencies to install (detects local Chrome).")
	default:
		printChromeUsage()
	}
}

func verifyChrome(port int, debug bool) {
	logger.LogInfo("Verifying Chrome/Chromium connectivity (Target Port: %d)...", port)
	if err := chrome.VerifyChrome(port, debug); err != nil {
		logger.LogFatal("Chrome verification FAILED: %v", err)
	}
	logger.LogInfo("Chrome verification SUCCESS")
}

func handleList(headedOnly, headlessOnly, verbose bool) {
	logger.LogInfo("Scanning for Chrome/Chromium resources...")
	// We always get all and filter/categorize here
	procs, err := chrome.ListResources(true) 
	if err != nil {
		logger.LogFatal("Failed to list resources: %v", err)
	}

	if len(procs) == 0 {
		logger.LogInfo("No Chrome processes detected.")
		return
	}

	if verbose {
		fmt.Printf("\n%-8s %-8s %-8s %-10s %-6s %-10s %-8s %-10s %-8s %-5s %s\n", "PID", "PPID", "HEADLESS", "ORIGIN", "%CPU", "MEM(MB)", "CHILDREN", "PLATFORM", "PORT", "GPU", "COMMAND")
		fmt.Println(strings.Repeat("-", 180))
	} else {
		fmt.Printf("\n%-8s %-8s %-8s %-10s %-10s %-6s %-10s %-8s %-5s %-10s\n", "PID", "PPID", "HEADLESS", "ORIGIN", "TYPE", "%CPU", "MEM(MB)", "PORT", "GPU", "PLATFORM")
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

			fmt.Printf("%-8d %-8d %-8s %-10s %-6s %-10.1f %-8d %-10s %-8s %-5s %s\n", 
				p.PID, p.PPID, headless, p.Origin, cpuStr, p.MemoryMB, p.ChildCount, platform, portStr, gpuStatus, cmd)
		} else {
			fmt.Printf("%-8d %-8d %-8s %-10s %-10s %-6s %-10.1f %-8s %-5s %-10s\n", 
				p.PID, p.PPID, headless, p.Origin, procType, cpuStr, p.MemoryMB, portStr, gpuStatus, platform)
		}
		count++
	}
	fmt.Println()
	logger.LogInfo("Total: %d processes (displayed: %d)", len(procs), count)
	if !verbose {
		logger.LogInfo("Use --verbose to see the full command line report.")
	}
}

func handleKill(arg string, isWindows, totalAll bool) {
	if arg == "all" {
		if totalAll {
			logger.LogInfo("Killing ALL Chrome processes system-wide...")
			if err := chrome.KillAllResources(); err != nil {
				logger.LogFatal("Failed to kill all resources: %v", err)
			}
			logger.LogInfo("Successfully killed all Chrome processes")
		} else {
			logger.LogInfo("Killing all Dialtone Chrome instances...")
			if err := chrome.KillDialtoneResources(); err != nil {
				logger.LogFatal("Failed to kill Dialtone resources: %v", err)
			}
			logger.LogInfo("Successfully killed Dialtone Chrome processes")
		}
		return
	}

	var pid int
	fmt.Sscanf(arg, "%d", &pid)
	if pid == 0 {
		logger.LogFatal("Invalid PID: %s", arg)
	}

	logger.LogInfo("Killing Chrome process PID %d...", pid)
	
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
		logger.LogFatal("Failed to kill resource: %v", err)
	}
	logger.LogInfo("Successfully killed process %d", pid)
}

func handleNew(port int, gpu bool, targetURL string, debug bool) {
	logger.LogInfo("Launching new headed Chrome instance...")
	// If port is the default 9222, let's try to find a free one to avoid conflicts if 9222 is taken
	if port == 9222 {
		port = 0 // app.LaunchChrome will find one
	}

	res, err := chrome.LaunchChrome(port, gpu, targetURL)
	if err != nil {
		logger.LogFatal("Failed to launch Chrome: %v", err)
	}

	fmt.Println("\n🚀 Chrome started successfully!")
	fmt.Printf("%-15s: %d\n", "PID", res.PID)
	fmt.Printf("%-15s: %d\n", "Debug Port", res.Port)
	fmt.Printf("%-15s: %s\n", "WebSocket URL", res.WebsocketURL)
	fmt.Println()
	logger.LogInfo("You can now connect to this instance using the WebSocket URL.")
}

func printChromeUsage() {
	fmt.Println("Usage: dialtone chrome <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  verify [--port N]   Verify chrome connectivity")
	fmt.Println("  list [flags]        List detected chrome processes")
	fmt.Println("  new [URL] [flags]   Launch a new headed chrome instance")
	fmt.Println("  kill [PID|all] [--all] Kill Dialtone processes (default) or all processes")
	fmt.Println("  install             Install chrome dependencies")
	fmt.Println("\nFlags for list:")
	fmt.Println("  --headed            Filter for headed instances only")
	fmt.Println("  --headless          Filter for headless instances only")
	fmt.Println("  --verbose, -v       Show full command line report")
	fmt.Println("\nFlags for new:")
	fmt.Println("  --gpu               Enable GPU acceleration")
	fmt.Println("\nFlags for kill:")
	fmt.Println("  --all               Kill ALL Chrome/Edge processes system-wide")
	fmt.Println("  --windows           Use with 'kill' for WSL host processes (auto-detected usually)")
	fmt.Println("\nGeneral Options:")
	fmt.Println("  --port 9222         Remote debugging port")
	fmt.Println("  --debug             Enable verbose logging")
}

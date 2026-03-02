package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
	"dialtone/dev/plugins/logs/src_v1/go"
	"github.com/chromedp/cdproto/browser"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
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
		host := listFlags.String("host", "", "List instances on a host or 'all' (remote)")
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
		handleListWithHost(strings.TrimSpace(*host), *headed, *headless, *verbose)
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
		gpu := newFlags.Bool("gpu", true, "Enable GPU acceleration")
		headless := newFlags.Bool("headless", false, "Launch in headless mode")
		kiosk := newFlags.Bool("kiosk", false, "Launch in kiosk mode (headed only)")
		role := newFlags.String("role", "", "Dialtone role tag (e.g. dev, smoke)")
		reuseExisting := newFlags.Bool("reuse-existing", false, "Attach to existing matching role/headless instance")
		userDataDir := newFlags.String("user-data-dir", "", "Explicit Chrome user data dir")
		debugAddress := newFlags.String("debug-address", "", "Remote debug bind address (empty=auto)")
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
		handleNew(*port, *gpu, *headless, *kiosk, targetURL, *role, *reuseExisting, *userDataDir, *debugAddress, *debug)
	case "open":
		handleOpenCmd(args[1:])
	case "dashboard":
		handleDashboardCmd(args[1:])
	case "click":
		handleClickCmd(args[1:])
	case "assert-url":
		handleAssertURLCmd(args[1:])
	case "session":
		sessionFlags := flag.NewFlagSet("chrome session", flag.ExitOnError)
		port := sessionFlags.Int("port", 0, "Remote debugging port (0 for auto)")
		gpu := sessionFlags.Bool("gpu", true, "Enable GPU acceleration")
		headless := sessionFlags.Bool("headless", true, "Launch in headless mode")
		role := sessionFlags.String("role", "", "Dialtone role tag (e.g. dev, smoke)")
		reuseExisting := sessionFlags.Bool("reuse-existing", true, "Attach to existing matching role/headless instance")
		userDataDir := sessionFlags.String("user-data-dir", "", "Explicit Chrome user data dir")
		debugAddress := sessionFlags.String("debug-address", "", "Remote debug bind address (empty=auto)")
		url := sessionFlags.String("url", "about:blank", "Initial URL")
		if err := sessionFlags.Parse(args[1:]); err != nil {
			logs.Fatal("session parse failed: %v", err)
		}
		handleSession(*port, *gpu, *headless, *url, *role, *reuseExisting, *userDataDir, *debugAddress)
	case "debug-url":
		handleDebugURLCmd(args[1:])
	case "service-start":
		handleServiceStartCmd(args[1:])
	case "service-daemon":
		handleServiceDaemonCmd(args[1:])
	case "service-stop":
		handleServiceStopCmd(args[1:])
	case "service-status":
		handleServiceStatusCmd(args[1:])
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
		handleInstallCmd(args[1:])
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
	case "deploy":
		handleDeployCmd(args[1:])
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

func handleNew(port int, gpu bool, headless bool, kiosk bool, targetURL, role string, reuseExisting bool, userDataDir string, debugAddress string, debug bool) {
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
		Kiosk:         kiosk,
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

func handleDebugURLCmd(args []string) {
	fs := flag.NewFlagSet("chrome debug-url", flag.ExitOnError)
	host := fs.String("host", "", "Optional mesh node name (example: darkmac)")
	servicePort := fs.Int("service-port", defaultChromeServicePort, "Remote chrome service command port")
	role := fs.String("role", "dev", "Dialtone role tag")
	headless := fs.Bool("headless", false, "Request headless session when launching")
	url := fs.String("url", "about:blank", "Initial URL when launching")
	reuse := fs.Bool("reuse-existing", true, "Reuse an existing matching session")
	port := fs.Int("port", 0, "Preferred debugging port")
	userDataDir := fs.String("user-data-dir", "", "Explicit Chrome user data dir")
	debugAddress := fs.String("debug-address", "0.0.0.0", "Remote debug bind address")
	_ = fs.Parse(args)

	if strings.TrimSpace(*host) != "" {
		if ws, err := requestRemoteServiceDebugURL(strings.TrimSpace(*host), *servicePort, debugURLRequest{
			Role:         strings.TrimSpace(*role),
			Headless:     *headless,
			URL:          strings.TrimSpace(*url),
			Port:         *port,
			Reuse:        *reuse,
			UserDataDir:  strings.TrimSpace(*userDataDir),
			DebugAddress: strings.TrimSpace(*debugAddress),
		}); err == nil && strings.TrimSpace(ws) != "" {
			fmt.Println(strings.TrimSpace(ws))
			return
		} else if err != nil {
			logs.Fatal("debug-url --host %s failed: %v", strings.TrimSpace(*host), err)
		}
	}

	sess, err := chrome.StartSession(chrome.SessionOptions{
		RequestedPort: *port,
		GPU:           true,
		Headless:      *headless,
		TargetURL:     strings.TrimSpace(*url),
		Role:          strings.TrimSpace(*role),
		ReuseExisting: *reuse,
		UserDataDir:   strings.TrimSpace(*userDataDir),
		DebugAddress:  strings.TrimSpace(*debugAddress),
	})
	if err != nil {
		logs.Fatal("debug-url failed: %v", err)
	}
	fmt.Println(strings.TrimSpace(sess.WebSocketURL))
}

func handleServiceDaemonCmd(args []string) {
	fs := flag.NewFlagSet("chrome service-daemon", flag.ExitOnError)
	listenAddr := fs.String("listen-address", "0.0.0.0", "Service listen address")
	listenPort := fs.Int("listen-port", defaultChromeServicePort, "Service listen port")
	defaultRole := fs.String("role", "dev", "Default role when request omits role")
	defaultDebugAddress := fs.String("debug-address", "0.0.0.0", "Default debug bind address")
	_ = fs.Parse(args)
	var proxyPort int64

	mux := http.NewServeMux()
	buildProxy := func(port int) *httputil.ReverseProxy {
		target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
		proxy := httputil.NewSingleHostReverseProxy(target)
		origDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			origDirector(req)
			req.Host = target.Host
		}
		return proxy
	}
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.HandleFunc("/processes", handleProcessStats)
	mux.HandleFunc("/process-ui", handleProcessUI)
	mux.HandleFunc("/ws/processes", handleProcessStatsWS)
	mux.HandleFunc("/debug-url", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		req := debugURLRequest{
			Role:         strings.TrimSpace(*defaultRole),
			URL:          "about:blank",
			Reuse:        true,
			DebugAddress: strings.TrimSpace(*defaultDebugAddress),
		}
		if r.Method == http.MethodPost && r.Body != nil {
			dec := json.NewDecoder(r.Body)
			_ = dec.Decode(&req)
		}
		q := r.URL.Query()
		if v := strings.TrimSpace(q.Get("role")); v != "" {
			req.Role = v
		}
		if v := strings.TrimSpace(q.Get("url")); v != "" {
			req.URL = v
		}
		req.Headless = parseBoolQuery(q.Get("headless"), req.Headless)
		req.Reuse = parseBoolQuery(q.Get("reuse_existing"), req.Reuse)
		req.Port = parseIntQuery(q.Get("port"), req.Port)
		if v := strings.TrimSpace(q.Get("user_data_dir")); v != "" {
			req.UserDataDir = v
		}
		if v := strings.TrimSpace(q.Get("debug_address")); v != "" {
			req.DebugAddress = v
		}
		normalizeDebugRequestDefaults(&req, strings.TrimSpace(*defaultRole), strings.TrimSpace(*defaultDebugAddress))
		sess, err := startDaemonManagedSession(req, false)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		atomic.StoreInt64(&proxyPort, int64(sess.Port))
		wsPath := chrome.WebSocketPathFromURL(strings.TrimSpace(sess.WebSocketURL))
		if wsPath == "" {
			wsPath = "/devtools/browser"
		}
		// Return a daemon-proxied websocket URL so all traffic can traverse this service.
		proxyWS := fmt.Sprintf("ws://127.0.0.1:%d%s", *listenPort, wsPath)
		writeJSON(w, http.StatusOK, debugURLResponse{
			WebSocketURL: proxyWS,
			PID:          sess.PID,
			Port:         sess.Port,
			IsNew:        sess.IsNew,
		})
	})
	mux.HandleFunc("/open", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		req := openRequest{
			debugURLRequest: debugURLRequest{
				Role:         strings.TrimSpace(*defaultRole),
				URL:          "about:blank",
				Reuse:        true,
				DebugAddress: strings.TrimSpace(*defaultDebugAddress),
			},
		}
		if r.Method == http.MethodPost && r.Body != nil {
			dec := json.NewDecoder(r.Body)
			_ = dec.Decode(&req)
		}
		q := r.URL.Query()
		if v := strings.TrimSpace(q.Get("role")); v != "" {
			req.Role = v
		}
		if v := strings.TrimSpace(q.Get("url")); v != "" {
			req.URL = v
		}
		req.Headless = parseBoolQuery(q.Get("headless"), req.Headless)
		req.Reuse = parseBoolQuery(q.Get("reuse_existing"), req.Reuse)
		req.Port = parseIntQuery(q.Get("port"), req.Port)
		req.Fullscreen = parseBoolQuery(q.Get("fullscreen"), req.Fullscreen)
		req.Kiosk = parseBoolQuery(q.Get("kiosk"), req.Kiosk)
		if v := strings.TrimSpace(q.Get("user_data_dir")); v != "" {
			req.UserDataDir = v
		}
		if v := strings.TrimSpace(q.Get("debug_address")); v != "" {
			req.DebugAddress = v
		}
		normalizeDebugRequestDefaults(&req.debugURLRequest, strings.TrimSpace(*defaultRole), strings.TrimSpace(*defaultDebugAddress))
		sess, err := startDaemonManagedSession(req.debugURLRequest, req.Kiosk)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		atomic.StoreInt64(&proxyPort, int64(sess.Port))
		_ = serviceNavigateAndFullscreen(sess.WebSocketURL, req.URL, req.Fullscreen || req.Kiosk)
		wsPath := chrome.WebSocketPathFromURL(strings.TrimSpace(sess.WebSocketURL))
		if wsPath == "" {
			wsPath = "/devtools/browser"
		}
		proxyWS := fmt.Sprintf("ws://127.0.0.1:%d%s", *listenPort, wsPath)
		writeJSON(w, http.StatusOK, debugURLResponse{
			WebSocketURL: proxyWS,
			PID:          sess.PID,
			Port:         sess.Port,
			IsNew:        sess.IsNew,
		})
	})
	mux.HandleFunc("/action", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, actionResponse{OK: false, Error: "method not allowed"})
			return
		}
		req := actionRequest{
			debugURLRequest: debugURLRequest{
				Role:         strings.TrimSpace(*defaultRole),
				URL:          "about:blank",
				Reuse:        true,
				DebugAddress: strings.TrimSpace(*defaultDebugAddress),
			},
			Action: "click",
		}
		if r.Body != nil {
			dec := json.NewDecoder(r.Body)
			_ = dec.Decode(&req)
		}
		req.Action = strings.ToLower(strings.TrimSpace(req.Action))
		if req.Action == "" {
			req.Action = "click"
		}
		normalizeDebugRequestDefaults(&req.debugURLRequest, strings.TrimSpace(*defaultRole), strings.TrimSpace(*defaultDebugAddress))
		sess, err := startDaemonManagedSession(req.debugURLRequest, false)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, actionResponse{OK: false, Error: err.Error()})
			return
		}
		atomic.StoreInt64(&proxyPort, int64(sess.Port))
		actionLogs, actionErr := serviceRunAction(sess.Port, sess.WebSocketURL, req)
		if actionErr != nil {
			writeJSON(w, http.StatusBadRequest, actionResponse{
				OK:           false,
				Error:        actionErr.Error(),
				WebSocketURL: strings.TrimSpace(sess.WebSocketURL),
				PID:          sess.PID,
				Port:         sess.Port,
				Logs:         actionLogs,
			})
			return
		}
		writeJSON(w, http.StatusOK, actionResponse{
			OK:           true,
			WebSocketURL: strings.TrimSpace(sess.WebSocketURL),
			PID:          sess.PID,
			Port:         sess.Port,
			Logs:         actionLogs,
		})
	})
	mux.HandleFunc("/json/", func(w http.ResponseWriter, r *http.Request) {
		p := int(atomic.LoadInt64(&proxyPort))
		if p <= 0 {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "chrome proxy port unavailable; call /debug-url first"})
			return
		}
		proxy := buildProxy(p)
		proxy.ServeHTTP(w, r)
	})
	mux.HandleFunc("/devtools/", func(w http.ResponseWriter, r *http.Request) {
		p := int(atomic.LoadInt64(&proxyPort))
		if p <= 0 {
			http.Error(w, "chrome proxy port unavailable; call /debug-url first", http.StatusServiceUnavailable)
			return
		}
		proxy := buildProxy(p)
		proxy.ServeHTTP(w, r)
	})

	addr := fmt.Sprintf("%s:%d", strings.TrimSpace(*listenAddr), *listenPort)
	logs.Info("chrome service daemon listening on %s", addr)
	logs.Fatal("chrome service daemon stopped: %v", http.ListenAndServe(addr, mux))
}

func serviceNavigateAndFullscreen(wsURL, targetURL string, fullscreen bool) error {
	ctx, cancel, err := chrome.AttachToWebSocket(strings.TrimSpace(wsURL))
	if err != nil {
		return err
	}
	defer cancel()
	runCtx, runCancel := context.WithTimeout(ctx, 20*time.Second)
	defer runCancel()
	if err := chromedp.Run(runCtx, chromedp.Navigate(strings.TrimSpace(targetURL))); err != nil {
		return err
	}
	if !fullscreen {
		return nil
	}
	winID, _, err := browser.GetWindowForTarget().Do(runCtx)
	if err != nil {
		return err
	}
	return browser.SetWindowBounds(winID, &browser.Bounds{WindowState: browser.WindowStateFullscreen}).Do(runCtx)
}

func enforceSingleRoleInstance(role string, headless bool, keepPID int) {
	role = strings.TrimSpace(role)
	if role == "" || keepPID <= 0 {
		return
	}
	procs, err := chrome.ListResources(true)
	if err != nil {
		return
	}
	for _, p := range procs {
		if p.PID <= 0 || p.PID == keepPID {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(p.Origin), "dialtone") {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(p.Role), role) {
			continue
		}
		if p.IsHeadless != headless {
			continue
		}
		_ = chrome.KillResource(p.PID, p.IsWindows)
	}
}

func ensureSinglePageTab(port int) error {
	if port <= 0 {
		return nil
	}
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	listURL := fmt.Sprintf("http://127.0.0.1:%d/json/list", port)
	resp, err := client.Get(listURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	type targetInfo struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	var targets []targetInfo
	if err := json.Unmarshal(body, &targets); err != nil {
		return err
	}
	pageCount := 0
	for _, t := range targets {
		if strings.EqualFold(strings.TrimSpace(t.Type), "page") {
			pageCount++
		}
	}
	if pageCount == 1 {
		return nil
	}
	if pageCount > 1 {
		return fmt.Errorf("chrome tab guard failed: expected 1 page tab, found %d", pageCount)
	}
	createURL := fmt.Sprintf("http://127.0.0.1:%d/json/new?about:blank", port)
	r, err := client.Get(createURL)
	if err != nil {
		return fmt.Errorf("chrome tab guard failed: no page tab and create failed: %w", err)
	}
	_ = r.Body.Close()
	if r.StatusCode < 200 || r.StatusCode >= 300 {
		return fmt.Errorf("chrome tab guard failed: no page tab and create returned %s", r.Status)
	}
	return nil
}

func serviceRunAction(port int, wsURL string, req actionRequest) ([]string, error) {
	attachWS := strings.TrimSpace(wsURL)
	if resolved, err := resolveLocalPageWebSocket(port, true); err == nil && strings.TrimSpace(resolved) != "" {
		attachWS = strings.TrimSpace(resolved)
	}
	ctx, cancel, err := chrome.AttachToWebSocket(attachWS)
	if err != nil {
		return nil, err
	}
	defer cancel()

	logsOut := make([]string, 0)
	chromedp.ListenTarget(ctx, func(ev any) {
		switch e := ev.(type) {
		case *cdpruntime.EventConsoleAPICalled:
			parts := make([]string, 0, len(e.Args))
			for _, a := range e.Args {
				val := strings.TrimSpace(fmt.Sprintf("%v", a.Value))
				if val != "" {
					parts = append(parts, val)
				}
			}
			if len(parts) > 0 {
				logsOut = append(logsOut, "console: "+strings.Join(parts, " "))
			}
		case *cdpruntime.EventExceptionThrown:
			if e.ExceptionDetails.Exception != nil && strings.TrimSpace(e.ExceptionDetails.Exception.Description) != "" {
				logsOut = append(logsOut, "exception: "+strings.TrimSpace(e.ExceptionDetails.Exception.Description))
			}
		}
	})

	runCtx, runCancel := context.WithTimeout(ctx, 20*time.Second)
	defer runCancel()
	if err := chromedp.Run(runCtx, cdpruntime.Enable()); err != nil {
		return logsOut, err
	}

	switch req.Action {
	case "click", "tap":
		sel := normalizeActionSelector(req.Selector)
		if sel == "" {
			return logsOut, fmt.Errorf("selector is required for click")
		}
		actions := make([]chromedp.Action, 0, 2)
		if strings.TrimSpace(req.URL) != "" {
			actions = append(actions, chromedp.Navigate(strings.TrimSpace(req.URL)))
		}
		actions = append(actions, chromedp.Click(sel, chromedp.ByQuery))
		if err := chromedp.Run(runCtx, actions...); err != nil {
			return logsOut, err
		}
	case "navigate", "open":
		if strings.TrimSpace(req.URL) == "" {
			return logsOut, fmt.Errorf("url is required for navigate")
		}
		if err := chromedp.Run(runCtx, chromedp.Navigate(strings.TrimSpace(req.URL))); err != nil {
			return logsOut, err
		}
	case "type":
		sel := normalizeActionSelector(req.Selector)
		if sel == "" {
			return logsOut, fmt.Errorf("selector is required for type")
		}
		if err := chromedp.Run(runCtx, chromedp.SetValue(sel, strings.TrimSpace(req.Text), chromedp.ByQuery)); err != nil {
			return logsOut, err
		}
	default:
		return logsOut, fmt.Errorf("unsupported action: %s", req.Action)
	}
	return logsOut, nil
}

func resolveLocalPageWebSocket(port int, createIfMissing bool) (string, error) {
	if port <= 0 {
		return "", fmt.Errorf("invalid debug port")
	}
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	listURL := fmt.Sprintf("http://127.0.0.1:%d/json/list", port)
	resp, err := client.Get(listURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	type targetInfo struct {
		Type string `json:"type"`
		WS   string `json:"webSocketDebuggerUrl"`
	}
	var targets []targetInfo
	if err := json.Unmarshal(body, &targets); err != nil {
		return "", err
	}
	for _, t := range targets {
		if strings.EqualFold(strings.TrimSpace(t.Type), "page") && strings.TrimSpace(t.WS) != "" {
			return strings.TrimSpace(t.WS), nil
		}
	}
	if createIfMissing {
		newURL := fmt.Sprintf("http://127.0.0.1:%d/json/new?about:blank", port)
		r, err := client.Get(newURL)
		if err == nil && r != nil {
			_ = r.Body.Close()
		}
		resp2, err := client.Get(listURL)
		if err != nil {
			return "", err
		}
		defer resp2.Body.Close()
		body2, err := io.ReadAll(resp2.Body)
		if err != nil {
			return "", err
		}
		var targets2 []targetInfo
		if err := json.Unmarshal(body2, &targets2); err != nil {
			return "", err
		}
		for _, t := range targets2 {
			if strings.EqualFold(strings.TrimSpace(t.Type), "page") && strings.TrimSpace(t.WS) != "" {
				return strings.TrimSpace(t.WS), nil
			}
		}
	}
	return "", fmt.Errorf("no page websocket found")
}

func defaultServiceUserDataDir(role string, headless bool) string {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		role = "dev"
	}
	suffix := role
	if headless {
		suffix += "-headless"
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join(home, ".dialtone", "chrome", suffix+"-profile"))
}

func ensureSessionPageReady(req debugURLRequest, sess *chrome.Session) *chrome.Session {
	if sess == nil || sess.Port <= 0 {
		return sess
	}
	if _, err := resolveLocalPageWebSocket(sess.Port, true); err == nil {
		return sess
	}
	if !req.Reuse {
		return sess
	}
	_ = chrome.KillResource(sess.PID, sess.IsWindows)
	fresh, err := chrome.StartSession(chrome.SessionOptions{
		RequestedPort: sess.Port,
		GPU:           true,
		Headless:      req.Headless,
		TargetURL:     req.URL,
		Role:          req.Role,
		ReuseExisting: false,
		UserDataDir:   req.UserDataDir,
		DebugAddress:  req.DebugAddress,
	})
	if err != nil || fresh == nil {
		return sess
	}
	return fresh
}

func normalizeActionSelector(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, ".") || strings.HasPrefix(raw, "[") || strings.Contains(raw, " ") || strings.Contains(raw, ">") {
		return raw
	}
	if isLikelyHTMLTag(raw) {
		return strings.ToLower(raw)
	}
	return "#" + raw
}

func isLikelyHTMLTag(raw string) bool {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "html", "body", "main", "header", "footer", "nav", "section", "article", "div", "span", "button", "input", "textarea", "select", "a", "img", "canvas", "video":
		return true
	default:
		return false
	}
}

func handleServiceStartCmd(args []string) {
	fs := flag.NewFlagSet("chrome service-start", flag.ExitOnError)
	role := fs.String("role", "dev", "Service role tag")
	headless := fs.Bool("headless", false, "Run service browser headless")
	url := fs.String("url", "about:blank", "Initial URL")
	port := fs.Int("port", 0, "Preferred debug port")
	debugAddress := fs.String("debug-address", "", "Remote debug bind address (empty=auto)")
	_ = fs.Parse(args)
	sess, err := chrome.StartSession(chrome.SessionOptions{
		RequestedPort: *port,
		GPU:           true,
		Headless:      *headless,
		TargetURL:     strings.TrimSpace(*url),
		Role:          strings.TrimSpace(*role),
		ReuseExisting: true,
		DebugAddress:  strings.TrimSpace(*debugAddress),
	})
	if err != nil {
		logs.Fatal("service-start failed: %v", err)
	}
	logs.Info("chrome service running role=%s pid=%d port=%d ws=%s", strings.TrimSpace(*role), sess.PID, sess.Port, strings.TrimSpace(sess.WebSocketURL))
}

func handleServiceStopCmd(args []string) {
	fs := flag.NewFlagSet("chrome service-stop", flag.ExitOnError)
	role := fs.String("role", "dev", "Service role tag")
	_ = fs.Parse(args)
	procs, err := chrome.ListResources(true)
	if err != nil {
		logs.Fatal("service-stop list failed: %v", err)
	}
	stopped := 0
	for _, p := range procs {
		if p.Origin != "Dialtone" || strings.TrimSpace(p.Role) != strings.TrimSpace(*role) {
			continue
		}
		if err := chrome.KillResource(p.PID, p.IsWindows); err == nil {
			stopped++
		}
	}
	logs.Info("chrome service stopped role=%s count=%d", strings.TrimSpace(*role), stopped)
}

func handleServiceStatusCmd(args []string) {
	fs := flag.NewFlagSet("chrome service-status", flag.ExitOnError)
	role := fs.String("role", "dev", "Service role tag")
	_ = fs.Parse(args)
	procs, err := chrome.ListResources(true)
	if err != nil {
		logs.Fatal("service-status list failed: %v", err)
	}
	count := 0
	for _, p := range procs {
		if p.Origin == "Dialtone" && strings.TrimSpace(p.Role) == strings.TrimSpace(*role) {
			count++
		}
	}
	fmt.Printf("role=%s count=%d\n", strings.TrimSpace(*role), count)
}

func printChromeUsage() {
	fmt.Println("Usage: ./dialtone.sh chrome src_v1 <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  verify [--port N]   Verify chrome connectivity")
	fmt.Println("  list [flags]        List detected chrome processes")
	fmt.Println("  new [URL] [flags]   Launch a new Chrome instance")
	fmt.Println("  open [flags]        Open URL on host(s) and optionally fullscreen/kiosk it")
	fmt.Println("  dashboard [flags]   Open daemon process-monitor dashboard on host(s)")
	fmt.Println("  click <selector>    Click/tap selector via remote chrome service")
	fmt.Println("  assert-url [flags]  Verify host page URL matches expected")
	fmt.Println("  session [flags]     Launch/reuse and emit machine-readable session metadata")
	fmt.Println("  debug-url [flags]   Ensure/reuse debug session and print websocket URL")
	fmt.Println("  service-start       Start/reuse background chrome service session")
	fmt.Println("  service-daemon      Run command server for remote chrome session control")
	fmt.Println("  service-stop        Stop background chrome service session")
	fmt.Println("  service-status      Show background chrome service process count")
	fmt.Println("  test                Run chrome plugin self-test (dev/test roles)")
	fmt.Println("  kill [PID|all] [--all] Kill Dialtone processes (default) or all processes")
	fmt.Println("  remote-list [flags] List Chrome processes across mesh nodes")
	fmt.Println("  remote-new [flags]  Start or reuse Chrome on a mesh node with role tag")
	fmt.Println("  remote-probe [flags] Probe debug ports/listeners across mesh nodes")
	fmt.Println("  remote-relay [flags] Start remote TCP relay for debug port exposure")
	fmt.Println("  remote-doctor [flags] Diagnose remote debug reachability/listener issues")
	fmt.Println("  remote-kill [flags] Kill remote Chrome processes by role/origin")
	fmt.Println("  remote-wsl-forward [flags] Configure Windows WSL devtools portproxy/firewall")
	fmt.Println("  deploy [flags]      Build and deploy chrome plugin binary to mesh host")
	fmt.Println("  install             Install chrome dependencies")
	fmt.Println("\nFlags for list:")
	fmt.Println("  --headed            Filter for headed instances only")
	fmt.Println("  --headless          Filter for headless instances only")
	fmt.Println("  --verbose, -v       Show full command line report")
	fmt.Println("\nFlags for new:")
	fmt.Println("  --gpu               Enable GPU acceleration")
	fmt.Println("  --headless          Enable headless mode")
	fmt.Println("  --kiosk             Launch in kiosk mode (headed only)")
	fmt.Println("  --role <name>       Tag launched browser role (dev|test)")
	fmt.Println("  --reuse-existing    Reuse existing matching role/headless instance")
	fmt.Println("  --user-data-dir     Set explicit profile directory")
	fmt.Println("  --debug-address     Set remote debug bind address (empty=auto, or 127.0.0.1/0.0.0.0)")
	fmt.Println("\nFlags for kill:")
	fmt.Println("  --all               Kill ALL Chrome/Edge processes system-wide")
	fmt.Println("  --windows           Use with 'kill' for WSL host processes (auto-detected usually)")
	fmt.Println("\nMesh Flags:")
	fmt.Println("  --nodes <csv|all>   Node filter (ex: chroma,darkmac,legion)")
	fmt.Println("  --host <name>       Single host for remote-new/remote-relay (preferred)")
	fmt.Println("  --node <name>       Alias for --host (deprecated)")
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

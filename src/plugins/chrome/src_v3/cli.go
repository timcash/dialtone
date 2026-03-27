package src_v3

import (
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

func replIndexInfof(format string, args ...any) {
	msg := strings.TrimSpace(fmt.Sprintf(format, args...))
	if msg == "" {
		return
	}
	if strings.TrimSpace(os.Getenv("DIALTONE_INTERNAL_SUBTONE")) == "1" {
		logs.Info("DIALTONE_INDEX: %s", msg)
		return
	}
	logs.Info("%s", msg)
}

func defaultHostLabel(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return "local"
	}
	return host
}

func emitChromeCommandSummary(command, host, role, url string) {
	switch command {
	case "status":
		replIndexInfof("chrome status: checking %s role=%s", host, role)
	case "open":
		if strings.TrimSpace(url) == "" {
			replIndexInfof("chrome open: opening managed tab on %s role=%s", host, role)
		} else {
			replIndexInfof("chrome open: opening %s on %s role=%s", strings.TrimSpace(url), host, role)
		}
	case "goto":
		replIndexInfof("chrome goto: opening %s on %s role=%s", strings.TrimSpace(url), host, role)
	case "get-url":
		replIndexInfof("chrome get-url: reading managed tab URL on %s role=%s", host, role)
	case "tabs":
		replIndexInfof("chrome tabs: listing tabs on %s role=%s", host, role)
	case "tab-open":
		replIndexInfof("chrome tab-open: opening tab on %s role=%s", host, role)
	case "tab-close":
		replIndexInfof("chrome tab-close: closing tab on %s role=%s", host, role)
	case "close":
		replIndexInfof("chrome close: closing browser on %s role=%s", host, role)
	case "console":
		replIndexInfof("chrome console: reading console buffer on %s role=%s", host, role)
	}
}

func emitChromeCommandResult(command, host, role string, resp *commandResponse) {
	switch command {
	case "goto", "open":
		replIndexInfof("chrome %s: managed tab ready", command)
	case "status":
		if resp != nil && resp.BrowserPID > 0 {
			replIndexInfof("chrome status: daemon ready on %s role=%s browser_pid=%d", host, role, resp.BrowserPID)
		} else {
			replIndexInfof("chrome status: daemon ready on %s role=%s", host, role)
		}
	case "tabs":
		if resp != nil {
			replIndexInfof("chrome tabs: %d tab(s) visible", len(resp.Tabs))
		}
	case "get-url":
		if resp != nil {
			replIndexInfof("chrome get-url: %s", strings.TrimSpace(resp.CurrentURL))
		}
	case "console":
		if resp != nil {
			replIndexInfof("chrome console: captured %d line(s)", len(resp.ConsoleLines))
		}
	case "close":
		replIndexInfof("chrome close: browser stopped")
	}
}

func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}
	switch strings.TrimSpace(args[0]) {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "daemon":
		return runDaemon(args[1:])
	case "install":
		logs.Info("chrome src_v3 install: no-op")
		return nil
	case "build":
		return buildLocalBinary()
	case "deploy":
		return handleDeploy(args[1:])
	case "service":
		return handleService(args[1:])
	case "status":
		return handleRequestCommand("status", args[1:])
	case "instances":
		return handleInstances(args[1:])
	case "open":
		return handleRequestCommand("open", args[1:])
	case "goto":
		return handleRequestCommand("goto", args[1:])
	case "get-url":
		return handleRequestCommand("get-url", args[1:])
	case "tabs":
		return handleRequestCommand("tabs", args[1:])
	case "tab-open":
		return handleRequestCommand("tab-open", args[1:])
	case "tab-close":
		return handleRequestCommand("tab-close", args[1:])
	case "close":
		return handleRequestCommand("close", args[1:])
	case "close-all":
		return handleCloseAll(args[1:])
	case "click-aria":
		return handleAriaCommand("click-aria", args[1:])
	case "type-aria":
		return handleAriaCommand("type-aria", args[1:])
	case "wait-aria":
		return handleAriaWaitCommand("wait-aria", args[1:])
	case "wait-aria-attr":
		return handleAriaWaitCommand("wait-aria-attr", args[1:])
	case "get-aria-attr":
		return handleAriaGetAttrCommand(args[1:])
	case "set-html":
		return handleSetHTMLCommand(args[1:])
	case "wait-log":
		return handleWaitLogCommand(args[1:])
	case "console":
		return handleRequestCommand("console", args[1:])
	case "screenshot":
		return handleScreenshotCommand(args[1:])
	case "nats-example":
		return handleNATSExample(args[1:])
	case "test":
		return handleSmokeTest(args[1:])
	case "test-actions":
		return handleActionSmokeTest(args[1:])
	case "doctor":
		return handleDoctor(args[1:])
	case "logs":
		return handleLogs(args[1:])
	case "reset":
		return handleReset(args[1:])
	default:
		return fmt.Errorf("unknown chrome src_v3 command: %s", args[0])
	}
}

func printUsage() {
	logs.Info("Usage: ./dialtone.sh chrome src_v3 <command> [args]")
	logs.Info("Commands:")
	logs.Info("  install")
	logs.Info("  build")
	logs.Info("  deploy [--host <host>] [--service]")
	logs.Info("  service [--host <host>] --mode start|stop|status")
	logs.Info("  status [--host <host>]")
	logs.Info("  instances [--host <host>] [--role <role>]")
	logs.Info("  open [--host <host>] --url <url>")
	logs.Info("  goto [--host <host>] --url <url>")
	logs.Info("  get-url [--host <host>]")
	logs.Info("  tabs [--host <host>]")
	logs.Info("  tab-open [--host <host>] [--url <url>]")
	logs.Info("  tab-close [--host <host>] [--index <n>]")
	logs.Info("  close [--host <host>]")
	logs.Info("  close-all [--host <host>] [--role <role>]")
	logs.Info("  click-aria [--host <host>] --label <aria-label>")
	logs.Info("  type-aria [--host <host>] --label <aria-label> --value <text>")
	logs.Info("  wait-aria [--host <host>] --label <aria-label> [--timeout-ms 5000]")
	logs.Info("  wait-aria-attr [--host <host>] --label <aria-label> --attr <name> --expected <value> [--timeout-ms 5000]")
	logs.Info("  get-aria-attr [--host <host>] --label <aria-label> --attr <name>")
	logs.Info("  set-html [--host <host>] --value <html>")
	logs.Info("  wait-log [--host <host>] --contains <text> [--timeout-ms 5000]")
	logs.Info("  console [--host <host>]")
	logs.Info("  screenshot [--host <host>] --out <png-path>")
	logs.Info("  nats-example [--host <host>] [--role <role>]")
	logs.Info("  test [--host <host>]")
	logs.Info("  test-actions [--host <host>]")
	logs.Info("  doctor [--host <host>]")
	logs.Info("  logs [--host <host>]")
	logs.Info("  reset [--host <host>]")
	logs.Info("  daemon --role dev --chrome-port 19464 --nats-port 19465")
	logs.Info("NATS example:")
	logs.Info("  %s", NATSExample("<host>", defaultRole))
}

func buildLocalBinary() error {
	return buildBinaryFor(localBinaryPathFor(runtime.GOOS, runtime.GOARCH), runtime.GOOS, runtime.GOARCH)
}

func defaultChromeTestHost() string {
	if v := strings.TrimSpace(os.Getenv("DIALTONE_CHROME_TEST_HOST")); v != "" {
		return v
	}
	if strings.TrimSpace(os.Getenv("WSL_DISTRO_NAME")) == "" {
		return ""
	}
	if _, err := sshv1.ResolveMeshNode("legion"); err == nil {
		return "legion"
	}
	return ""
}

func hasFlag(args []string, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	prefix := name + "="
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg == name || strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}

func handleDeploy(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 deploy", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host (optional; default local)")
	role := fs.String("role", defaultRole, "Chrome role")
	service := fs.Bool("service", false, "Start service after deploy")
	_ = fs.Parse(args)
	targetHost := defaultHostLabel(*host)
	if *service {
		replIndexInfof("chrome deploy: syncing binary to %s role=%s and starting service", targetHost, strings.TrimSpace(*role))
	} else {
		replIndexInfof("chrome deploy: syncing binary to %s role=%s", targetHost, strings.TrimSpace(*role))
	}
	return deployTarget(strings.TrimSpace(*host), strings.TrimSpace(*role), *service)
}

func handleService(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 service", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host (optional; default local)")
	mode := fs.String("mode", "status", "start|stop|status")
	role := fs.String("role", defaultRole, "Chrome role")
	_ = fs.Parse(args)
	targetHost := defaultHostLabel(*host)
	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "start":
		replIndexInfof("chrome service: starting on %s role=%s", targetHost, strings.TrimSpace(*role))
	case "stop":
		replIndexInfof("chrome service: stopping on %s role=%s", targetHost, strings.TrimSpace(*role))
	default:
		replIndexInfof("chrome service: checking on %s role=%s", targetHost, strings.TrimSpace(*role))
	}
	resp, err := serviceTarget(strings.TrimSpace(*host), strings.TrimSpace(*mode), strings.TrimSpace(*role))
	if err != nil {
		return err
	}
	if resp != nil {
		printResponse(resp)
	}
	return nil
}

func handleInstances(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 instances", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host (optional; default local)")
	role := fs.String("role", "", "Chrome role filter")
	_ = fs.Parse(args)
	targetHost := defaultHostLabel(*host)
	replIndexInfof("chrome instances: scanning managed chrome browsers on %s", targetHost)
	resources, err := listResourcesByTarget(strings.TrimSpace(*host), true)
	if err != nil {
		return err
	}
	summaries := summarizeManagedResources(resources, strings.TrimSpace(*role))
	active := make([]managedRoleSummary, 0, len(summaries))
	totalBrowsers := 0
	for _, item := range summaries {
		if len(item.BrowserPIDs) == 0 {
			continue
		}
		active = append(active, item)
		totalBrowsers += len(item.BrowserPIDs)
	}
	replIndexInfof("chrome instances: found %d managed browser instance(s) across %d role(s) on %s", totalBrowsers, len(active), targetHost)
	for _, item := range active {
		fmt.Printf("INSTANCE role=%s browser_count=%d browser_pids=%s daemon_count=%d daemon_pids=%s headless=%t windows=%t\n",
			item.Role,
			len(item.BrowserPIDs),
			formatPIDList(item.BrowserPIDs),
			len(item.DaemonPIDs),
			formatPIDList(item.DaemonPIDs),
			item.IsHeadless,
			item.IsWindows,
		)
	}
	fmt.Printf("TOTAL browser_count=%d role_count=%d\n", totalBrowsers, len(active))
	return nil
}

func handleRequestCommand(command string, args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 "+command, flag.ExitOnError)
	host := fs.String("host", "", "Mesh host (optional; default local)")
	role := fs.String("role", defaultRole, "Chrome role")
	rawURL := fs.String("url", "", "URL")
	index := fs.Int("index", -1, "Tab index")
	_ = fs.Parse(args)
	req := commandRequest{
		Command: command,
		Role:    strings.TrimSpace(*role),
		URL:     normalizeURL(strings.TrimSpace(*rawURL)),
		Index:   *index,
	}
	autoStart := command != "status" && command != "instances" && command != "close"
	targetHost := defaultHostLabel(*host)
	emitChromeCommandSummary(command, targetHost, strings.TrimSpace(*role), req.URL)
	if autoStart {
		replIndexInfof("chrome service: ensuring daemon on %s role=%s", targetHost, strings.TrimSpace(*role))
	}
	resp, err := sendCommandByTarget(strings.TrimSpace(*host), req, autoStart)
	if err != nil {
		return err
	}
	emitChromeCommandResult(command, targetHost, strings.TrimSpace(*role), resp)
	printResponse(resp)
	return nil
}

func handleCloseAll(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 close-all", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host (optional; default local)")
	role := fs.String("role", "", "Chrome role filter")
	_ = fs.Parse(args)
	targetHost := defaultHostLabel(*host)
	replIndexInfof("chrome close-all: closing managed chrome browsers on %s", targetHost)
	before, after, err := closeManagedBrowsersByTarget(strings.TrimSpace(*host), strings.TrimSpace(*role))
	if err != nil {
		return err
	}
	activeBefore := make([]managedRoleSummary, 0, len(before))
	for _, item := range before {
		if len(item.BrowserPIDs) == 0 {
			continue
		}
		activeBefore = append(activeBefore, item)
	}
	beforeByRole := map[string]managedRoleSummary{}
	for _, item := range activeBefore {
		beforeByRole[item.Role] = item
	}
	afterByRole := map[string]managedRoleSummary{}
	for _, item := range after {
		afterByRole[item.Role] = item
	}
	totalClosed := 0
	totalRemaining := 0
	for _, item := range activeBefore {
		remaining := afterByRole[item.Role]
		closed := len(item.BrowserPIDs) - len(remaining.BrowserPIDs)
		if closed < 0 {
			closed = 0
		}
		totalClosed += closed
		totalRemaining += len(remaining.BrowserPIDs)
		fmt.Printf("CLOSE role=%s before=%d after=%d browser_pids_before=%s browser_pids_after=%s daemon_count=%d\n",
			item.Role,
			len(item.BrowserPIDs),
			len(remaining.BrowserPIDs),
			formatPIDList(item.BrowserPIDs),
			formatPIDList(remaining.BrowserPIDs),
			len(beforeByRole[item.Role].DaemonPIDs),
		)
	}
	replIndexInfof("chrome close-all: closed %d managed browser instance(s) on %s", totalClosed, targetHost)
	fmt.Printf("TOTAL closed_browser_count=%d remaining_browser_count=%d role_count=%d\n", totalClosed, totalRemaining, len(activeBefore))
	if totalRemaining > 0 {
		return fmt.Errorf("managed chrome instances still running on %s", targetHost)
	}
	return nil
}

func handleSmokeTest(args []string) error {
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	runArgs := []string{"run", "./plugins/chrome/src_v3/test/cmd/main.go"}
	if !hasFlag(args, "--host") {
		if host := defaultChromeTestHost(); host != "" {
			runArgs = append(runArgs, "--host", host)
		}
	}
	runArgs = append(runArgs, args...)
	cmd := exec.Command(goBin, runArgs...)
	cmd.Dir = resolveSrcRoot()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func handleAriaCommand(command string, args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 "+command, flag.ExitOnError)
	host := fs.String("host", "", "Mesh host (optional; default local)")
	role := fs.String("role", defaultRole, "Chrome role")
	label := fs.String("label", "", "ARIA label")
	value := fs.String("value", "", "Text to type")
	_ = fs.Parse(args)
	if strings.TrimSpace(*label) == "" {
		return fmt.Errorf("%s requires --label", command)
	}
	targetHost := defaultHostLabel(*host)
	switch command {
	case "click-aria":
		replIndexInfof("chrome click: clicking aria-label %q on %s role=%s", strings.TrimSpace(*label), targetHost, strings.TrimSpace(*role))
	case "type-aria":
		replIndexInfof("chrome type: typing into aria-label %q on %s role=%s", strings.TrimSpace(*label), targetHost, strings.TrimSpace(*role))
	}
	replIndexInfof("chrome service: ensuring daemon on %s role=%s", targetHost, strings.TrimSpace(*role))
	resp, err := sendCommandByTarget(strings.TrimSpace(*host), commandRequest{
		Command:   command,
		Role:      strings.TrimSpace(*role),
		AriaLabel: strings.TrimSpace(*label),
		Value:     *value,
	}, true)
	if err != nil {
		return err
	}
	if command == "click-aria" {
		replIndexInfof("chrome click: action completed")
	} else {
		replIndexInfof("chrome type: input updated")
	}
	printResponse(resp)
	return nil
}

func handleAriaWaitCommand(command string, args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 "+command, flag.ExitOnError)
	host := fs.String("host", "", "Mesh host (optional; default local)")
	role := fs.String("role", defaultRole, "Chrome role")
	label := fs.String("label", "", "ARIA label")
	attr := fs.String("attr", "", "Attribute name")
	expected := fs.String("expected", "", "Expected attribute value")
	timeoutMS := fs.Int("timeout-ms", 5000, "Timeout milliseconds")
	_ = fs.Parse(args)
	if strings.TrimSpace(*label) == "" {
		return fmt.Errorf("%s requires --label", command)
	}
	targetHost := defaultHostLabel(*host)
	if command == "wait-aria" {
		replIndexInfof("chrome wait: waiting for aria-label %q on %s role=%s", strings.TrimSpace(*label), targetHost, strings.TrimSpace(*role))
	} else {
		replIndexInfof("chrome wait: waiting for %q attr %q=%q on %s role=%s", strings.TrimSpace(*label), strings.TrimSpace(*attr), *expected, targetHost, strings.TrimSpace(*role))
	}
	replIndexInfof("chrome service: ensuring daemon on %s role=%s", targetHost, strings.TrimSpace(*role))
	req := commandRequest{
		Command:   command,
		Role:      strings.TrimSpace(*role),
		AriaLabel: strings.TrimSpace(*label),
		TimeoutMS: *timeoutMS,
	}
	if command == "wait-aria-attr" {
		req.Attr = strings.TrimSpace(*attr)
		req.Expected = *expected
	}
	resp, err := sendCommandByTarget(strings.TrimSpace(*host), req, true)
	if err != nil {
		return err
	}
	replIndexInfof("chrome wait: condition satisfied")
	printResponse(resp)
	return nil
}

func handleAriaGetAttrCommand(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 get-aria-attr", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host (optional; default local)")
	role := fs.String("role", defaultRole, "Chrome role")
	label := fs.String("label", "", "ARIA label")
	attr := fs.String("attr", "", "Attribute name")
	_ = fs.Parse(args)
	if strings.TrimSpace(*label) == "" {
		return fmt.Errorf("get-aria-attr requires --label")
	}
	if strings.TrimSpace(*attr) == "" {
		return fmt.Errorf("get-aria-attr requires --attr")
	}
	targetHost := defaultHostLabel(*host)
	replIndexInfof("chrome inspect: reading aria-label %q attr %q on %s role=%s", strings.TrimSpace(*label), strings.TrimSpace(*attr), targetHost, strings.TrimSpace(*role))
	replIndexInfof("chrome service: ensuring daemon on %s role=%s", targetHost, strings.TrimSpace(*role))
	resp, err := sendCommandByTarget(strings.TrimSpace(*host), commandRequest{
		Command:   "get-aria-attr",
		Role:      strings.TrimSpace(*role),
		AriaLabel: strings.TrimSpace(*label),
		Attr:      strings.TrimSpace(*attr),
	}, true)
	if err != nil {
		return err
	}
	replIndexInfof("chrome inspect: value captured")
	printResponse(resp)
	if strings.TrimSpace(resp.Value) != "" {
		logs.Raw("%s", resp.Value)
	}
	return nil
}

func handleWaitLogCommand(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 wait-log", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host (optional; default local)")
	role := fs.String("role", defaultRole, "Chrome role")
	contains := fs.String("contains", "", "Substring to wait for")
	timeoutMS := fs.Int("timeout-ms", 5000, "Timeout in milliseconds")
	_ = fs.Parse(args)
	if strings.TrimSpace(*contains) == "" {
		return fmt.Errorf("wait-log requires --contains")
	}
	targetHost := defaultHostLabel(*host)
	replIndexInfof("chrome wait-log: waiting for %q on %s role=%s", strings.TrimSpace(*contains), targetHost, strings.TrimSpace(*role))
	replIndexInfof("chrome service: ensuring daemon on %s role=%s", targetHost, strings.TrimSpace(*role))
	resp, err := sendCommandByTarget(strings.TrimSpace(*host), commandRequest{
		Command:   "wait-log",
		Role:      strings.TrimSpace(*role),
		Contains:  strings.TrimSpace(*contains),
		TimeoutMS: *timeoutMS,
	}, true)
	if err != nil {
		return err
	}
	replIndexInfof("chrome wait-log: log observed")
	printResponse(resp)
	return nil
}

func handleSetHTMLCommand(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 set-html", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host (optional; default local)")
	role := fs.String("role", defaultRole, "Chrome role")
	value := fs.String("value", "", "HTML markup")
	_ = fs.Parse(args)
	targetHost := defaultHostLabel(*host)
	replIndexInfof("chrome html: replacing managed tab DOM on %s role=%s", targetHost, strings.TrimSpace(*role))
	replIndexInfof("chrome service: ensuring daemon on %s role=%s", targetHost, strings.TrimSpace(*role))
	resp, err := sendCommandByTarget(strings.TrimSpace(*host), commandRequest{
		Command: "set-html",
		Role:    strings.TrimSpace(*role),
		Value:   *value,
	}, true)
	if err != nil {
		return err
	}
	replIndexInfof("chrome html: content applied")
	printResponse(resp)
	return nil
}

func handleScreenshotCommand(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 screenshot", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host (optional; default local)")
	role := fs.String("role", defaultRole, "Chrome role")
	outPath := fs.String("out", "", "Local PNG output path")
	_ = fs.Parse(args)
	if strings.TrimSpace(*outPath) == "" {
		return fmt.Errorf("screenshot requires --out")
	}
	targetHost := defaultHostLabel(*host)
	replIndexInfof("chrome screenshot: capturing managed tab on %s role=%s", targetHost, strings.TrimSpace(*role))
	replIndexInfof("chrome service: ensuring daemon on %s role=%s", targetHost, strings.TrimSpace(*role))
	resp, err := sendCommandByTarget(strings.TrimSpace(*host), commandRequest{
		Command:   "screenshot",
		Role:      strings.TrimSpace(*role),
		TimeoutMS: 60000,
	}, true)
	if err != nil {
		return err
	}
	targetPath := strings.TrimSpace(*outPath)
	if err := saveScreenshotOutput(strings.TrimSpace(*host), resp, targetPath); err != nil {
		return err
	}
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(resolveRepoRoot(), targetPath)
	}
	replIndexInfof("chrome screenshot: saved to %s", targetPath)
	printResponse(resp)
	fmt.Printf("SCREENSHOT_SAVED %s\n", targetPath)
	return nil
}

func handleActionSmokeTest(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 test-actions", flag.ExitOnError)
	host := fs.String("host", defaultChromeTestHost(), "Mesh host (optional; default local)")
	role := fs.String("role", defaultRole, "Chrome role")
	outPath := fs.String("out", filepath.Join(os.TempDir(), "chrome-src-v3-actions.png"), "Local PNG output path")
	_ = fs.Parse(args)
	marker := strconv.FormatInt(time.Now().UnixNano(), 10)
	markup := actionSmokeHTML(marker)
	steps := []commandRequest{
		{Command: "open", Role: *role, URL: "about:blank"},
		{Command: "set-html", Role: *role, Value: markup},
		{Command: "wait-log", Role: *role, Contains: "page-ready:" + marker, TimeoutMS: 8000},
		{Command: "type-aria", Role: *role, AriaLabel: "Name Input", Value: "dialtone"},
		{Command: "wait-log", Role: *role, Contains: "typed:dialtone:" + marker, TimeoutMS: 8000},
		{Command: "click-aria", Role: *role, AriaLabel: "Do Thing"},
		{Command: "wait-log", Role: *role, Contains: "clicked:" + marker, TimeoutMS: 8000},
		{Command: "screenshot", Role: *role, TimeoutMS: 60000},
	}
	for _, step := range steps {
		resp, err := sendCommandByTarget(strings.TrimSpace(*host), step, true)
		if err != nil {
			return fmt.Errorf("%s failed: %w", step.Command, err)
		}
		if step.Command == "screenshot" {
			targetPath := strings.TrimSpace(*outPath)
			if err := saveScreenshotOutput(strings.TrimSpace(*host), resp, targetPath); err != nil {
				return err
			}
			if !filepath.IsAbs(targetPath) {
				targetPath = filepath.Join(resolveRepoRoot(), targetPath)
			}
			fmt.Printf("SCREENSHOT_SAVED %s\n", targetPath)
		}
		printResponse(resp)
	}
	return nil
}

func saveScreenshotOutput(host string, resp *commandResponse, outPath string) error {
	if resp == nil {
		return fmt.Errorf("missing screenshot response")
	}
	targetPath := strings.TrimSpace(outPath)
	if targetPath == "" {
		return fmt.Errorf("screenshot output path is required")
	}
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(resolveRepoRoot(), targetPath)
	}
	data, err := screenshotBytesForTarget(strings.TrimSpace(host), resp)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(targetPath, data, 0o644)
}

func WriteScreenshotByTarget(host string, resp *CommandResponse, outPath string) error {
	return saveScreenshotOutput(strings.TrimSpace(host), resp, strings.TrimSpace(outPath))
}

func screenshotBytesForTarget(host string, resp *commandResponse) ([]byte, error) {
	if resp == nil {
		return nil, fmt.Errorf("missing screenshot response")
	}
	if raw := strings.TrimSpace(resp.ScreenshotB64); raw != "" {
		data, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("decode screenshot: %w", err)
		}
		return data, nil
	}
	path := strings.TrimSpace(resp.ScreenshotPath)
	if path == "" {
		return nil, fmt.Errorf("screenshot response missing data and path")
	}
	if isLocalHost(host) {
		return os.ReadFile(path)
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(host))
	if err != nil {
		return nil, err
	}
	var out string
	if strings.EqualFold(strings.TrimSpace(node.OS), "windows") || node.PreferWSLPowerShell {
		cmd := fmt.Sprintf("$p=%s; if (!(Test-Path -LiteralPath $p)) { throw ('missing screenshot artifact: ' + $p) }; [Convert]::ToBase64String([IO.File]::ReadAllBytes($p))", psSingleQuoted(path))
		out, err = sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
	} else {
		cmd := fmt.Sprintf("base64 -w0 %s", shellSingleQuoted(path))
		out, err = sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
	}
	if err != nil {
		return nil, err
	}
	payload := extractBase64Payload(out)
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("decode remote screenshot artifact: %w", err)
	}
	return data, nil
}

func extractBase64Payload(raw string) string {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
	if raw == "" {
		return ""
	}
	best := ""
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if line == "" {
			continue
		}
		if isBase64Line(line) && len(line) > len(best) {
			best = line
		}
	}
	if best != "" {
		return best
	}
	return raw
}

func isBase64Line(v string) bool {
	for _, r := range v {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '+', r == '/', r == '=', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func psSingleQuoted(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}

func shellSingleQuoted(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'"'"'`) + "'"
}

func actionSmokeHTML(marker string) string {
	return fmt.Sprintf(`<!doctype html>
<html>
<head><meta charset="utf-8"><title>chrome-src-v3-actions</title></head>
<body>
  <input aria-label="Name Input" oninput="console.log('typed:' + this.value + ':%s')" />
  <button aria-label="Do Thing" onclick="document.querySelector('[aria-label=&quot;Status&quot;]').textContent='clicked'; console.log('clicked:%s')">Go</button>
  <div aria-label="Status">idle</div>
  <script>console.log('page-ready:%s')</script>
</body>
</html>`, marker, marker, marker)
}

func handleNATSExample(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 nats-example", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host")
	role := fs.String("role", defaultRole, "Chrome role")
	_ = fs.Parse(args)
	fmt.Println(NATSExample(strings.TrimSpace(*host), strings.TrimSpace(*role)))
	return nil
}

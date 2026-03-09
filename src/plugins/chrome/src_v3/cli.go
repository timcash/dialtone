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
)

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
	case "status", "instances":
		return handleRequestCommand("status", args[1:])
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
	logs.Info("  open [--host <host>] --url <url>")
	logs.Info("  goto [--host <host>] --url <url>")
	logs.Info("  get-url [--host <host>]")
	logs.Info("  tabs [--host <host>]")
	logs.Info("  tab-open [--host <host>] [--url <url>]")
	logs.Info("  tab-close [--host <host>] [--index <n>]")
	logs.Info("  close [--host <host>]")
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
	return buildBinaryFor(filepath.Join("..", "bin", binaryName(runtime.GOOS, runtime.GOARCH)), runtime.GOOS, runtime.GOARCH)
}

func handleDeploy(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 deploy", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host (optional; default local)")
	role := fs.String("role", defaultRole, "Chrome role")
	service := fs.Bool("service", false, "Start service after deploy")
	_ = fs.Parse(args)
	return deployTarget(strings.TrimSpace(*host), strings.TrimSpace(*role), *service)
}

func handleService(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 service", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host (optional; default local)")
	mode := fs.String("mode", "status", "start|stop|status")
	role := fs.String("role", defaultRole, "Chrome role")
	_ = fs.Parse(args)
	resp, err := serviceTarget(strings.TrimSpace(*host), strings.TrimSpace(*mode), strings.TrimSpace(*role))
	if err != nil {
		return err
	}
	if resp != nil {
		printResponse(resp)
	}
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
	resp, err := sendCommandByTarget(strings.TrimSpace(*host), req, autoStart)
	if err != nil {
		return err
	}
	printResponse(resp)
	return nil
}

func handleSmokeTest(args []string) error {
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	runArgs := append([]string{"run", "./plugins/chrome/src_v3/test/cmd/main.go"}, args...)
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
	resp, err := sendCommandByTarget(strings.TrimSpace(*host), commandRequest{
		Command:   command,
		Role:      strings.TrimSpace(*role),
		AriaLabel: strings.TrimSpace(*label),
		Value:     *value,
	}, true)
	if err != nil {
		return err
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
	resp, err := sendCommandByTarget(strings.TrimSpace(*host), commandRequest{
		Command:   "get-aria-attr",
		Role:      strings.TrimSpace(*role),
		AriaLabel: strings.TrimSpace(*label),
		Attr:      strings.TrimSpace(*attr),
	}, true)
	if err != nil {
		return err
	}
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
	resp, err := sendCommandByTarget(strings.TrimSpace(*host), commandRequest{
		Command:   "wait-log",
		Role:      strings.TrimSpace(*role),
		Contains:  strings.TrimSpace(*contains),
		TimeoutMS: *timeoutMS,
	}, true)
	if err != nil {
		return err
	}
	printResponse(resp)
	return nil
}

func handleSetHTMLCommand(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 set-html", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host (optional; default local)")
	role := fs.String("role", defaultRole, "Chrome role")
	value := fs.String("value", "", "HTML markup")
	_ = fs.Parse(args)
	resp, err := sendCommandByTarget(strings.TrimSpace(*host), commandRequest{
		Command: "set-html",
		Role:    strings.TrimSpace(*role),
		Value:   *value,
	}, true)
	if err != nil {
		return err
	}
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
	resp, err := sendCommandByTarget(strings.TrimSpace(*host), commandRequest{
		Command: "screenshot",
		Role:    strings.TrimSpace(*role),
	}, true)
	if err != nil {
		return err
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(resp.ScreenshotB64))
	if err != nil {
		return fmt.Errorf("decode screenshot: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(*outPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(*outPath, data, 0644); err != nil {
		return err
	}
	printResponse(resp)
	fmt.Printf("SCREENSHOT_SAVED %s\n", strings.TrimSpace(*outPath))
	return nil
}

func handleActionSmokeTest(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 test-actions", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host (optional; default local)")
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
		{Command: "screenshot", Role: *role},
	}
	for _, step := range steps {
		resp, err := sendCommandByTarget(strings.TrimSpace(*host), step, true)
		if err != nil {
			return fmt.Errorf("%s failed: %w", step.Command, err)
		}
		if step.Command == "screenshot" {
			data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(resp.ScreenshotB64))
			if err != nil {
				return fmt.Errorf("decode screenshot: %w", err)
			}
			if err := os.MkdirAll(filepath.Dir(*outPath), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(*outPath, data, 0644); err != nil {
				return err
			}
			fmt.Printf("SCREENSHOT_SAVED %s\n", strings.TrimSpace(*outPath))
		}
		printResponse(resp)
	}
	return nil
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

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
	logs.Info("  deploy --host <host> [--service]")
	logs.Info("  service --host <host> --mode start|stop|status")
	logs.Info("  status --host <host>")
	logs.Info("  open --host <host> --url <url>")
	logs.Info("  goto --host <host> --url <url>")
	logs.Info("  get-url --host <host>")
	logs.Info("  tabs --host <host>")
	logs.Info("  tab-open --host <host> [--url <url>]")
	logs.Info("  tab-close --host <host> [--index <n>]")
	logs.Info("  close --host <host>")
	logs.Info("  click-aria --host <host> --label <aria-label>")
	logs.Info("  type-aria --host <host> --label <aria-label> --value <text>")
	logs.Info("  set-html --host <host> --value <html>")
	logs.Info("  wait-log --host <host> --contains <text> [--timeout-ms 5000]")
	logs.Info("  console --host <host>")
	logs.Info("  screenshot --host <host> --out <png-path>")
	logs.Info("  nats-example --host <host> [--role <role>]")
	logs.Info("  test --host <host>")
	logs.Info("  test-actions --host <host>")
	logs.Info("  doctor --host <host>")
	logs.Info("  logs --host <host>")
	logs.Info("  reset --host <host>")
	logs.Info("  daemon --role dev --chrome-port 19464 --nats-port 19465")
	logs.Info("NATS example:")
	logs.Info("  %s", NATSExample("<host>", defaultRole))
}

func buildLocalBinary() error {
	return buildBinaryFor(filepath.Join("..", "bin", binaryName(runtime.GOOS, runtime.GOARCH)), runtime.GOOS, runtime.GOARCH)
}

func buildBinaryFor(outPath, goos, goarch string) error {
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}
	cmd := exec.Command(goBin, "build", "-o", outPath, "./plugins/chrome/scaffold/main.go")
	cmd.Dir = resolveSrcRoot()
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	logs.Info("chrome src_v3 build ok: %s", outPath)
	return nil
}

func handleDeploy(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 deploy", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host")
	role := fs.String("role", defaultRole, "Chrome role")
	service := fs.Bool("service", false, "Start service after deploy")
	_ = fs.Parse(args)
	if strings.TrimSpace(*host) == "" {
		return fmt.Errorf("deploy requires --host")
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(*host))
	if err != nil {
		return err
	}
	goos := mapNodeGOOS(node.OS)
	goarch := detectRemoteGOARCH(node)
	localBin := filepath.Join(resolveRepoRoot(), "bin", binaryName(goos, goarch))
	if err := buildBinaryFor(localBin, goos, goarch); err != nil {
		return err
	}
	remoteBin, err := remoteBinaryPath(node)
	if err != nil {
		return err
	}
	if err := sshv1.UploadNodeFile(node.Name, localBin, remoteBin+".upload", sshv1.CommandOptions{}); err != nil {
		return err
	}
	if strings.EqualFold(node.OS, "windows") {
		cmd := fmt.Sprintf(`$bin=%s; New-Item -ItemType Directory -Path ([IO.Path]::GetDirectoryName($bin)) -Force | Out-Null; if(Test-Path $bin){ Remove-Item -Force $bin }; Move-Item -Force %s $bin`, psQuote(remoteBin), psQuote(remoteBin+".upload"))
		if _, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{}); err != nil {
			return err
		}
	} else {
		cmd := fmt.Sprintf("mkdir -p %s && chmod +x %s && mv %s %s", shellQuote(filepath.Dir(remoteBin)), shellQuote(remoteBin+".upload"), shellQuote(remoteBin+".upload"), shellQuote(remoteBin))
		if _, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{}); err != nil {
			return err
		}
	}
	logs.Info("chrome src_v3 deployed to %s:%s", node.Name, remoteBin)
	if *service {
		return startRemoteService(node, strings.TrimSpace(*role))
	}
	return nil
}

func handleService(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 service", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host")
	mode := fs.String("mode", "status", "start|stop|status")
	role := fs.String("role", defaultRole, "Chrome role")
	_ = fs.Parse(args)
	if strings.TrimSpace(*host) == "" {
		return fmt.Errorf("service requires --host")
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(*host))
	if err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "start":
		return startRemoteService(node, strings.TrimSpace(*role))
	case "stop":
		return stopRemoteService(node)
	case "status":
		resp, err := sendRemoteCommand(node, commandRequest{Command: "status", Role: strings.TrimSpace(*role)})
		if err != nil {
			return err
		}
		printResponse(resp)
		return nil
	default:
		return fmt.Errorf("unsupported service mode: %s", *mode)
	}
}

func handleRequestCommand(command string, args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 "+command, flag.ExitOnError)
	host := fs.String("host", "", "Mesh host")
	role := fs.String("role", defaultRole, "Chrome role")
	rawURL := fs.String("url", "", "URL")
	index := fs.Int("index", -1, "Tab index")
	_ = fs.Parse(args)
	if strings.TrimSpace(*host) == "" {
		return fmt.Errorf("%s requires --host", command)
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(*host))
	if err != nil {
		return err
	}
	req := commandRequest{
		Command: command,
		Role:    strings.TrimSpace(*role),
		URL:     normalizeURL(strings.TrimSpace(*rawURL)),
		Index:   *index,
	}
	resp, err := sendRemoteCommand(node, req)
	if err != nil {
		return err
	}
	printResponse(resp)
	return nil
}

func handleSmokeTest(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 test", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host")
	role := fs.String("role", defaultRole, "Chrome role")
	_ = fs.Parse(args)
	if strings.TrimSpace(*host) == "" {
		return fmt.Errorf("test requires --host")
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(*host))
	if err != nil {
		return err
	}
	steps := []commandRequest{
		{Command: "status", Role: *role},
		{Command: "open", Role: *role, URL: "https://example.com/?dialtone=open"},
		{Command: "get-url", Role: *role},
		{Command: "tab-open", Role: *role, URL: "https://example.com/?dialtone=tab-open"},
		{Command: "tabs", Role: *role},
		{Command: "tab-close", Role: *role},
		{Command: "close", Role: *role},
	}
	for _, step := range steps {
		resp, err := sendRemoteCommand(node, step)
		if err != nil {
			return fmt.Errorf("%s failed: %w", step.Command, err)
		}
		printResponse(resp)
	}
	return nil
}

func handleAriaCommand(command string, args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 "+command, flag.ExitOnError)
	host := fs.String("host", "", "Mesh host")
	role := fs.String("role", defaultRole, "Chrome role")
	label := fs.String("label", "", "ARIA label")
	value := fs.String("value", "", "Text to type")
	_ = fs.Parse(args)
	if strings.TrimSpace(*host) == "" {
		return fmt.Errorf("%s requires --host", command)
	}
	if strings.TrimSpace(*label) == "" {
		return fmt.Errorf("%s requires --label", command)
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(*host))
	if err != nil {
		return err
	}
	resp, err := sendRemoteCommand(node, commandRequest{
		Command:   command,
		Role:      strings.TrimSpace(*role),
		AriaLabel: strings.TrimSpace(*label),
		Value:     *value,
	})
	if err != nil {
		return err
	}
	printResponse(resp)
	return nil
}

func handleWaitLogCommand(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 wait-log", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host")
	role := fs.String("role", defaultRole, "Chrome role")
	contains := fs.String("contains", "", "Substring to wait for")
	timeoutMS := fs.Int("timeout-ms", 5000, "Timeout in milliseconds")
	_ = fs.Parse(args)
	if strings.TrimSpace(*host) == "" {
		return fmt.Errorf("wait-log requires --host")
	}
	if strings.TrimSpace(*contains) == "" {
		return fmt.Errorf("wait-log requires --contains")
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(*host))
	if err != nil {
		return err
	}
	resp, err := sendRemoteCommand(node, commandRequest{
		Command:   "wait-log",
		Role:      strings.TrimSpace(*role),
		Contains:  strings.TrimSpace(*contains),
		TimeoutMS: *timeoutMS,
	})
	if err != nil {
		return err
	}
	printResponse(resp)
	return nil
}

func handleSetHTMLCommand(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 set-html", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host")
	role := fs.String("role", defaultRole, "Chrome role")
	value := fs.String("value", "", "HTML markup")
	_ = fs.Parse(args)
	if strings.TrimSpace(*host) == "" {
		return fmt.Errorf("set-html requires --host")
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(*host))
	if err != nil {
		return err
	}
	resp, err := sendRemoteCommand(node, commandRequest{
		Command: "set-html",
		Role:    strings.TrimSpace(*role),
		Value:   *value,
	})
	if err != nil {
		return err
	}
	printResponse(resp)
	return nil
}

func handleScreenshotCommand(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 screenshot", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host")
	role := fs.String("role", defaultRole, "Chrome role")
	outPath := fs.String("out", "", "Local PNG output path")
	_ = fs.Parse(args)
	if strings.TrimSpace(*host) == "" {
		return fmt.Errorf("screenshot requires --host")
	}
	if strings.TrimSpace(*outPath) == "" {
		return fmt.Errorf("screenshot requires --out")
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(*host))
	if err != nil {
		return err
	}
	resp, err := sendRemoteCommand(node, commandRequest{
		Command: "screenshot",
		Role:    strings.TrimSpace(*role),
	})
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
	host := fs.String("host", "", "Mesh host")
	role := fs.String("role", defaultRole, "Chrome role")
	outPath := fs.String("out", filepath.Join(os.TempDir(), "chrome-src-v3-actions.png"), "Local PNG output path")
	_ = fs.Parse(args)
	if strings.TrimSpace(*host) == "" {
		return fmt.Errorf("test-actions requires --host")
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(*host))
	if err != nil {
		return err
	}
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
		resp, err := sendRemoteCommand(node, step)
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

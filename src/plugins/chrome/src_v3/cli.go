package src_v3

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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
	case "test":
		return handleSmokeTest(args[1:])
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
	logs.Info("  test --host <host>")
	logs.Info("  doctor --host <host>")
	logs.Info("  logs --host <host>")
	logs.Info("  reset --host <host>")
	logs.Info("  daemon --role dev --chrome-port 19464 --nats-port 19465")
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

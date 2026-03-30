package uiv1

import (
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	uipaths "dialtone/dev/plugins/ui/src_v1/go"
)

func Run(args []string) error {
	logs.SetOutput(os.Stdout)
	if len(args) < 1 {
		printUsage()
		return fmt.Errorf("no command provided")
	}

	normalized, warnedOldOrder, err := normalizeUIArgs(args)
	if err != nil {
		printUsage()
		return err
	}
	if warnedOldOrder {
		logs.Warn("old ui CLI order is deprecated. Use: ./dialtone.sh ui src_v1 <command> [args]")
	}
	args = normalized

	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "help", "--help", "-h":
		printUsage()
		return nil
	case "dev":
		return runUIDev(cmdArgs)
	case "build":
		return runUIFixtureScript("build", cmdArgs)
	case "lint":
		return runUIFixtureScript("lint", cmdArgs)
	case "format", "fmt":
		return runUIFixtureScript("fmt", cmdArgs)
	case "fmt-check":
		return runUIFixtureScript("fmt:check", cmdArgs)
	case "install":
		return runUIFixtureScript("install", cmdArgs)
	case "mock-data":
		RunMockData(cmdArgs)
		return nil
	case "test":
		return runUITests(cmdArgs)
	case "kill":
		runKill()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown ui command: %s", command)
	}
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh ui src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  dev       Run fixture dev server + attachable browser session")
	logs.Raw("  install   Install fixture dependencies (bun install)")
	logs.Raw("  format    Format fixture UI sources (bun run fmt)")
	logs.Raw("  lint      Lint/type-check fixture UI (bun run lint)")
	logs.Raw("  build     Build fixture UI dist (bun run build)")
	logs.Raw("  test      Run ui src_v1 test suite (supports --attach <mesh-node>)")
	logs.Raw("  mock-data Start mock data server for testing")
	logs.Raw("  kill      Kill running UI processes (dev, mock-data)")
}

func runKill() {
	logs.Info("ui src_v1 kill: stopping local UI helper processes")
	exec.Command("pkill", "-f", "vite").Run()
	exec.Command("pkill", "-f", "dialtone ui mock-data").Run()
	exec.Command("pkill", "-f", "dialtone ui dev").Run()

	ports := []string{"4222", "4223", "8080", "5173", "5174", "5177"}
	for _, p := range ports {
		exec.Command("fuser", "-k", "-n", "tcp", p).Run()
	}
	logs.Info("ui src_v1 kill: complete")
}

func runUIFixtureScript(script string, args []string) error {
	paths, err := uipaths.ResolvePaths("")
	if err != nil {
		return err
	}
	bunBin := strings.TrimSpace(paths.Runtime.BunBin)
	if bunBin == "" {
		bunBin = configv1.ManagedBunBinPath(configv1.DefaultDialtoneEnv())
	}
	if _, err := os.Stat(bunBin); os.IsNotExist(err) {
		bunBin = "bun"
	}

	var cmdArgsRun []string
	if script == "install" {
		cmdArgsRun = []string{"install"}
	} else {
		cmdArgsRun = []string{"run", script}
	}
	cmdArgsRun = append(cmdArgsRun, args...)

	logs.Info("ui src_v1 %s: %s %s", script, bunBin, strings.Join(cmdArgsRun, " "))
	cmd := exec.Command(bunBin, cmdArgsRun...)
	cmd.Dir = paths.FixtureApp
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	return cmd.Run()
}

func normalizeUIArgs(args []string) ([]string, bool, error) {
	if len(args) == 0 {
		return nil, false, fmt.Errorf("no command provided")
	}
	if isHelp(args[0]) {
		return []string{"help"}, false, nil
	}
	if strings.HasPrefix(args[0], "src_v") {
		if args[0] != "src_v1" {
			return nil, false, fmt.Errorf("unsupported version %s", args[0])
		}
		if len(args) < 2 {
			return nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh ui src_v1 <command> [args])")
		}
		return append([]string{args[1]}, args[2:]...), false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		if args[1] != "src_v1" {
			return nil, false, fmt.Errorf("unsupported version %s", args[1])
		}
		return append([]string{args[0]}, args[2:]...), true, nil
	}
	return nil, false, fmt.Errorf("expected version as first ui argument (usage: ./dialtone.sh ui src_v1 <command> [args])")
}

func isHelp(s string) bool {
	switch strings.TrimSpace(s) {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
}

func runUITests(extraArgs []string) error {
	paths, err := uipaths.ResolvePaths("")
	if err != nil {
		return err
	}
	goBin := strings.TrimSpace(paths.Runtime.GoBin)
	if goBin == "" {
		goBin = "go"
	}
	runArgs := []string{"run", "./plugins/ui/src_v1/test/cmd/main.go"}
	runArgs = append(runArgs, extraArgs...)
	cmd := exec.Command(goBin, runArgs...)
	cmd.Dir = paths.Runtime.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	return cmd.Run()
}

func runUIDev(args []string) error {
	paths, err := uipaths.ResolvePaths("")
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("ui dev", flag.ContinueOnError)
	fs.SetOutput(nil)
	port := fs.Int("port", 5177, "Dev server port")
	host := fs.String("host", "0.0.0.0", "Vite bind host")
	browserNode := fs.String("browser-node", "", "Optional mesh node for headed browser session (example: legion)")
	publicURL := fs.String("public-url", "", "Public URL that remote browser should open")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	devURL := strings.TrimSpace(*publicURL)
	node := strings.TrimSpace(*browserNode)
	if node == "" {
		node = defaultDevBrowserNode()
	}
	if node != "" && devURL == "" {
		u, err := inferPublicDevURL(*port)
		if err != nil {
			return err
		}
		devURL = u
	}
	testv1.SetRuntimeConfig(testv1.RuntimeConfig{BrowserNode: node})
	if node != "" {
		logs.Info("ui dev remote browser node=%s url=%s", node, devURL)
	}

	return testv1.RunDev(testv1.DevOptions{
		RepoRoot:          paths.Runtime.RepoRoot,
		PluginDir:         paths.PluginVersionRoot,
		UIDir:             paths.FixtureApp,
		DevPort:           *port,
		DevHost:           strings.TrimSpace(*host),
		DevPublicURL:      devURL,
		Role:              "ui-dev",
		BrowserMetaPath:   filepath.Join(paths.PluginVersionRoot, "dev.browser.json"),
		BrowserModeEnvVar: "UI_DEV_BROWSER_MODE",
		NATSURL:           "nats://127.0.0.1:4222",
		NATSSubject:       "logs.dev.ui.src-v1",
	})
}

func defaultDevBrowserNode() string {
	return testv1.ResolveDefaultAttachNode(configv1.LookupEnvString("DIALTONE_TEST_BROWSER_NODE"))
}

func inferPublicDevURL(port int) (string, error) {
	wsl, err := sshv1.ResolveMeshNode("wsl")
	if err != nil {
		return "", fmt.Errorf("resolve wsl mesh node for public url: %w", err)
	}
	host := strings.TrimSpace(wsl.Host)
	if host == "" {
		return "", fmt.Errorf("wsl mesh host is empty")
	}
	u := &url.URL{Scheme: "http", Host: fmt.Sprintf("%s:%d", host, port)}
	return u.String(), nil
}

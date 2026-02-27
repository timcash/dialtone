package earth

import (
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
)

func Run(args []string) error {
	if len(args) == 0 {
		PrintUsage()
		return nil
	}
	switch strings.TrimSpace(args[0]) {
	case "help", "--help", "-h":
		PrintUsage()
		return nil
	case "install":
		return runInstall(args[1:])
	case "dev":
		return runDev(args[1:])
	case "build":
		return runBuild(args[1:])
	case "serve":
		return runServe(args[1:])
	case "go-build":
		return runGoBuild(args[1:])
	default:
		PrintUsage()
		return fmt.Errorf("unknown earth command: %s", args[0])
	}
}

func PrintUsage() {
	logs.Raw("Usage: ./dialtone.sh earth src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  install                 Install UI dependencies (bun install)")
	logs.Raw("  dev [--port P] [--browser-node N] [--public-url U] [--host H]")
	logs.Raw("  build                   Build UI dist")
	logs.Raw("  serve [--addr A]        Serve UI dist through Go server")
	logs.Raw("  go-build                Build earth server binary to repo bin/")
	logs.Raw("  test [--attach N]       Run earth plugin test suite (optional remote attach node)")
}

func runInstall(_ []string) error {
	paths, err := ResolvePaths("")
	if err != nil {
		return err
	}
	return runDialtone(paths.Runtime.RepoRoot, "bun", "src_v1", "exec", "--cwd", paths.Preset.UI, "install")
}

func runDev(args []string) error {
	paths, err := ResolvePaths("")
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("earth dev", flag.ContinueOnError)
	fs.SetOutput(nil)
	port := fs.Int("port", 5181, "Dev server port")
	host := fs.String("host", "0.0.0.0", "Vite bind host")
	browserNode := fs.String("browser-node", "", "Optional mesh node for headed browser session (example: chroma)")
	publicURL := fs.String("public-url", "", "Public URL that remote browser should open")
	if err := fs.Parse(args); err != nil {
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
	if node != "" {
		_ = os.Setenv("DIALTONE_TEST_BROWSER_NODE", node)
		logs.Info("earth dev remote browser node=%s url=%s", node, devURL)
	} else {
		_ = os.Unsetenv("DIALTONE_TEST_BROWSER_NODE")
	}
	return testv1.RunDev(testv1.DevOptions{
		RepoRoot:          paths.Runtime.RepoRoot,
		PluginDir:         paths.Preset.PluginVersionRoot,
		UIDir:             paths.Preset.UI,
		DevPort:           *port,
		DevHost:           strings.TrimSpace(*host),
		DevPublicURL:      devURL,
		Role:              "earth-dev",
		BrowserMetaPath:   filepath.Join(paths.Preset.PluginVersionRoot, "dev.browser.json"),
		BrowserModeEnvVar: "EARTH_DEV_BROWSER_MODE",
		NATSURL:           "nats://127.0.0.1:4222",
		NATSSubject:       "logs.dev.earth-src-v1",
	})
}

func defaultDevBrowserNode() string {
	if envNode := strings.TrimSpace(os.Getenv("DIALTONE_TEST_BROWSER_NODE")); envNode != "" {
		return envNode
	}
	// WSL typically has no native Chrome; prefer known remote dev browser node.
	if isWSL() {
		return "chroma"
	}
	return ""
}

func isWSL() bool {
	if strings.Contains(strings.ToLower(strings.TrimSpace(os.Getenv("WSL_DISTRO_NAME"))), "ubuntu") ||
		strings.TrimSpace(os.Getenv("WSL_DISTRO_NAME")) != "" {
		return true
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

func runBuild(_ []string) error {
	paths, err := ResolvePaths("")
	if err != nil {
		return err
	}
	return runDialtone(paths.Runtime.RepoRoot, "bun", "src_v1", "exec", "--cwd", paths.Preset.UI, "run", "build")
}

func runServe(args []string) error {
	paths, err := ResolvePaths("")
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("earth serve", flag.ContinueOnError)
	fs.SetOutput(nil)
	addr := fs.String("addr", ":8891", "Bind address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	cmd := exec.Command(goBin, "run", "./plugins/earth/src_v1/cmd/server/main.go", "--addr", *addr)
	cmd.Dir = paths.Runtime.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runGoBuild(_ []string) error {
	paths, err := ResolvePaths("")
	if err != nil {
		return err
	}
	if strings.TrimSpace(paths.Runtime.RepoRoot) == "" {
		return errors.New("repo root unavailable")
	}
	out := filepath.Join(paths.Runtime.RepoRoot, "bin", "dialtone_earth_v1")
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	cmd := exec.Command(goBin, "build", "-o", out, "./plugins/earth/src_v1/cmd/server/main.go")
	cmd.Dir = paths.Runtime.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	logs.Info("built %s", out)
	return nil
}

func runDialtone(repoRoot string, args ...string) error {
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), args...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func inferPublicDevURL(port int) (string, error) {
	wsl, err := sshv1.ResolveMeshNode("wsl")
	if err != nil {
		return "", fmt.Errorf("resolve wsl mesh node for public url: %w", err)
	}
	host := strings.TrimSpace(wsl.Host)
	if host == "" {
		return "", errors.New("wsl mesh host is empty")
	}
	u := &url.URL{Scheme: "http", Host: fmt.Sprintf("%s:%d", host, port)}
	return u.String(), nil
}

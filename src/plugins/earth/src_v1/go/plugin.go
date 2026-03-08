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
	case "open-dev":
		return runOpenDev(args[1:])
	case "build":
		return runBuild(args[1:])
	case "serve":
		return runServe(args[1:])
	case "go-build":
		return runGoBuild(args[1:])
	case "format":
		return runFormat(args[1:])
	case "test":
		return runTest(args[1:])
	case "download-dev":
		return runDownloadDev(args[1:])
	default:
		PrintUsage()
		return fmt.Errorf("unknown earth command: %s", args[0])
	}
}

func PrintUsage() {
	logs.Raw("Usage: ./dialtone.sh earth src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  install [flags]         Install UI dependencies, then launch Vite dev (HMR) unless --no-dev")
	logs.Raw("  dev [--port P] [--browser-node N] [--public-url U] [--host H]")
	logs.Raw("  open-dev [--hosts H] [--port P] [--public-url U] [--role R]")
	logs.Raw("  build                   Build UI dist")
	logs.Raw("  serve [--addr A]        Serve UI dist through Go server")
	logs.Raw("  go-build                Build earth server binary to repo bin/")
	logs.Raw("  format                  Format Go and UI source code")
	logs.Raw("  test [--attach N]       Run earth plugin test suite")
	logs.Raw("  download-dev [flags]    Clone/pull repo then run earth ui vite dev")
}

func runInstall(args []string) error {
	paths, err := ResolvePaths("")
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("earth install", flag.ContinueOnError)
	fs.SetOutput(nil)
	noDev := fs.Bool("no-dev", false, "Only install dependencies; do not launch Vite dev")
	port := fs.Int("port", 5181, "Dev server port")
	host := fs.String("host", "0.0.0.0", "Vite bind host")
	browserNode := fs.String("browser-node", "", "Optional mesh node for headed browser session")
	publicURL := fs.String("public-url", "", "Public URL that remote browser should open")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := runDialtone(paths.Runtime.RepoRoot, "bun", "src_v1", "exec", "--cwd", paths.Preset.UI, "install"); err != nil {
		return err
	}
	if *noDev {
		logs.Info("earth install complete (dev launch skipped via --no-dev)")
		return nil
	}
	devArgs := []string{
		"--port", fmt.Sprintf("%d", *port),
		"--host", strings.TrimSpace(*host),
	}
	if strings.TrimSpace(*browserNode) != "" {
		devArgs = append(devArgs, "--browser-node", strings.TrimSpace(*browserNode))
	}
	if strings.TrimSpace(*publicURL) != "" {
		devArgs = append(devArgs, "--public-url", strings.TrimSpace(*publicURL))
	}
	return runDev(devArgs)
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

func runOpenDev(args []string) error {
	paths, err := ResolvePaths("")
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("earth open-dev", flag.ContinueOnError)
	fs.SetOutput(nil)
	hosts := fs.String("hosts", "all", "Chrome target hosts CSV or 'all' (example: darkmac,legion,gold)")
	port := fs.Int("port", 5181, "Vite dev server port")
	publicURL := fs.String("public-url", "", "Explicit URL (default: http://127.0.0.1:<port> on each host)")
	role := fs.String("role", "dev", "Chrome role to reuse/start")
	if err := fs.Parse(args); err != nil {
		return err
	}
	devURL := strings.TrimSpace(*publicURL)
	if devURL == "" {
		devURL = fmt.Sprintf("http://127.0.0.1:%d", *port)
	}
	logs.Info("earth open-dev hosts=%s url=%s role=%s", strings.TrimSpace(*hosts), devURL, strings.TrimSpace(*role))
	return runDialtone(paths.Runtime.RepoRoot,
		"chrome", "src_v3", "open",
		"--host", strings.TrimSpace(*hosts),
		"--role", strings.TrimSpace(*role),
		"--url", devURL,
	)
}

func defaultDevBrowserNode() string {
	if envNode := strings.TrimSpace(os.Getenv("DIALTONE_TEST_BROWSER_NODE")); envNode != "" {
		return envNode
	}
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

func runFormat(args []string) error {
	paths, err := ResolvePaths("")
	if err != nil {
		return err
	}
	logs.Info("earth src_v1 formatting Go code...")
	if err := runCmd(paths.Preset.PluginVersionRoot, "go", "fmt", "./..."); err != nil {
		return err
	}
	logs.Info("earth src_v1 formatting UI code...")
	return runDialtone(paths.Runtime.RepoRoot, "bun", "src_v1", "exec", "--cwd", paths.Preset.UI, "run", "fmt")
}

func runTest(args []string) error {
	paths, err := ResolvePaths("")
	if err != nil {
		return err
	}
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	runArgs := []string{"run", "./plugins/earth/src_v1/test/cmd/main.go"}
	runArgs = append(runArgs, args...)
	cmd := exec.Command(goBin, runArgs...)
	cmd.Dir = paths.Runtime.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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

func runDownloadDev(args []string) error {
	paths, err := ResolvePaths("")
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("earth download-dev", flag.ContinueOnError)
	fs.SetOutput(nil)
	repoURL := fs.String("repo", "https://github.com/timcash/dialtone.git", "Git repo URL to clone/pull")
	branch := fs.String("branch", "main", "Git branch")
	dest := fs.String("dest", filepath.Join(paths.Runtime.RepoRoot, ".dialtone", "plugins", "earth-dev"), "Destination directory")
	pluginPath := fs.String("plugin-path", "src/plugins/earth/src_v1/ui", "Path to earth ui dir relative to repo root")
	host := fs.String("host", "0.0.0.0", "Vite bind host")
	port := fs.Int("port", 5181, "Vite dev server port")
	if err := fs.Parse(args); err != nil {
		return err
	}
	targetDir := strings.TrimSpace(*dest)
	if targetDir == "" {
		return errors.New("download-dev requires --dest")
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create destination failed: %w", err)
	}
	repoRoot, err := ensureRepoCheckout(strings.TrimSpace(*repoURL), strings.TrimSpace(*branch), targetDir)
	if err != nil {
		return err
	}
	uiDir := filepath.Join(repoRoot, filepath.FromSlash(strings.TrimSpace(*pluginPath)))
	if info, statErr := os.Stat(uiDir); statErr != nil || !info.IsDir() {
		return fmt.Errorf("plugin ui directory not found: %s", uiDir)
	}
	logs.Info("earth download-dev repo=%s branch=%s ui=%s", strings.TrimSpace(*repoURL), strings.TrimSpace(*branch), uiDir)
	if err := runDialtone(paths.Runtime.RepoRoot, "bun", "src_v1", "exec", "--cwd", uiDir, "install"); err != nil {
		return err
	}
	return runDialtone(paths.Runtime.RepoRoot, "bun", "src_v1", "exec", "--cwd", uiDir, "run", "dev", "--host", strings.TrimSpace(*host), "--port", fmt.Sprintf("%d", *port))
}

func ensureRepoCheckout(repoURL, branch, dest string) (string, error) {
	repoURL = strings.TrimSpace(repoURL)
	branch = strings.TrimSpace(branch)
	dest = strings.TrimSpace(dest)
	if repoURL == "" {
		return "", errors.New("download-dev requires --repo")
	}
	if branch == "" {
		branch = "main"
	}
	gitDir := filepath.Join(dest, ".git")
	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		if err := runCmd(dest, "git", "fetch", "--all", "--tags", "--prune"); err != nil {
			return "", err
		}
		if err := runCmd(dest, "git", "checkout", branch); err != nil {
			return "", err
		}
		if err := runCmd(dest, "git", "pull", "--ff-only", "origin", branch); err != nil {
			return "", err
		}
		return dest, nil
	}
	if entries, err := os.ReadDir(dest); err == nil && len(entries) > 0 {
		return "", fmt.Errorf("destination is not empty and not a git checkout: %s", dest)
	}
	if err := runCmd("", "git", "clone", "--branch", branch, "--single-branch", repoURL, dest); err != nil {
		return "", err
	}
	return dest, nil
}

func runCmd(cwd string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if strings.TrimSpace(cwd) != "" {
		cmd.Dir = cwd
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
	}
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

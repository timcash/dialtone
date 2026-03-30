package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	cadv1 "dialtone/dev/plugins/cad/src_v1/go"
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	pixiv1 "dialtone/dev/plugins/pixi/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)

	version, command, rest, warnedOldOrder, err := parseArgs(os.Args[1:])
	if err != nil {
		logs.Error("%v", err)
		printUsage()
		os.Exit(1)
	}
	if warnedOldOrder {
		logs.Warn("old cad CLI order is deprecated. Use: ./dialtone.sh cad src_v1 <command> [args]")
	}

	switch version {
	case "src_v1":
		switch command {
		case "help", "-h", "--help":
			printUsage()
			return
		case "install":
			if err := runInstall(); err != nil {
				logs.Error("cad src_v1 install failed: %v", err)
				os.Exit(1)
			}
			return
		case "test":
			if err := runTests(rest); err != nil {
				logs.Error("cad src_v1 tests failed: %v", err)
				os.Exit(1)
			}
			return
		case "format":
			if err := runFormat(); err != nil {
				logs.Error("cad src_v1 format failed: %v", err)
				os.Exit(1)
			}
			return
		case "lint":
			if err := runLint(); err != nil {
				logs.Error("cad src_v1 lint failed: %v", err)
				os.Exit(1)
			}
			return
		case "build":
			if err := runBuild(); err != nil {
				logs.Error("cad src_v1 build failed: %v", err)
				os.Exit(1)
			}
			return
		case "dev":
			if err := runDev(rest); err != nil {
				logs.Error("cad src_v1 dev failed: %v", err)
				os.Exit(1)
			}
			return
		}
		if err := cadv1.Run(command, rest); err != nil {
			logs.Error("%v", err)
			os.Exit(1)
		}
	default:
		logs.Error("unsupported version %s", version)
		os.Exit(1)
	}
}

func parseArgs(args []string) (version, command string, rest []string, warnedOldOrder bool, err error) {
	if len(args) == 0 {
		return "src_v1", "help", nil, false, nil
	}
	if isHelp(args[0]) {
		return "src_v1", "help", nil, false, nil
	}
	if strings.HasPrefix(args[0], "src_v") {
		if len(args) < 2 {
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh cad src_v1 <command> [args])")
		}
		return args[0], args[1], args[2:], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], args[0], args[2:], true, nil
	}
	return "", "", nil, false, fmt.Errorf("expected version as first cad argument (for example: ./dialtone.sh cad src_v1 serve)")
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh cad src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  serve [--port <n>]   Start the CAD backend server")
	logs.Raw("  server [--port <n>]  Alias for serve")
	logs.Raw("  status [--port <n>]  Check local CAD server health")
	logs.Raw("  stop [--port <n>]    Stop the tracked local CAD server")
	logs.Raw("  dev [--port <n>] [--backend-port <n>] [--host <host>] [--browser-node <node>] [--public-url <url>]")
	logs.Raw("  install              Verify/install CAD backend and UI dependencies")
	logs.Raw("  build                Build the CAD UI assets")
	logs.Raw("  format               Format Go and UI sources")
	logs.Raw("  lint                 Run Go and UI lint checks")
	logs.Raw("  test                 Run cad src_v1 test suite")
	logs.Raw("  help                 Show this help")
}

func runInstall() error {
	paths, err := cadv1.ResolvePaths("", "src_v1")
	if err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: cad install: verifying plugin layout")
	if err := cadv1.VerifyInstallLayout(paths); err != nil {
		return err
	}

	pixiBin, err := pixiv1.EnsureManaged(paths.Runtime)
	if err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: cad install: using pixi at %s", pixiBin)
	logs.Info("DIALTONE_INDEX: cad install: ensuring backend environment")
	if err := runCmd(paths.BackendDir, pixiBin, "install"); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: cad install: verifying backend imports")
	if err := runCmd(paths.BackendDir, pixiBin, "run", "python", "-c", "import cadquery, fastapi, uvicorn; print('cad-backend-deps-ok')"); err != nil {
		return err
	}

	bunBin, err := cadv1.ResolveBunBinary(paths)
	if err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: cad install: using bun at %s", bunBin)
	logs.Info("DIALTONE_INDEX: cad install: installing ui dependencies")
	if err := runCmd(paths.UIDir, bunBin, "install"); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(paths.UIDir, "node_modules")); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("cad install: ui dependencies missing after bun install")
		}
		return fmt.Errorf("cad install: unable to verify ui dependencies: %w", err)
	}
	logs.Info("DIALTONE_INDEX: cad install: dependencies ready")
	return nil
}

func runTests(args []string) error {
	paths, err := cadv1.ResolvePaths("", "src_v1")
	if err != nil {
		return err
	}
	cmdArgs := append([]string{"run", "./plugins/cad/src_v1/test/cmd/main.go"}, args...)
	cmd := exec.Command(resolveGoBin(paths), cmdArgs...)
	cmd.Dir = paths.Runtime.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runFormat() error {
	paths, err := cadv1.ResolvePaths("", "src_v1")
	if err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: cad format: formatting go sources")
	if err := runCmd(paths.Preset.PluginVersionRoot, "gofmt", "-w",
		filepath.Join(paths.Preset.Go, "cad.go"),
		filepath.Join(paths.Preset.Go, "paths.go"),
		filepath.Join(paths.Preset.Go, "plugin.go"),
		filepath.Join(paths.Preset.TestCmd, "main.go"),
		filepath.Join(paths.Preset.Test, "01_self_check", "suite.go"),
		filepath.Join(paths.Preset.Test, "02_browser_smoke", "suite.go"),
	); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: cad format: formatting ui sources")
	if err := runBun(paths, "run", "format"); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: cad format: completed")
	return nil
}

func runLint() error {
	paths, err := cadv1.ResolvePaths("", "src_v1")
	if err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: cad lint: ensuring ui dependencies")
	if err := ensureUIDeps(paths); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: cad lint: vetting go sources")
	if err := runCmd(paths.Runtime.SrcRoot, resolveGoBin(paths), "vet", "./plugins/cad/..."); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: cad lint: checking ui sources")
	if err := runBun(paths, "run", "lint"); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: cad lint: completed")
	return nil
}

func runBuild() error {
	paths, err := cadv1.ResolvePaths("", "src_v1")
	if err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: cad build: installing ui dependencies")
	if err := runBun(paths, "install"); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: cad build: building ui dist")
	if err := runBun(paths, "run", "build"); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: cad build: ui dist ready")
	return nil
}

func runDev(args []string) error {
	paths, err := cadv1.ResolvePaths("", "src_v1")
	if err != nil {
		return err
	}

	fs := flag.NewFlagSet("cad-dev", flag.ContinueOnError)
	port := fs.Int("port", 3012, "Vite dev server port")
	host := fs.String("host", "0.0.0.0", "Vite dev server host")
	backendPort := fs.Int("backend-port", 8081, "CAD backend port")
	browserNode := fs.String("browser-node", "", "Optional mesh node for headed browser session (use none/off/local to disable)")
	publicURL := fs.String("public-url", "", "URL a remote browser should open")
	if err := fs.Parse(args); err != nil {
		return err
	}

	logs.Info("DIALTONE_INDEX: cad dev: ensuring ui dependencies")
	if err := ensureUIDeps(paths); err != nil {
		return err
	}

	backendURL := fmt.Sprintf("http://127.0.0.1:%d", *backendPort)
	ok, err := devBackendHealthy(*backendPort)
	if err != nil {
		return err
	}
	if ok {
		logs.Info("DIALTONE_INDEX: cad dev: reusing backend on 127.0.0.1:%d", *backendPort)
	} else {
		logs.Info("DIALTONE_INDEX: cad dev: starting backend on 127.0.0.1:%d", *backendPort)
		if err := startDevBackend(paths, *backendPort); err != nil {
			return err
		}
		logs.Info("DIALTONE_INDEX: cad dev: backend ready on 127.0.0.1:%d", *backendPort)
	}

	localURL := fmt.Sprintf("http://127.0.0.1:%d", *port)
	devURL := strings.TrimSpace(*publicURL)
	if devURL == "" {
		devURL = localURL
	}
	node := strings.TrimSpace(*browserNode)
	if resolved, disabled := testv1.ResolveConfiguredAttachNode(node); disabled {
		node = ""
	} else if resolved != "" {
		node = resolved
	}
	if node == "" {
		testv1.SetRuntimeConfig(testv1.RuntimeConfig{})
	} else {
		testv1.SetRuntimeConfig(testv1.RuntimeConfig{
			BrowserNode:       node,
			RemoteBrowserRole: "cad-dev",
		})
		logs.Info("DIALTONE_INDEX: cad dev: browser node=%s url=%s", node, devURL)
	}

	prevProxy := os.Getenv("VITE_PROXY_TARGET")
	defer restoreEnv("VITE_PROXY_TARGET", prevProxy)
	_ = os.Setenv("VITE_PROXY_TARGET", backendURL)

	logs.Info("DIALTONE_INDEX: cad dev: starting vite on %s", localURL)
	return testv1.RunDev(testv1.DevOptions{
		RepoRoot:          paths.Runtime.RepoRoot,
		PluginDir:         paths.Preset.PluginVersionRoot,
		UIDir:             paths.UIDir,
		DevPort:           *port,
		DevHost:           strings.TrimSpace(*host),
		DevPublicURL:      devURL,
		Role:              "cad-dev",
		DisableBrowser:    node == "",
		BrowserMetaPath:   filepath.Join(paths.Preset.PluginVersionRoot, "dev.browser.json"),
		NATSURL:           resolveDevNATSURL(),
		NATSSubject:       "logs.dev.cad.src-v1",
	})
}

func ensureUIDeps(paths cadv1.Paths) error {
	if _, err := os.Stat(filepath.Join(paths.UIDir, "node_modules")); err == nil {
		return nil
	}
	return runBun(paths, "install")
}

func startDevBackend(paths cadv1.Paths, port int) error {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: cadv1.NewHandler(paths),
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logs.Error("cad dev backend failed: %v", err)
		}
	}()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		ok, _ := devBackendHealthy(port)
		if ok {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("cad dev backend did not become healthy on 127.0.0.1:%d", port)
}

func devBackendHealthy(port int) (bool, error) {
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

func resolveDevNATSURL() string {
	if raw := strings.TrimSpace(configv1.LookupEnvString("DIALTONE_REPL_NATS_URL")); raw != "" {
		return raw
	}
	return "nats://127.0.0.1:4222"
}

func restoreEnv(key, prev string) {
	if strings.TrimSpace(prev) == "" {
		_ = os.Unsetenv(key)
		return
	}
	_ = os.Setenv(key, prev)
}

func runBun(paths cadv1.Paths, args ...string) error {
	bunBin, err := cadv1.ResolveBunBinary(paths)
	if err != nil {
		return err
	}
	return runCmd(paths.UIDir, bunBin, args...)
}

func resolveGoBin(paths cadv1.Paths) string {
	if v := strings.TrimSpace(paths.Runtime.GoBin); v != "" {
		return v
	}
	return "go"
}

func runCmd(dir, bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

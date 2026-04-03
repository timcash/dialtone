package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	gacadv1 "dialtone/dev/plugins/ga_cad/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
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
		logs.Warn("old ga_cad CLI order is deprecated. Use: ./dialtone.sh ga_cad src_v1 <command> [args]")
	}

	switch version {
	case "src_v1":
		switch command {
		case "help", "-h", "--help":
			printUsage()
			return
		case "install":
			if err := runInstall(); err != nil {
				logs.Error("ga_cad src_v1 install failed: %v", err)
				os.Exit(1)
			}
			return
		case "format":
			if err := runFormat(); err != nil {
				logs.Error("ga_cad src_v1 format failed: %v", err)
				os.Exit(1)
			}
			return
		case "lint":
			if err := runLint(); err != nil {
				logs.Error("ga_cad src_v1 lint failed: %v", err)
				os.Exit(1)
			}
			return
		case "build":
			if err := runBuild(); err != nil {
				logs.Error("ga_cad src_v1 build failed: %v", err)
				os.Exit(1)
			}
			return
		case "dev":
			if err := runDev(rest); err != nil {
				logs.Error("ga_cad src_v1 dev failed: %v", err)
				os.Exit(1)
			}
			return
		}
		if err := gacadv1.Run(command, rest); err != nil {
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
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh ga_cad src_v1 <command> [args])")
		}
		return args[0], args[1], args[2:], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], args[0], args[2:], true, nil
	}
	return "", "", nil, false, fmt.Errorf("expected version as first ga_cad argument (for example: ./dialtone.sh ga_cad src_v1 serve)")
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh ga_cad src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  serve [--port <n>]   Start the GA CAD server")
	logs.Raw("  server [--port <n>]  Alias for serve")
	logs.Raw("  status [--port <n>]  Check local GA CAD server health")
	logs.Raw("  stop [--port <n>]    Stop the tracked local GA CAD server")
	logs.Raw("  install              Verify/install UI dependencies")
	logs.Raw("  build                Build the UI assets")
	logs.Raw("  format               Format Go and UI sources")
	logs.Raw("  lint                 Run Go and UI lint checks")
	logs.Raw("  dev [--port <n>] [--host <host>] [--browser-node <node>] [--public-url <url>]")
	logs.Raw("  help                 Show this help")
}

func runInstall() error {
	paths, err := gacadv1.ResolvePaths("", "src_v1")
	if err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: ga_cad install: verifying plugin layout")
	if err := gacadv1.VerifyInstallLayout(paths); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: ga_cad install: installing ui dependencies")
	if err := runBun(paths, "install"); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: ga_cad install: dependencies ready")
	return nil
}

func runFormat() error {
	paths, err := gacadv1.ResolvePaths("", "src_v1")
	if err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: ga_cad format: formatting go sources")
	if err := runCmd(paths.Preset.PluginVersionRoot, "gofmt", "-w",
		filepath.Join(paths.Preset.Go, "dependencies.go"),
		filepath.Join(paths.Preset.Go, "paths.go"),
		filepath.Join(paths.Preset.Go, "plugin.go"),
		filepath.Join(paths.Preset.PluginBase, "scaffold", "main.go"),
	); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: ga_cad format: formatting ui sources")
	if err := runBun(paths, "run", "format"); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: ga_cad format: completed")
	return nil
}

func runLint() error {
	paths, err := gacadv1.ResolvePaths("", "src_v1")
	if err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: ga_cad lint: ensuring ui dependencies")
	if err := ensureUIDeps(paths); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: ga_cad lint: vetting go sources")
	if err := runCmd(paths.Runtime.SrcRoot, resolveGoBin(paths.Runtime), "vet", "./plugins/ga_cad/..."); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: ga_cad lint: checking ui sources")
	if err := runBun(paths, "run", "lint"); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: ga_cad lint: completed")
	return nil
}

func runBuild() error {
	paths, err := gacadv1.ResolvePaths("", "src_v1")
	if err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: ga_cad build: installing ui dependencies")
	if err := ensureUIDeps(paths); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: ga_cad build: building ui dist")
	if err := runBun(paths, "run", "build"); err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: ga_cad build: ui dist ready")
	return nil
}

func runDev(args []string) error {
	paths, err := gacadv1.ResolvePaths("", "src_v1")
	if err != nil {
		return err
	}

	fs := flag.NewFlagSet("ga-cad-dev", flag.ContinueOnError)
	port := fs.Int("port", 3013, "Vite dev server port")
	host := fs.String("host", "0.0.0.0", "Vite dev server host")
	browserNode := fs.String("browser-node", "", "Optional mesh node for headed browser session (use none/off/local to disable)")
	publicURL := fs.String("public-url", "", "URL a remote browser should open")
	if err := fs.Parse(args); err != nil {
		return err
	}

	logs.Info("DIALTONE_INDEX: ga_cad dev: ensuring ui dependencies")
	if err := ensureUIDeps(paths); err != nil {
		return err
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
			RemoteBrowserRole: "ga-cad-dev",
		})
		logs.Info("DIALTONE_INDEX: ga_cad dev: browser node=%s url=%s", node, devURL)
	}

	logs.Info("DIALTONE_INDEX: ga_cad dev: starting vite on %s", localURL)
	return testv1.RunDev(testv1.DevOptions{
		RepoRoot:        paths.Runtime.RepoRoot,
		PluginDir:       paths.Preset.PluginVersionRoot,
		UIDir:           paths.UIDir,
		DevPort:         *port,
		DevHost:         strings.TrimSpace(*host),
		DevPublicURL:    devURL,
		Role:            "ga-cad-dev",
		DisableBrowser:  node == "",
		BrowserMetaPath: filepath.Join(paths.Preset.PluginVersionRoot, "dev.browser.json"),
		NATSURL:         resolveDevNATSURL(),
		NATSSubject:     "logs.dev.ga_cad.src-v1",
	})
}

func ensureUIDeps(paths gacadv1.Paths) error {
	if _, err := os.Stat(filepath.Join(paths.UIDir, "node_modules")); err == nil {
		return nil
	}
	return runBun(paths, "install")
}

func runBun(paths gacadv1.Paths, args ...string) error {
	bunBin, err := gacadv1.ResolveBunBinary(paths)
	if err != nil {
		return err
	}
	return runCmd(paths.UIDir, bunBin, args...)
}

func runCmd(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func resolveGoBin(rt configv1.Runtime) string {
	if strings.TrimSpace(rt.GoBin) != "" {
		return rt.GoBin
	}
	return filepath.Join(rt.DialtoneEnv, "go", "bin", "go")
}

func resolveDevNATSURL() string {
	if raw := strings.TrimSpace(configv1.LookupEnvString("DIALTONE_REPL_NATS_URL")); raw != "" {
		return raw
	}
	return "nats://127.0.0.1:4222"
}

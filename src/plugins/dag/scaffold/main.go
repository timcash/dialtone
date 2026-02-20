package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dialtone/dev/plugins/logs/src_v1/go"
	go_plugin "dialtone/dev/plugins/go/src_v1/go"
	bun_plugin "dialtone/dev/plugins/bun/src_v1/go"
	test_plugin "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	versionDir := ""
	if len(args) > 0 && strings.HasPrefix(args[0], "src_v") {
		versionDir = args[0]
		args = args[1:]
	} else {
		versionDir = getLatestVersionDir(repoRoot)
	}

	switch command {
	case "install":
		if err := runInstall(repoRoot, versionDir); err != nil {
			fmt.Printf("Install failed: %v\n", err)
			os.Exit(1)
		}
	case "fmt":
		pkg := "./plugins/dag/" + versionDir + "/..."
		if err := go_plugin.RunGo("fmt", pkg); err != nil {
			os.Exit(1)
		}
	case "vet":
		pkg := "./plugins/dag/" + versionDir + "/..."
		if err := go_plugin.RunGo("vet", pkg); err != nil {
			os.Exit(1)
		}
	case "go-build":
		pkg := "./plugins/dag/" + versionDir + "/..."
		if err := go_plugin.RunGo("build", pkg); err != nil {
			os.Exit(1)
		}
	case "lint":
		uiDir := filepath.Join(repoRoot, "src", "plugins", "dag", versionDir, "ui")
		if err := bun_plugin.RunBun(uiDir, "run", "lint"); err != nil {
			os.Exit(1)
		}
	case "format":
		uiDir := filepath.Join(repoRoot, "src", "plugins", "dag", versionDir, "ui")
		if err := bun_plugin.RunBun(uiDir, "run", "format"); err != nil {
			os.Exit(1)
		}
	case "build":
		uiDir := filepath.Join(repoRoot, "src", "plugins", "dag", versionDir, "ui")
		if err := bun_plugin.RunBun(uiDir, "run", "build"); err != nil {
			os.Exit(1)
		}
	case "dev":
		pluginDir := filepath.Join(repoRoot, "src", "plugins", "dag", versionDir)
		uiDir := filepath.Join(pluginDir, "ui")
		opts := test_plugin.DevOptions{
			RepoRoot:          repoRoot,
			PluginDir:         pluginDir,
			UIDir:             uiDir,
			DevPort:           3000,
			Role:              "dag-dev",
			BrowserMetaPath:   filepath.Join(pluginDir, "dev.browser.json"),
			BrowserModeEnvVar: "DAG_DEV_BROWSER_MODE",
		}
		if err := test_plugin.RunDev(opts); err != nil {
			fmt.Printf("Dev failed: %v\n", err)
			os.Exit(1)
		}
	case "test":
		testFlags := flag.NewFlagSet("dag test", flag.ContinueOnError)
		attach := testFlags.Bool("attach", false, "Attach to running headed dev browser session")
		cps := testFlags.Int("cps", 3, "Max clicks per second for UI interactions")
		_ = testFlags.Parse(args)

		repoRoot, _ := findRepoRoot()
		pluginDir := filepath.Join(repoRoot, "src", "plugins", "dag", versionDir)
		testPkg := "./plugins/dag/" + versionDir + "/suite/cmd"
		
		opts := test_plugin.TestOptions{
			RepoRoot:   repoRoot,
			PluginDir:  pluginDir,
			VersionDir: versionDir,
			Attach:     *attach,
			CPS:        *cps,
			BaseURL:    "http://127.0.0.1:8080",
			DevBaseURL: "http://127.0.0.1:3000",
			TestPkg:    testPkg,
			EnvPrefix:  "DAG",
		}
		if err := test_plugin.RunPluginTests(opts); err != nil {
			os.Exit(1)
		}
	case "serve":
		mainGo := filepath.Join("plugins", "dag", versionDir, "cmd", "main.go")
		if err := go_plugin.RunGo("run", mainGo); err != nil {
			os.Exit(1)
		}
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", fmt.Errorf("repo root not found")
		}
		cwd = parent
	}
}

func getLatestVersionDir(repoRoot string) string {
	pluginDir := filepath.Join(repoRoot, "src", "plugins", "dag")
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return "src_v1"
	}
	maxVer := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "src_v") {
			v, _ := strconv.Atoi(entry.Name()[5:])
			if v > maxVer {
				maxVer = v
			}
		}
	}
	if maxVer == 0 {
		return "src_v1"
	}
	return fmt.Sprintf("src_v%d", maxVer)
}

func runInstall(repoRoot, versionDir string) error {
	logs.Info(">> [DAG] Install: %s", versionDir)
	// Generic bun install
	uiDir := filepath.Join(repoRoot, "src", "plugins", "dag", versionDir, "ui")
	if err := bun_plugin.RunBun(uiDir, "install", "--force"); err != nil {
		return err
	}
	// Custom hook if exists
	hook := filepath.Join(repoRoot, "src", "plugins", "dag", versionDir, "cmd", "ops", "install.go")
	if _, err := os.Stat(hook); err == nil {
		logs.Info("   [DAG] Running version install hook...")
		pkg := "./plugins/dag/" + versionDir + "/cmd/ops/install.go"
		return go_plugin.RunGo("run", pkg)
	}
	return nil
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh dag <command> [src_vN] [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  install    Install dependencies")
	fmt.Println("  fmt        Run go fmt")
	fmt.Println("  vet        Run go vet")
	fmt.Println("  go-build   Run go build")
	fmt.Println("  lint       Run TS lint")
	fmt.Println("  format     Run TS format")
	fmt.Println("  build      Build UI")
	fmt.Println("  dev        Start dev server")
	fmt.Println("  test       Run tests")
	fmt.Println("  serve      Run plugin server")
}

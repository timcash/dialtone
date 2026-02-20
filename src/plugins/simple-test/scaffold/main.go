package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	versionDir := "src_v1"
	if len(args) > 0 && strings.HasPrefix(args[0], "src_v") {
		versionDir = args[0]
		args = args[1:]
	}

	switch command {
	case "install":
		uiDir := filepath.Join(repoRoot, "src", "plugins", "simple-test", versionDir, "ui")
		if err := bun_plugin.RunBun(uiDir, "install"); err != nil {
			os.Exit(1)
		}
	case "fmt":
		pkg := "./plugins/simple-test/" + versionDir + "/..."
		if err := go_plugin.RunGo("fmt", pkg); err != nil {
			os.Exit(1)
		}
	case "vet":
		pkg := "./plugins/simple-test/" + versionDir + "/..."
		if err := go_plugin.RunGo("vet", pkg); err != nil {
			os.Exit(1)
		}
	case "go-build":
		pkg := "./plugins/simple-test/" + versionDir + "/..."
		if err := go_plugin.RunGo("build", pkg); err != nil {
			os.Exit(1)
		}
	case "lint":
		uiDir := filepath.Join(repoRoot, "src", "plugins", "simple-test", versionDir, "ui")
		if err := bun_plugin.RunBun(uiDir, "run", "lint"); err != nil {
			os.Exit(1)
		}
	case "format":
		uiDir := filepath.Join(repoRoot, "src", "plugins", "simple-test", versionDir, "ui")
		if err := bun_plugin.RunBun(uiDir, "run", "format"); err != nil {
			os.Exit(1)
		}
	case "build":
		uiDir := filepath.Join(repoRoot, "src", "plugins", "simple-test", versionDir, "ui")
		if err := bun_plugin.RunBun(uiDir, "run", "build"); err != nil {
			os.Exit(1)
		}
	case "dev":
		pluginDir := filepath.Join(repoRoot, "src", "plugins", "simple-test", versionDir)
		uiDir := filepath.Join(pluginDir, "ui")
		opts := test_plugin.DevOptions{
			RepoRoot:          repoRoot,
			PluginDir:         pluginDir,
			UIDir:             uiDir,
			DevPort:           3000,
			Role:              "simple-test-dev",
			BrowserMetaPath:   filepath.Join(pluginDir, "dev.browser.json"),
			BrowserModeEnvVar: "SIMPLE_TEST_DEV_BROWSER_MODE",
		}
		if err := test_plugin.RunDev(opts); err != nil {
			fmt.Printf("Dev failed: %v\n", err)
			os.Exit(1)
		}
	case "test":
		testFlags := flag.NewFlagSet("simple-test test", flag.ContinueOnError)
		attach := testFlags.Bool("attach", false, "Attach to running headed dev browser session")
		_ = testFlags.Parse(args)

		pluginDir := filepath.Join(repoRoot, "src", "plugins", "simple-test", versionDir)
		testPkg := "./plugins/simple-test/" + versionDir + "/test/cmd"
		
		opts := test_plugin.TestOptions{
			RepoRoot:   repoRoot,
			PluginDir:  pluginDir,
			VersionDir: versionDir,
			Attach:     *attach,
			BaseURL:    "http://127.0.0.1:3000",
			DevBaseURL: "http://127.0.0.1:3000",
			TestPkg:    testPkg,
			EnvPrefix:  "SIMPLE_TEST",
		}
		if err := test_plugin.RunPluginTests(opts); err != nil {
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

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh simple-test <command> [src_vN] [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  install    Install dependencies")
	fmt.Println("  dev        Start dev server")
	fmt.Println("  build      Build UI")
	fmt.Println("  test       Run tests")
	fmt.Println("  lint       Run TS lint")
	fmt.Println("  format     Run TS format")
}

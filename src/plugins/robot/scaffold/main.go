package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	robot_ops "dialtone/dev/plugins/robot/src_v1/cmd/ops"
	robot_cli "dialtone/dev/plugins/robot/src_v1/cmd/cli"
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

	// For src_v1, we use the specialized logic in cmd/ops
	if versionDir == "src_v1" {
		switch command {
		case "install":
			err = robot_ops.Install()
		case "fmt":
			err = robot_ops.Fmt()
		case "vet":
			err = robot_ops.Vet()
		case "go-build":
			err = robot_ops.GoBuild()
		case "lint":
			err = robot_ops.Lint()
		case "format":
			err = robot_ops.Format()
		case "build":
			err = robot_ops.Build(args...)
		case "dev":
			err = robot_ops.Dev(repoRoot, args)
		case "test":
			err = robot_ops.Test(repoRoot, args)
		case "serve":
			err = robot_ops.Serve(repoRoot)
		case "ui-run":
			port := 0
			for i, arg := range args {
				if arg == "--port" && i+1 < len(args) {
					port, _ = strconv.Atoi(args[i+1])
				}
			}
			err = robot_ops.UIRun(port)
		case "deploy-test":
			err = robot_cli.RunDeployTest(versionDir, args)
		case "diagnostic":
			err = robot_cli.RunDiagnostic(versionDir)
		case "vpn-test":
			err = robot_cli.RunVPNTest(args)
		default:
			fmt.Printf("Unknown command: %s\n", command)
			printUsage()
			os.Exit(1)
		}
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Generic fallback for other versions (if any)
	switch command {
	case "install":
		uiDir := filepath.Join(repoRoot, "src", "plugins", "robot", versionDir, "ui")
		if err := bun_plugin.RunBun(uiDir, "install", "--force"); err != nil {
			os.Exit(1)
		}
	case "fmt":
		pkg := "./plugins/robot/" + versionDir + "/..."
		if err := go_plugin.RunGo("fmt", pkg); err != nil {
			os.Exit(1)
		}
	case "vet":
		pkg := "./plugins/robot/" + versionDir + "/..."
		if err := go_plugin.RunGo("vet", pkg); err != nil {
			os.Exit(1)
		}
	case "go-build":
		pkg := "./plugins/robot/" + versionDir + "/..."
		if err := go_plugin.RunGo("build", pkg); err != nil {
			os.Exit(1)
		}
	case "lint":
		uiDir := filepath.Join(repoRoot, "src", "plugins", "robot", versionDir, "ui")
		if err := bun_plugin.RunBun(uiDir, "run", "lint"); err != nil {
			os.Exit(1)
		}
	case "format":
		uiDir := filepath.Join(repoRoot, "src", "plugins", "robot", versionDir, "ui")
		if err := bun_plugin.RunBun(uiDir, "run", "format"); err != nil {
			os.Exit(1)
		}
	case "build":
		uiDir := filepath.Join(repoRoot, "src", "plugins", "robot", versionDir, "ui")
		if err := bun_plugin.RunBun(uiDir, "run", "build"); err != nil {
			os.Exit(1)
		}
	case "dev":
		pluginDir := filepath.Join(repoRoot, "src", "plugins", "robot", versionDir)
		uiDir := filepath.Join(pluginDir, "ui")
		opts := test_plugin.DevOptions{
			RepoRoot:          repoRoot,
			PluginDir:         pluginDir,
			UIDir:             uiDir,
			DevPort:           3000,
			Role:              "robot-dev",
			BrowserMetaPath:   filepath.Join(pluginDir, "dev.browser.json"),
			BrowserModeEnvVar: "ROBOT_DEV_BROWSER_MODE",
		}
		if err := test_plugin.RunDev(opts); err != nil {
			fmt.Printf("Dev failed: %v\n", err)
			os.Exit(1)
		}
	case "test":
		// Standard test plugin logic...
		fmt.Println("Standard test logic not yet implemented for Robot generic fallback")
		os.Exit(1)
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
	pluginDir := filepath.Join(repoRoot, "src", "plugins", "robot")
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

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh robot <command> [src_vN] [options]")
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
	fmt.Println("  deploy-test Run step-by-step verification on remote robot")
	fmt.Println("  diagnostic Run UI and connectivity diagnostics")
	fmt.Println("  vpn-test   Test Tailscale connectivity")
}

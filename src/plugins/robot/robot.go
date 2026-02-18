package robot

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	rcli "dialtone/cli/src/plugins/robot/robot_cli"
	robot_ops "dialtone/cli/src/plugins/robot/src_v1/cmd/ops"
)

// RunRobot handles 'robot <subcommand>'
func RunRobot(args []string) {
	if len(args) == 0 {
		printRobotUsage()
		return
	}

	subcommand := args[0]
	restArgs := args[1:]

	// Helper to get directory with latest default
	getDir := func() string {
		if len(args) > 1 && strings.HasPrefix(args[1], "src_v") {
			return args[1]
		}
		return getLatestVersionDir()
	}

	switch subcommand {
	case "start":
		vDir := getDir()
		var startArgs []string
		for _, arg := range restArgs {
			if arg != vDir {
				startArgs = append(startArgs, arg)
			}
		}
		RunStart(startArgs)
	case "deploy":
		vDir := getDir()
		var deployArgs []string
		for _, arg := range restArgs {
			if arg != vDir {
				deployArgs = append(deployArgs, arg)
			}
		}
		RunDeploy(vDir, deployArgs)
	case "sleep":
		vDir := getDir()
		var sleepArgs []string
		for _, arg := range restArgs {
			if arg != vDir {
				sleepArgs = append(sleepArgs, arg)
			}
		}
		RunSleep(vDir, sleepArgs)
	case "sync-code":
		vDir := getDir()
		var syncArgs []string
		for _, arg := range restArgs {
			if arg != vDir {
				syncArgs = append(syncArgs, arg)
			}
		}
		RunSyncCode(vDir, syncArgs)
	case "deploy-test":
		vDir := getDir()
		cmdArgs := restArgs
		if len(restArgs) > 0 && restArgs[0] == vDir {
			cmdArgs = restArgs[1:]
		}
		if err := rcli.RunDeployTest(vDir, cmdArgs); err != nil {
			fmt.Printf("Robot deploy-test error: %v\n", err)
			os.Exit(1)
		}
	case "vpn-test":
		if err := rcli.RunVPNTest(restArgs); err != nil {
			fmt.Printf("Robot vpn-test error: %v\n", err)
			os.Exit(1)
		}
	case "install":
		vDir := getDir()
		var installArgs []string
		for _, arg := range restArgs {
			if arg != vDir {
				installArgs = append(installArgs, arg)
			}
		}
		if err := RunInstall(vDir, installArgs...); err != nil {
			fmt.Printf("Robot install error: %v\n", err)
			os.Exit(1)
		}
	case "fmt":
		RunFmt(getDir())
	case "format":
		RunFormat(getDir())
	case "vet":
		RunVet(getDir())
	case "go-build":
		RunGoBuild(getDir())
	case "lint":
		RunLint(getDir())
	case "serve":
		RunServe(getDir())
	case "ui-run":
		RunUIRun(getDir(), args[2:])
	case "dev":
		RunDev(getDir(), restArgs)
	case "local-web-remote-robot":
		vDir := getDir()
		if err := RunLocalWebRemoteRobot(vDir); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "build":
		if err := robot_ops.Build(); err != nil {
			fmt.Printf("Robot build error: %v\n", err)
			os.Exit(1)
		}
	case "test":
		vDir := getDir()
		var testArgs []string
		for _, arg := range restArgs {
			if arg != vDir {
				testArgs = append(testArgs, arg)
			}
		}
		RunVersionedTest(vDir, testArgs)
	case "diagnostic":
		if len(restArgs) == 0 {
			fmt.Println("Usage: dialtone robot diagnostic <src_vN>")
			os.Exit(1)
		}
		if err := rcli.RunDiagnostic(restArgs[0]); err != nil {
			fmt.Printf("Robot diagnostic error: %v\n", err)
			os.Exit(1)
		}
	case "telemetry":
		RunTelemetry(restArgs)
	case "help", "-h", "--help":
		printRobotUsage()
	default:
		fmt.Printf("Unknown robot subcommand: %s\n", subcommand)
		printRobotUsage()
	}
}

func printRobotUsage() {
	fmt.Println("Usage: dialtone <command> robot <subcommand> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  start       Start the NATS and Web server (core robot logic)")
	fmt.Println("  deploy      Deploy binary to remote robot via SSH")
	fmt.Println("  sleep       Replace robot binary with lightweight sleep server")
	fmt.Println("  sync-code   Sync source code to robot for remote building")
	fmt.Println("  deploy-test Step-by-step remote verification using debug binaries")
	fmt.Println("  vpn-test    Test Tailscale (tsnet) connectivity")
	fmt.Println("\nVersioned Source Commands (src_vN):")
	fmt.Println("  install     Install UI dependencies [--remote]")
	fmt.Println("  fmt         Run formatting checks/fixes")
	fmt.Println("  format      Run UI format checks")
	fmt.Println("  vet         Run go vet checks")
	fmt.Println("  go-build    Run go build checks")
	fmt.Println("  lint        Run lint checks")
	fmt.Println("  dev         Run UI in development mode")
	fmt.Println("  local-web-remote-robot Run local UI connected to a remote robot")
	fmt.Println("  build       Build everything needed (UI assets) [--remote]")
	fmt.Println("  serve       Run the plugin Go server")
	fmt.Println("  test        Run automated test suite")
	fmt.Println("  diagnostic  Run UI diagnostic against a deployed robot")
	fmt.Println("  telemetry   Monitor MAVLink latency on local NATS")
}

func getLatestVersionDir() string {
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "robot")
	entries, _ := os.ReadDir(pluginDir)
	maxVer := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "src_v") {
			ver, _ := strconv.Atoi(e.Name()[5:])
			if ver > maxVer {
				maxVer = ver
			}
		}
	}
	if maxVer == 0 {
		return "src_v1"
	}
	return fmt.Sprintf("src_v%d", maxVer)
}

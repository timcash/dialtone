package cli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runBun(repoRoot, uiDir string, args ...string) *exec.Cmd {
	bunArgs := append([]string{"bun", "exec", "--cwd", uiDir}, args...)
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), bunArgs...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// Run is the main entry for the logs plugin CLI (shared lib + CLI; test with logs test src_v1, logs dev src_v1).
func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	if isHelpArg(args[0]) {
		printUsage()
		return nil
	}

	version, command, rest, warnedOldOrder, err := parseArgs(args)
	if err != nil {
		return err
	}
	if warnedOldOrder {
		fmt.Println("[WARN] old logs CLI order is deprecated. Use: ./dialtone.sh logs src_v1 <command> [args]")
	}

	switch command {
	case "install":
		return RunInstall(version)
	case "fmt":
		return RunFmt(version)
	case "format":
		return RunFormat(version)
	case "vet":
		return RunVet(version)
	case "go-build":
		return RunGoBuild(version)
	case "lint":
		return RunLint(version)
	case "dev":
		return RunDev(version)
	case "ui-run":
		return RunUIRun(version, rest)
	case "serve":
		return RunServe(version)
	case "build":
		return RunBuild(version)
	case "test":
		testFlags := flag.NewFlagSet("logs test", flag.ContinueOnError)
		attach := testFlags.Bool("attach", false, "Attach to running headed dev browser session")
		cps := testFlags.Int("cps", 3, "Max clicks per second for UI interactions (must be >= 1)")
		_ = testFlags.Parse(rest)
		if *cps < 1 {
			return fmt.Errorf("--cps must be >= 1")
		}
		return RunTest(version, *attach, *cps)
	case "stream", "tail":
		RunLogs(version, rest)
		return nil
	case "pingpong":
		return RunPingPong(version, rest)
	case "nats-daemon":
		return RunNATSDaemon(version, rest)
	case "nats-start":
		return RunNATSStart(version, rest)
	case "nats-status":
		return RunNATSStatus(version, rest)
	case "nats-stop":
		return RunNATSStop(version, rest)
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func parseArgs(args []string) (version string, command string, rest []string, warnedOldOrder bool, err error) {
	if len(args) == 0 {
		return "", "", nil, false, fmt.Errorf("missing arguments")
	}

	if strings.HasPrefix(strings.TrimSpace(args[0]), "src_v") {
		version = strings.TrimSpace(args[0])
		if len(args) < 2 {
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh logs %s <command> [args])", version)
		}
		command = strings.TrimSpace(args[1])
		rest = args[2:]
		return version, command, rest, false, nil
	}

	if len(args) >= 2 && strings.HasPrefix(strings.TrimSpace(args[1]), "src_v") {
		version = strings.TrimSpace(args[1])
		command = strings.TrimSpace(args[0])
		rest = args[2:]
		return version, command, rest, true, nil
	}

	return "", "", nil, false, fmt.Errorf("expected version as first logs argument (usage: ./dialtone.sh logs src_v1 <command> [args])")
}

func isHelpArg(s string) bool {
	switch strings.TrimSpace(s) {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh logs src_v1 <command> [args]")
	fmt.Println("\nCommands:")
	fmt.Println("  install <dir>  Install Go/Bun and UI deps")
	fmt.Println("  fmt <dir>      Run go fmt")
	fmt.Println("  format <dir>   Run UI format checks")
	fmt.Println("  vet <dir>      Run go vet")
	fmt.Println("  go-build <dir> Run go build")
	fmt.Println("  lint <dir>     Run TypeScript lint checks")
	fmt.Println("  dev <dir>      Start Vite + debug browser attach")
	fmt.Println("  ui-run <dir>   Run UI dev server")
	fmt.Println("  serve <dir>   Run plugin Go server")
	fmt.Println("  build <dir>   Build UI assets")
	fmt.Println("  test <dir>    Run automated tests and write TEST.md artifacts")
	fmt.Println("  stream        Stream logs (local or --remote from robot)")
	fmt.Println("  tail          Alias for stream")
	fmt.Println("  pingpong      Ping/pong test participant for NATS topic")
	fmt.Println("  nats-start    Start local embedded NATS daemon")
	fmt.Println("  nats-status   Check local NATS daemon status")
	fmt.Println("  nats-stop     Stop local NATS daemon")
	fmt.Println("\nExamples:")
	fmt.Println("  ./dialtone.sh logs src_v1 test")
	fmt.Println("  ./dialtone.sh logs src_v1 test --attach")
	fmt.Println("  ./dialtone.sh logs src_v1 dev")
	fmt.Println("  ./dialtone.sh logs src_v1 stream --remote")
}

package logsv1

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func resolveLogsPaths(versionDir string) (logs.Paths, error) {
	return logs.ResolvePaths("", versionDir)
}

func runBun(repoRoot, uiDir string, args ...string) *exec.Cmd {
	bunArgs := append([]string{"bun", "src_v1", "exec", "--cwd", uiDir}, args...)
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), bunArgs...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// Run is the main entry for the logs plugin CLI (shared lib + CLI; test with logs test src_v1, logs dev src_v1).
func Run(args []string) error {
	logs.SetOutput(os.Stdout)
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
		logs.Warn("old logs CLI order is deprecated. Use: ./dialtone.sh logs src_v1 <command> [args]")
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
		return RunTest(version, rest)
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
	logs.Raw("Usage: ./dialtone.sh logs src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  install     Install UI dependencies")
	logs.Raw("  fmt         Run go fmt")
	logs.Raw("  format      Run UI format checks")
	logs.Raw("  vet         Run go vet")
	logs.Raw("  go-build    Run go build")
	logs.Raw("  lint        Run TypeScript lint checks")
	logs.Raw("  dev         Start Vite + debug browser attach")
	logs.Raw("  ui-run      Run UI dev server")
	logs.Raw("  serve       Run plugin Go server")
	logs.Raw("  build       Build Go package and UI assets")
	logs.Raw("  test        Run automated tests and write TEST.md artifacts")
	logs.Raw("  stream      Stream logs (local or --remote from robot)")
	logs.Raw("  tail        Alias for stream")
	logs.Raw("  pingpong    Ping/pong test participant for NATS topic")
	logs.Raw("  nats-start  Start local embedded NATS daemon")
	logs.Raw("  nats-status Check local NATS daemon status")
	logs.Raw("  nats-stop   Stop local NATS daemon")
	logs.Raw("")
	logs.Raw("Examples:")
	logs.Raw("  ./dialtone.sh logs src_v1 test")
	logs.Raw("  ./dialtone.sh logs src_v1 test --filter infra")
	logs.Raw("  ./dialtone.sh logs src_v1 build")
	logs.Raw("  ./dialtone.sh logs src_v1 stream --topic 'logs.>'")
}

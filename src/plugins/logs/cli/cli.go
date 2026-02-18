package cli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

	command := args[0]
	getDir := func() string {
		if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
			return args[1]
		}
		return getLatestVersionDir()
	}

	switch command {
	case "install":
		return RunInstall(getDir())
	case "fmt":
		return RunFmt(getDir())
	case "format":
		return RunFormat(getDir())
	case "vet":
		return RunVet(getDir())
	case "go-build":
		return RunGoBuild(getDir())
	case "lint":
		return RunLint(getDir())
	case "dev":
		return RunDev(getDir())
	case "ui-run":
		extraArgs := []string{}
		if len(args) > 2 {
			extraArgs = args[2:]
		}
		return RunUIRun(getDir(), extraArgs)
	case "serve":
		return RunServe(getDir())
	case "build":
		return RunBuild(getDir())
	case "test":
		testFlags := flag.NewFlagSet("logs test", flag.ContinueOnError)
		attach := testFlags.Bool("attach", false, "Attach to running headed dev browser session")
		cps := testFlags.Int("cps", 3, "Max clicks per second for UI interactions (must be >= 1)")
		dir := getDir()
		if len(args) > 1 && args[1] != "" && !strings.HasPrefix(args[1], "-") {
			dir = args[1]
			_ = testFlags.Parse(args[2:])
		} else {
			_ = testFlags.Parse(args[1:])
		}
		if *cps < 1 {
			return fmt.Errorf("--cps must be >= 1")
		}
		return RunTest(dir, *attach, *cps)
	case "stream", "tail":
		RunLogs(args[1:])
		return nil
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh logs <command> [args]")
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
	fmt.Println("\nDefault <dir> is the latest src_vN folder.")
	fmt.Println("\nExamples:")
	fmt.Println("  ./dialtone.sh logs test src_v1")
	fmt.Println("  ./dialtone.sh logs test src_v1 --attach")
	fmt.Println("  ./dialtone.sh logs dev src_v1")
	fmt.Println("  ./dialtone.sh logs stream --remote")
}

func getLatestVersionDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "src_v1"
	}
	pluginDir := filepath.Join(cwd, "src", "plugins", "logs")
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return "src_v1"
	}
	maxVer := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "src_v") {
			continue
		}
		version, err := strconv.Atoi(strings.TrimPrefix(name, "src_v"))
		if err != nil {
			continue
		}
		if version > maxVer {
			maxVer = version
		}
	}
	if maxVer == 0 {
		return "src_v1"
	}
	return fmt.Sprintf("src_v%d", maxVer)
}

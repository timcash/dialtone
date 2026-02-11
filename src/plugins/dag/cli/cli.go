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

func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	command := args[0]
	switch command {
	case "smoke":
		smokeFlags := flag.NewFlagSet("dag smoke", flag.ContinueOnError)
		timeout := smokeFlags.Int("smoke-timeout", 45, "Timeout in seconds for smoke test")

		dir := getLatestVersionDir()
		if len(args) > 1 && args[1] != "" && !strings.HasPrefix(args[1], "-") {
			dir = args[1]
			smokeFlags.Parse(args[2:])
		} else {
			smokeFlags.Parse(args[1:])
		}

		return runSmoke(dir, *timeout)
	case "dev":
		dir := getLatestVersionDir()
		if len(args) > 1 {
			dir = args[1]
		}
		return RunDev(dir)
	case "build":
		dir := getLatestVersionDir()
		if len(args) > 1 {
			dir = args[1]
		}
		return RunBuild(dir)
	case "lint":
		dir := ""
		if len(args) > 1 {
			dir = args[1]
		}
		return RunLint(dir)
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh dag <command> [args]")
	fmt.Println("\nCommands:")
	fmt.Println("  dev [dir]                           Start UI in Vite development mode")
	fmt.Println("  build [dir]                         Build UI assets")
	fmt.Println("  lint [dir]                          Lint Go + TypeScript")
	fmt.Println("  smoke [dir] [--smoke-timeout <sec>] Run automated UI tests (runs lint + build first)")
	fmt.Println("\nDefault [dir] is the latest src_vN folder.")
}

func runSmoke(versionDir string, timeoutSec int) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	smokeFile := filepath.Join(cwd, "src", "plugins", "dag", versionDir, "smoke", "smoke.go")
	if _, err := os.Stat(smokeFile); os.IsNotExist(err) {
		return fmt.Errorf("smoke test file not found: %s", smokeFile)
	}

	cmd := exec.Command(
		filepath.Join(cwd, "dialtone.sh"),
		"go", "exec", "run", smokeFile, versionDir, strconv.Itoa(timeoutSec),
	)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getLatestVersionDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "src_v1"
	}
	pluginDir := filepath.Join(cwd, "src", "plugins", "dag")
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

package cli

import (
	"dialtone/cli/src/plugins/wsl/test"
	"flag"
	"fmt"
)

func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	command := args[0]
	switch command {
	case "smoke":
		smokeFlags := flag.NewFlagSet("wsl smoke", flag.ContinueOnError)
		timeout := smokeFlags.Int("smoke-timeout", 120, "Timeout in seconds for smoke test")

		if len(args) < 2 {
			return fmt.Errorf("usage: wsl smoke <dir> [--smoke-timeout <sec>]")
		}

		dir := args[1]
		smokeFlags.Parse(args[2:])

		// Always build before smoke test
		if err := RunBuild(dir); err != nil {
			return fmt.Errorf("pre-smoke build failed: %v", err)
		}

		return test.RunSmoke(dir, *timeout)
	case "lint":
		return RunLint()
	case "dev":
		dir := "src_v1"
		if len(args) > 1 {
			dir = args[1]
		}
		return RunDev(dir)
	case "build":
		dir := "src_v1"
		if len(args) > 1 {
			dir = args[1]
		}
		return RunBuild(dir)
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh wsl <command> [args]")
	fmt.Println("\nCommands:")
	fmt.Println("  dev <dir>                            Start host and UI in development mode")
	fmt.Println("  build <dir>                          Build everything needed (UI assets)")
	fmt.Println("  smoke <dir> [--smoke-timeout <sec>]  Run automated UI tests")
	fmt.Println("  lint                                 Lint Go and TypeScript code")
}

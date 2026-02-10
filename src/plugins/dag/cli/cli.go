package cli

import (
	"dialtone/cli/src/plugins/dag/test"
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
		smokeFlags := flag.NewFlagSet("dag smoke", flag.ContinueOnError)
		timeout := smokeFlags.Int("smoke-timeout", 45, "Timeout in seconds for smoke test")

		dir := "src_v1"
		if len(args) > 1 && args[1] != "" {
			dir = args[1]
			smokeFlags.Parse(args[2:])
		} else {
			smokeFlags.Parse(args[1:])
		}

		return test.RunSmoke(dir, *timeout)
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
	fmt.Println("  dev [dir]                           Start host and UI in development mode")
	fmt.Println("  build [dir]                         Build UI assets")
	fmt.Println("  lint [dir]                          Lint Go + TypeScript")
	fmt.Println("  smoke [dir] [--smoke-timeout <sec>] Run automated UI tests (runs lint + build first)")
}

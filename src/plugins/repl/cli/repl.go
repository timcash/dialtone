package cli

import (
	"flag"
	"fmt"

	repl_test "dialtone/cli/src/plugins/repl/test"
)

func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "test":
		testFlags := flag.NewFlagSet("repl test", flag.ContinueOnError)
		timeout := testFlags.Int("timeout", 180, "Timeout in seconds for REPL workflow test")
		if err := testFlags.Parse(args[1:]); err != nil {
			return err
		}
		return repl_test.RunInstallWorkflow(*timeout)
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown repl command: %s", args[0])
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh repl <command> [args]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  test [--timeout <sec>]   Run REPL robot-install workflow test")
	fmt.Println("  help                     Show this help")
}

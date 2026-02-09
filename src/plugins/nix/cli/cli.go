package cli

import (
	"dialtone/cli/src/plugins/nix/test"
	"flag"
	"fmt"
)

func Run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: nix <command> [args]\n\nCommands:\n  smoke <dir> [--smoke-timeout <sec>]  Run automated UI tests\n  lint                                 Lint Go and TypeScript code")
	}

	command := args[0]
	switch command {
	case "smoke":
		smokeFlags := flag.NewFlagSet("nix smoke", flag.ContinueOnError)
		timeout := smokeFlags.Int("smoke-timeout", 45, "Timeout in seconds for smoke test")

		if len(args) < 2 {
			return fmt.Errorf("usage: nix smoke <dir> [--smoke-timeout <sec>]")
		}

		dir := args[1]
		smokeFlags.Parse(args[2:])

		return test.RunSmoke(dir, *timeout)
	case "lint":
		return RunLint()
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

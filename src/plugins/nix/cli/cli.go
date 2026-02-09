package cli

import (
	"fmt"
	"dialtone/cli/src/plugins/nix/test"
)

func Run(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: nix smoke <dir>")
	}

	command := args[0]
	dir := args[1]

	switch command {
	case "smoke":
		return test.RunSmoke(dir)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}
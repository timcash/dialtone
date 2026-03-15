package cli

import (
	"flag"
	"fmt"

	chromev4 "dialtone/dev/plugins/chrome/src_v4/go"
)

func Run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing command")
	}

	command := args[0]
	rest := args[1:]

	switch command {
	case "daemon":
		flags := flag.NewFlagSet("daemon", flag.ExitOnError)
		port := flags.Int("port", 9333, "MCP Port")
		if err := flags.Parse(rest); err != nil {
			return err
		}
		return chromev4.RunDaemon(*port)
	case "goto":
		if len(rest) == 0 {
			return fmt.Errorf("missing URL")
		}
		return chromev4.ExecuteCommand("goto", map[string]any{"url": rest[0]})
	case "mcp-call":
		if len(rest) < 1 {
			return fmt.Errorf("missing tool name")
		}
		return chromev4.ExecuteCommand("mcp_call", map[string]any{"tool": rest[0], "args": rest[1:]})
	case "help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh chrome src_v4 <command> [args]")
	fmt.Println("\nCommands:")
	fmt.Println("  daemon       Start the background Chrome MCP daemon proxy")
	fmt.Println("  goto <url>   Navigate the active browser to a URL")
	fmt.Println("  mcp-call     Execute a WebMCP tool on the current page")
}

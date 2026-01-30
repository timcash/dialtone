package cli

import (
"fmt"
)

// Run handles the 'earth' command
func Run(args []string) {
	if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		printUsage()
		return
	}

	fmt.Println("earth module: Core functionality.")
	printUsage()
}

func printUsage() {
	fmt.Println("Usage: dialtone-dev earth <command>")
	fmt.Println("\nCommands:")
	fmt.Println("  help       Show this help message")
}

package cli

import (
"fmt"
)

// Run handles the 'browser' command
func Run(args []string) {
	if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		printUsage()
		return
	}

	fmt.Println("browser module: Core functionality.")
	printUsage()
}

func printUsage() {
	fmt.Println("Usage: dialtone-dev browser <command>")
	fmt.Println("\nCommands:")
	fmt.Println("  help       Show this help message")
}

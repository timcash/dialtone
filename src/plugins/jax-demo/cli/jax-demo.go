package cli

import (
	"fmt"
)

// Run handles the 'jax-demo' command
func Run(args []string) {
	if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		printUsage()
		return
	}

	fmt.Println("JAX Demo Plugin: This is a demo plugin.")
	printUsage()
}

func printUsage() {
	fmt.Println("Usage: dialtone-dev jax-demo <command>")
	fmt.Println("\nCommands:")
	fmt.Println("  help       Show this help message")
}

package cli

import (
	"dialtone/cli/src/core/install"
	"fmt"
)

// Run handles the 'install' command
func Run(args []string) {
	if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		printUsage()
		return
	}

	install.RunInstall(args)
}

func printUsage() {
	fmt.Println("Usage: dialtone install [options] [install-path]")
	fmt.Println()
	fmt.Println("Install development dependencies (Go, Node.js, Zig, GH CLI, Pixi) for building Dialtone.")
	fmt.Println()
	fmt.Println("Arguments:")
	fmt.Println("  [install-path]  Optional: Path where dependencies should be installed.")
	fmt.Println("                  Overrides DIALTONE_ENV and default locations.")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --linux-wsl   Install for Linux/WSL x86_64")
	fmt.Println("  --macos-arm   Install for macOS ARM (Apple Silicon)")
	fmt.Println("  --host        SSH host for remote installation (user@host)")
	fmt.Println("  --port        SSH port (default: 22)")
	fmt.Println("  --user        SSH username")
	fmt.Println("  --pass        SSH password")
	fmt.Println("  --clean       Remove all dependencies before installation")
	fmt.Println("  --check       Check if dependencies are installed and exit")
	fmt.Println("  help          Show help for install command")
	fmt.Println("  --help        Show help for install command")
	fmt.Println()
	fmt.Println("Notes:")
	fmt.Println("  - Dependencies are installed to the directory specified by DIALTONE_ENV")
	fmt.Println("  - Default location is ~/.dialtone_env")
}

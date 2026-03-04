package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "help", "-h", "--help":
		printUsage()
	case "install":
		if err := runInstall(args); err != nil {
			exitIfErr(err, "mosh install")
		}
	case "setup":
		if err := runSetup(args); err != nil {
			exitIfErr(err, "mosh setup")
		}
	case "connect":
		if err := runConnect(args); err != nil {
			exitIfErr(err, "mosh connect")
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown mosh v1 command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone2.sh mosh v1 <install|setup|connect> [options]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  install [--nixpkgs-url URL] [--ensure]")
	fmt.Println("      Check for mosh availability; use --ensure to install via nix profile.")
	fmt.Println("  setup [--host NAME] [--ensure]")
	fmt.Println("      Verify local or remote mosh-server availability.")
	fmt.Println("      --host    target host for server setup check")
	fmt.Println("      --ensure  try to install mosh on target using nix profile")
	fmt.Println("  connect [--host NAME] [--ensure] [--session NAME] [--command CMD]")
	fmt.Println("         [--repo-root PATH] [--fallback-ssh] [--dry-run]")
	fmt.Println("      Connect to remote tmux shell through mosh or ssh fallback.")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", context, err)
	os.Exit(1)
}

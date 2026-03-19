package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "-h", "--help", "help":
		printUsage()
	case "install":
		if err := runInstall(args); err != nil {
			exitIfErr(err, "repl install")
		}
	case "build":
		if err := runBuild(args); err != nil {
			exitIfErr(err, "repl build")
		}
	case "format":
		if err := runFormat(args); err != nil {
			exitIfErr(err, "repl format")
		}
	case "test":
		if err := runTest(args); err != nil {
			exitIfErr(err, "repl test")
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown repl v1 command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod repl v1 <install|build|format|test> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  install                                  Verify the repo Nix dev shell can load repl prerequisites")
	fmt.Println("  build                                    Build repl v1 binary to <repo-root>/bin")
	fmt.Println("  format [--dir DIR]                       Run gofmt via nix on repl v1 Go files")
	fmt.Println("  test                                     Run go test via nix for repl v1")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s error: %v\n", context, err)
	os.Exit(1)
}

func parseFormatArgs(argv []string) (string, error) {
	fs := flag.NewFlagSet("repl v1 format", flag.ContinueOnError)
	dir := fs.String("dir", "", "Directory to format (default: src/mods/repl/v1)")
	if err := fs.Parse(argv); err != nil {
		return "", err
	}
	return filepath.Clean(*dir), nil
}

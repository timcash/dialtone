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
		return
	case "install":
		if err := runInstall(args); err != nil {
			exitIfErr(err, "mod install")
		}
	case "build":
		if err := runBuild(args); err != nil {
			exitIfErr(err, "mod build")
		}
	case "format":
		if err := runFormat(args); err != nil {
			exitIfErr(err, "mod format")
		}
	case "test":
		if err := runTest(args); err != nil {
			exitIfErr(err, "mod test")
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown mod v1 command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone2.sh mod v1 <install|build|format|test> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  install                                  Prepare shell environment (nix develop)")
	fmt.Println("  build                                    Build dialtone mod CLI to <repo-root>/bin")
	fmt.Println("  format [--dir DIR]                       Run gofmt on Go files")
	fmt.Println("  test                                     Run go test for mod management code")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s error: %v\n", context, err)
	os.Exit(1)
}

func parseFormatArgs(argv []string) (string, error) {
	fs := flag.NewFlagSet("mods v1 format", flag.ContinueOnError)
	dir := fs.String("dir", "", "Directory to format (default: src/mods/mod/v1)")
	if err := fs.Parse(argv); err != nil {
		return "", err
	}
	return filepath.Clean(*dir), nil
}

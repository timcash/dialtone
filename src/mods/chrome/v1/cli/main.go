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
			exitIfErr(err, "chrome install")
		}
	case "build":
		if err := runBuild(args); err != nil {
			exitIfErr(err, "chrome build")
		}
	case "format":
		if err := runFormat(args); err != nil {
			exitIfErr(err, "chrome format")
		}
	case "test":
		if err := runTest(args); err != nil {
			exitIfErr(err, "chrome test")
		}
	case "__service-loop":
		if err := runServer(args); err != nil {
			exitIfErr(err, "chrome __service-loop")
		}
	case "service":
		runServiceCommand(args)
	case "tab":
		runTabCommand(args)
	default:
		fmt.Fprintf(os.Stderr, "unknown chrome v1 command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runServiceCommand(args []string) {
	if len(args) < 1 {
		exitIfErr(fmt.Errorf("service action required: start|stop|status"), "chrome service")
	}
	action := args[0]
	rest := args[1:]
	switch action {
	case "start":
		exitIfErr(runStart(rest), "chrome service start")
	case "stop":
		exitIfErr(runStop(rest), "chrome service stop")
	case "status":
		exitIfErr(runStatus(rest), "chrome service status")
	default:
		exitIfErr(fmt.Errorf("unknown service action: %s", action), "chrome service")
	}
}

func runTabCommand(args []string) {
	if len(args) < 1 {
		exitIfErr(fmt.Errorf("tab action required: open|close|goto|list"), "chrome tab")
	}
	action := args[0]
	rest := args[1:]
	switch action {
	case "open":
		exitIfErr(runTabOpen(rest), "chrome tab open")
	case "close":
		exitIfErr(runTabClose(rest), "chrome tab close")
	case "goto":
		exitIfErr(runTabGoto(rest), "chrome tab goto")
	case "list":
		exitIfErr(runTabList(rest), "chrome tab list")
	default:
		exitIfErr(fmt.Errorf("unknown tab action: %s", action), "chrome tab")
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod chrome v1 <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  install                                             Verify nix shell packages for chrome workflows")
	fmt.Println("  build                                               Build chrome v1 CLI binary to <repo-root>/bin")
	fmt.Println("  format [--dir DIR]                                  Run gofmt on Go files")
	fmt.Println("  test [--filter PATTERN] [--integration]             Run go test for chrome CLI code")
	fmt.Println("  service start [--host HOST] [--port PORT]           Start background chrome+nats service")
	fmt.Println("        [--nats-url URL] [--nats-prefix PREFIX] [--embedded-nats]")
	fmt.Println("        [--chrome-debug-port PORT] [--headless] [--initial-url URL]")
	fmt.Println("  service stop                                        Stop background service")
	fmt.Println("  service status                                      Show service status")
	fmt.Println("  tab open [--tab NAME] [--url URL]                   Open tab via NATS")
	fmt.Println("  tab close [--tab NAME]                              Close tab via NATS")
	fmt.Println("  tab goto [--tab NAME] --url URL                     Navigate tab via NATS")
	fmt.Println("  tab list                                            List tabs via NATS")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s error: %v\n", context, err)
	os.Exit(1)
}

func parseFormatArgs(argv []string) (string, error) {
	fs := flag.NewFlagSet("chrome v1 format", flag.ContinueOnError)
	dir := fs.String("dir", "", "Directory to format (default: src/mods/chrome/v1)")
	if err := fs.Parse(argv); err != nil {
		return "", err
	}
	return filepath.Clean(*dir), nil
}

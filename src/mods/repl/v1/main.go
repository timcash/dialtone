package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

const buildVersion = "v1"

func main() {
	if err := run(os.Args[1:]); err != nil {
		exitIfErr(err, "repl")
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	command := strings.TrimSpace(args[0])
	rest := args[1:]

	switch command {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "version":
		fmt.Println(buildVersion)
		return nil
	case "run":
		return runREPL(rest)
	case "logs":
		return runLogs(rest)
	default:
		return fmt.Errorf("unknown repl command: %s", command)
	}
}

func runREPL(args []string) error {
	cfg, err := parseRunConfig(args)
	if err != nil {
		return err
	}

	store, err := NewLogStore(cfg.LogPath)
	if err != nil {
		return err
	}

	session := NewSession(cfg, store)
	if err := session.Start(); err != nil {
		return err
	}

	if strings.TrimSpace(cfg.Once) != "" {
		resp, err := session.HandleLine(cfg.Once)
		if err != nil {
			return err
		}
		if resp.Text != "" {
			fmt.Println(resp.Text)
		}
		return nil
	}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Printf("%s ", cfg.Prompt)
	for scanner.Scan() {
		resp, err := session.HandleLine(scanner.Text())
		if err != nil {
			return err
		}
		if resp.Text != "" {
			fmt.Println(resp.Text)
		}
		if resp.Exit {
			return nil
		}
		fmt.Printf("%s ", cfg.Prompt)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	return nil
}

func runLogs(args []string) error {
	cfg, err := parseLogsConfig(args)
	if err != nil {
		return err
	}

	store, err := NewLogStore(cfg.LogPath)
	if err != nil {
		return err
	}

	entries, err := store.Tail(cfg.Tail)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if cfg.JSON {
			fmt.Println(entry.JSON())
			continue
		}
		fmt.Println(entry.String())
	}
	return nil
}

func parseRunConfig(args []string) (Config, error) {
	modRoot, err := locateModRoot("")
	if err != nil {
		return Config{}, err
	}
	defaultLogPath := filepathJoin(modRoot, "runtime", "repl.log")

	fs := flag.NewFlagSet("repl v1 run", flag.ContinueOnError)
	name := fs.String("name", defaultPromptName(), "Session name")
	room := fs.String("room", "local", "Room name")
	prompt := fs.String("prompt", "repl>", "Prompt text")
	logPath := fs.String("log-file", defaultLogPath, "Append-only log file")
	once := fs.String("once", "", "Run a single line and exit")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	onceValue := strings.TrimSpace(*once)
	if onceValue == "" && fs.NArg() > 0 {
		onceValue = strings.Join(fs.Args(), " ")
	}

	return Config{
		Name:    strings.TrimSpace(*name),
		Room:    strings.TrimSpace(*room),
		Prompt:  strings.TrimSpace(*prompt),
		LogPath: strings.TrimSpace(*logPath),
		Once:    onceValue,
	}, nil
}

func parseLogsConfig(args []string) (LogsConfig, error) {
	modRoot, err := locateModRoot("")
	if err != nil {
		return LogsConfig{}, err
	}
	defaultLogPath := filepathJoin(modRoot, "runtime", "repl.log")

	fs := flag.NewFlagSet("repl v1 logs", flag.ContinueOnError)
	logPath := fs.String("log-file", defaultLogPath, "Log file to read")
	tail := fs.Int("tail", 20, "Number of lines to show")
	jsonOutput := fs.Bool("json", false, "Print raw JSON lines")
	if err := fs.Parse(args); err != nil {
		return LogsConfig{}, err
	}

	return LogsConfig{
		LogPath: strings.TrimSpace(*logPath),
		Tail:    *tail,
		JSON:    *jsonOutput,
	}, nil
}

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod repl v1 <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  run [--name NAME] [--room ROOM] [--log-file PATH] [--once TEXT]")
	fmt.Println("      Start the minimal local REPL and append session logs")
	fmt.Println("  logs [--log-file PATH] [--tail N] [--json]")
	fmt.Println("      Show saved REPL log entries")
	fmt.Println("  version")
	fmt.Println("      Print repl v1 version")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s error: %v\n", context, err)
	os.Exit(1)
}

package main

import (
	"fmt"
	"os"
	"strings"

	"dialtone/dev/plugins/gemini/src_v1/cmd/ops"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	version := "src_v1"
	if len(args) > 0 && strings.HasPrefix(args[0], "src_v") {
		version = args[0]
		args = args[1:]
	}
	if version != "src_v1" {
		fmt.Printf("Error: unsupported version %s\n", version)
		os.Exit(1)
	}

	switch command {
	case "run":
		taskFile := "TASK.md"
		model := "gemini-2.5-flash"
		prompt := ""
		for i := 0; i < len(args); i++ {
			if args[i] == "--task" && i+1 < len(args) {
				taskFile = args[i+1]
				i++
				continue
			}
			if args[i] == "--model" && i+1 < len(args) {
				model = args[i+1]
				i++
				continue
			}
			if args[i] == "--prompt" && i+1 < len(args) {
				prompt = args[i+1]
				i++
				continue
			}
		}
		if err := ops.Run(taskFile, model, prompt); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "doctor":
		if err := ops.Doctor(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: gemini <command> [src_v1] [args]")
	fmt.Println("  run [--task <file>] [--model <name>] [--prompt <text>]  Run Gemini CLI on a task file")
	fmt.Println("  doctor                               Check Gemini CLI/auth prerequisites")
}

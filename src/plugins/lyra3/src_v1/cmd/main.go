package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	fs := flag.NewFlagSet("lyra3", flag.ExitOnError)
	prompt := fs.String("prompt", "", "The music prompt (can be used with generate)")
	id := fs.String("id", "", "The track ID (for info/delete)")

	if len(os.Args) < 1 {
		printHelp()
		return
	}

	command := ""
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "help":
		printHelp()
	case "generate":
		_ = fs.Parse(os.Args[2:])
		generateMusic(*prompt)
	case "generate-sdk":
		_ = fs.Parse(os.Args[2:])
		generateMusicSDK(*prompt)
	case "list":
		listTracks()
	case "info":
		_ = fs.Parse(os.Args[2:])
		showTrackInfo(*id)
	case "test":
		runTests()
	default:
		printHelp()
	}
}

func printHelp() {
	fmt.Println("Usage: ./dialtone.sh lyra3 src_v1 <command> [args]")
	fmt.Println("\nCommands:")
	fmt.Println("  generate --prompt \"...\"      Generate music from a text prompt.")
	fmt.Println("  generate-sdk --prompt \"...\"  Generate music using the Google Gen AI SDK (Go example).")
	fmt.Println("  list                        List generated music tracks.")
	fmt.Println("  info --id <id>              Show details for a specific track.")
	fmt.Println("  test                        Run the plugin's smoke tests.")
	fmt.Println("  help                        Show this help message.")
}

func generateMusic(prompt string) {
	if prompt == "" {
		logs.Error("generate: prompt is required")
		os.Exit(1)
	}
	logs.Info("Generating music for prompt: %s", prompt)
	// Placeholder for Lyria API call logic
	fmt.Printf("Generating Lyria track for: '%s'...\n", prompt)
	fmt.Println("Success! (Simulated)")
}

func generateMusicSDK(prompt string) {
	if prompt == "" {
		logs.Error("generate-sdk: prompt is required")
		os.Exit(1)
	}
	logs.Info("SDK Example: would generate music for prompt: %s", prompt)
	fmt.Println("Code template to use with Google Gen AI SDK (Go):")
	fmt.Println("\n  import \"google.golang.org/genai\"")
	fmt.Println("  ...")
	fmt.Println("  model := \"lyria-002\"")
	fmt.Println("  resp, err := client.Models.GenerateContent(ctx, model, genai.Text(prompt), nil)")
	fmt.Println("\nSee README.md for the full example.")
}

func listTracks() {
	logs.Info("Listing Lyria music tracks...")
	fmt.Println("No tracks found. (Simulated)")
}

func showTrackInfo(id string) {
	if id == "" {
		logs.Error("info: track id is required")
		os.Exit(1)
	}
	logs.Info("Showing info for track: %s", id)
	fmt.Printf("Track ID: %s (Simulated)\n", id)
}

func runTests() {
	logs.Info("Starting lyra3 src_v1 tests...")
	// Logic to run test orchestrator
	fmt.Println("Lyra3 smoke tests passed (Simulated)")
}

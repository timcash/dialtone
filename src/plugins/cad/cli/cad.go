package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"dialtone/cli/src/plugins/cad/app"
)

// RunCad handles the 'cad' command
func RunCad(args []string) {
	if len(args) < 1 {
		printUsage()
		return
	}

	command := args[0]
	switch command {
	case "server":
		runServer()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown cad command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: dialtone-dev cad <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  server    Start the CAD backend server")
	fmt.Println("  help      Show this help message")
}

func runServer() {
	fmt.Println("[cad] Starting backend server on :8081...")
	
	http.HandleFunc("/api/cad", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*") // For local dev
		
		gear := app.NewGearObject(12, 5.0, 1.0)
		json.NewEncoder(w).Encode(gear)
	})

	err := http.ListenAndServe(":8081", nil)
	if err != nil {
		fmt.Printf("[cad] Server failed: %v\n", err)
		os.Exit(1)
	}
}

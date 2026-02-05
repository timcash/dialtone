package cli

import (
	"dialtone/cli/src/plugins/cad/app"
	"fmt"
	"net/http"
	"os"
	"os/exec"
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
	case "test":
		runTest()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown cad command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh cad <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  server    Start the CAD backend server")
	fmt.Println("  test      Run CAD plugin tests")
	fmt.Println("  help      Show this help message")
}

func runServer() {
	fmt.Println("[cad] Starting Go backend server on :8081...")

	mux := http.NewServeMux()

	// Register CAD handlers
	app.RegisterHandlers(mux)

	// Add CORS middleware
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			return
		}

		fmt.Printf("[cad] Handling %s %s\n", r.Method, r.URL.Path)
		mux.ServeHTTP(w, r)
	})

	err := http.ListenAndServe(":8081", handler)
	if err != nil {
		fmt.Printf("[cad] Server failed: %v\n", err)
		os.Exit(1)
	}
}

func runTest() {
	fmt.Println("[cad] Running plugin tests...")
	cmd := exec.Command("./dialtone.sh", "test", "plugin", "cad")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("[cad] Tests failed: %v\n", err)
		os.Exit(1)
	}
}

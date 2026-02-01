package cli

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"time"
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
	fmt.Println("Usage: dialtone-dev cad <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  server    Start the CAD backend server")
	fmt.Println("  test      Run CAD plugin tests")
	fmt.Println("  help      Show this help message")
}

func runServer() {
	// 1. Start Python Backend
	fmt.Println("[cad] Starting Python backend via pixi...")
	pythonCmd := exec.Command("pixi", "run", "python", "main.py")
	pythonCmd.Dir = "src/plugins/cad/backend"
	pythonCmd.Stdout = os.Stdout
	pythonCmd.Stderr = os.Stderr
	if err := pythonCmd.Start(); err != nil {
		fmt.Printf("[cad] Failed to start Python backend: %v\n", err)
		os.Exit(1)
	}
	defer pythonCmd.Process.Kill()

	// 2. Wait for Python backend
	time.Sleep(2 * time.Second)

	// 3. Setup Proxy
	pythonURL, _ := url.Parse("http://127.0.0.1:8082")
	proxy := httputil.NewSingleHostReverseProxy(pythonURL)

	fmt.Println("[cad] Starting Go proxy server on :8081...")
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		
		if r.Method == "OPTIONS" {
			return
		}
		
		fmt.Printf("[cad] Proxying %s %s\n", r.Method, r.URL.Path)
		proxy.ServeHTTP(w, r)
	})

	err := http.ListenAndServe(":8081", nil)
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

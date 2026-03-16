package chromev4

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strings"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// RunDaemon starts the background daemon holding the persistent MCP connection
// to Chrome and listens on NATS for agent commands.
func RunDaemon(mcpPort int) error {
	log.Printf("Starting Chrome v4 Daemon (Native MCP) on port %d...", mcpPort)
	log.Println("Initializing NATS proxy...")

	// Start embedded NATS server on a fixed port for v4 (e.g., 9334)
	natsPort := 9334
	opts := &natsserver.Options{Host: "127.0.0.1", Port: natsPort}
	ns, err := natsserver.NewServer(opts)
	if err != nil {
		return fmt.Errorf("failed to create nats server: %v", err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(10 * time.Second) {
		return fmt.Errorf("nats server failed to start")
	}

	nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", natsPort))
	if err != nil {
		return fmt.Errorf("failed to connect to nats: %v", err)
	}
	defer nc.Close()

	// Launch Chrome (mocking MCP flags as Chrome 146 is targeted)
	chromePath := findChromePath()
	if chromePath != "" {
		log.Printf("Launching Chrome from %s", chromePath)
		cmd := exec.Command(chromePath,
			"--remote-debugging-port="+fmt.Sprintf("%d", mcpPort),
			"--enable-features=ModelContextProtocol", // Mock flag for Chrome 146 MCP
			"--user-data-dir=/tmp/dialtone_chrome_v4_data",
			"--no-first-run",
			"--no-default-browser-check",
		)
		err = cmd.Start()
		if err != nil {
			log.Printf("Warning: Failed to launch Chrome: %v", err)
		} else {
			log.Printf("Chrome launched with PID %d", cmd.Process.Pid)
			defer cmd.Process.Kill()
		}
	} else {
		log.Printf("Warning: Chrome executable not found. Proceeding with mock browser.")
	}

	client := NewCDPClient(mcpPort)

	// Poll for Chrome to spin up and bind its debug port
	connected := false
	for i := 0; i < 15; i++ {
		if err := client.Connect(); err == nil {
			connected = true
			log.Println("Connected to Chrome WebSocket successfully!")
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !connected {
		log.Printf("Warning: Failed to connect to Chrome WebSocket after retries")
	}
	// Listen on NATS for `chrome.cmd.>` subjects
	_, err = nc.Subscribe("chrome.v4.cmd.>", func(m *nats.Msg) {
		subjectParts := strings.Split(m.Subject, ".")
		if len(subjectParts) < 4 {
			return
		}
		cmd := subjectParts[3]
		log.Printf("Received command %s: %s", cmd, string(m.Data))

		var payload map[string]any
		if err := json.Unmarshal(m.Data, &payload); err != nil {
			log.Printf("Failed to parse payload: %v", err)
			m.Respond([]byte(`{"status":"error","error":"invalid payload"}`))
			return
		}

		// Handle commands
		resp := map[string]any{"status": "success", "command": cmd}
		switch cmd {
		case "goto":
			url, _ := payload["url"].(string)
			log.Printf("Navigating to %s", url)
			if err := client.Navigate(url); err != nil {
				log.Printf("Navigation error: %v", err)
				resp["status"] = "error"
				resp["error"] = err.Error()
			} else {
				resp["url"] = url
			}
		case "mcp_call":
			tool, _ := payload["tool"].(string)
			args, _ := payload["args"].([]any)
			log.Printf("Calling tool %s with args %v", tool, args)

			// Build a JavaScript payload to invoke the tool via the new WebMCP API
			argsJSON, _ := json.Marshal(args)
			js := fmt.Sprintf(`
					(async () => {
						if (!navigator.modelContext) {
							throw new Error("WebMCP not supported on this page (navigator.modelContext missing)");
						}
						// The interface expects a tool name and parameters object
						return await navigator.modelContext.callTool({
							name: %q,
							parameters: %s
						});
					})()
				`, tool, string(argsJSON))

			result, err := client.Eval(js)
			if err != nil {
				log.Printf("MCP Call error: %v", err)
				resp["status"] = "error"
				resp["error"] = err.Error()
			} else {
				resp["tool"] = tool
				// result is a JSON representation of { result: { type: "object", value: ... } }
				var rawResult map[string]any
				if err := json.Unmarshal(result, &rawResult); err == nil {
					if ret, ok := rawResult["result"].(map[string]any); ok {
						resp["result"] = ret["value"]
					} else {
						resp["result"] = string(result)
					}
				}
			}
		default:
			resp["status"] = "error"
			resp["error"] = "unknown command"
		}

		respData, _ := json.Marshal(resp)
		m.Respond(respData)
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe to nats: %v", err)
	}

	log.Println("Daemon running. Press Ctrl+C to exit.")
	select {}
}

func findChromePath() string {
	switch runtime.GOOS {
	case "darwin":
		return "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	case "linux":
		return "/usr/bin/google-chrome"
	case "windows":
		return "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe"
	}
	return ""
}

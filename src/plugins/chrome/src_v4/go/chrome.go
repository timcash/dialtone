package chromev4

import (
	"encoding/json"
	"fmt"
	"log"
)

// ExecuteCommand publishes a command via NATS to the running Chrome daemon.
func ExecuteCommand(cmd string, payload map[string]any) error {
	data, _ := json.Marshal(payload)
	log.Printf("Publishing NATS message: chrome.cmd.%s -> %s", cmd, string(data))
	
	// TODO: Connect to NATS, publish message, wait for response
	fmt.Printf("[Mock] Executed %s successfully.\n", cmd)
	return nil
}

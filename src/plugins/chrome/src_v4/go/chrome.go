package chromev4

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

// ExecuteCommand publishes a command via NATS to the running Chrome daemon.
func ExecuteCommand(cmd string, payload map[string]any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	log.Printf("Publishing NATS message: chrome.v4.cmd.%s -> %s", cmd, string(data))

	// Connect to NATS on the hardcoded port used by the daemon
	natsPort := 9334
	nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", natsPort), nats.Timeout(2*time.Second))
	if err != nil {
		return fmt.Errorf("failed to connect to NATS daemon proxy (is the daemon running?): %w", err)
	}
	defer nc.Close()

	subject := fmt.Sprintf("chrome.v4.cmd.%s", cmd)
	msg, err := nc.Request(subject, data, 5*time.Second)
	if err != nil {
		return fmt.Errorf("NATS request failed: %w", err)
	}

	fmt.Printf("[Success] Response: %s\n", string(msg.Data))
	return nil
}

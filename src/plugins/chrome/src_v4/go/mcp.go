package chromev4

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// MCPRequest represents a standard JSON-RPC 2.0 request.
type MCPRequest struct {
	ID     int64  `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

// MCPResponse represents a standard JSON-RPC 2.0 response.
type MCPResponse struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *MCPError       `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type CDPClient struct {
	conn       *websocket.Conn
	mu         sync.Mutex
	msgID      int64
	pending    map[int64]chan *MCPResponse
	debugPort  int
	targetURL  string
}

func NewCDPClient(debugPort int) *CDPClient {
	return &CDPClient{
		pending:   make(map[int64]chan *MCPResponse),
		debugPort: debugPort,
	}
}

// Connect fetches the active page's WebSocket debugger URL and connects to it.
func (c *CDPClient) Connect() error {
	// 1. Fetch available targets (tabs)
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json", c.debugPort))
	if err != nil {
		return fmt.Errorf("failed to fetch devtools targets: %w", err)
	}
	defer resp.Body.Close()

	var targets []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return fmt.Errorf("failed to decode targets: %w", err)
	}

	if len(targets) == 0 {
		return fmt.Errorf("no browser targets found")
	}

	// Find a "page" target
	var wsURL string
	for _, t := range targets {
		if t["type"] == "page" {
			if ws, ok := t["webSocketDebuggerUrl"].(string); ok {
				wsURL = ws
				break
			}
		}
	}

	if wsURL == "" {
		return fmt.Errorf("no valid page target found")
	}

	log.Printf("Connecting to Chrome WebSocket: %s", wsURL)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	c.conn = conn
	go c.listen()

	return nil
}

// listen reads incoming messages from the WebSocket and routes them to the correct pending request.
func (c *CDPClient) listen() {
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			return
		}

		var resp MCPResponse
		if err := json.Unmarshal(msg, &resp); err != nil {
			// Some messages are events, we ignore them for now
			continue
		}

		if resp.ID > 0 {
			c.mu.Lock()
			ch, ok := c.pending[resp.ID]
			if ok {
				delete(c.pending, resp.ID)
			}
			c.mu.Unlock()

			if ok {
				ch <- &resp
			}
		}
	}
}

// Call sends a JSON-RPC command over the WebSocket and waits for a response.
func (c *CDPClient) Call(method string, params any) ([]byte, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	id := atomic.AddInt64(&c.msgID, 1)
	req := MCPRequest{
		ID:     id,
		Method: method,
		Params: params,
	}

	ch := make(chan *MCPResponse, 1)

	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	if err := c.conn.WriteJSON(req); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("write error: %w", err)
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("RPC error: %d %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-time.After(10 * time.Second):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("timeout waiting for response")
	}
}

// Navigate tells the connected browser page to navigate to a URL.
func (c *CDPClient) Navigate(url string) error {
	_, err := c.Call("Page.navigate", map[string]any{"url": url})
	return err
}

// Eval executes arbitrary JavaScript in the page context. We use this to bridge into navigator.modelContext
func (c *CDPClient) Eval(expression string) ([]byte, error) {
	result, err := c.Call("Runtime.evaluate", map[string]any{
		"expression":    expression,
		"returnByValue": true,
		"awaitPromise":  true,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

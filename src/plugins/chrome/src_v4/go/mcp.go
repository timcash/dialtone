package chromev4

// MCPRequest represents a standard JSON-RPC 2.0 request used by the Model Context Protocol.
type MCPRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// MCPResponse represents a standard JSON-RPC 2.0 response from an MCP Server.
type MCPResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
}

// TODO: Implement lightweight JSON-RPC over WebSocket client to communicate 
// with Chrome 146+ native MCP server.

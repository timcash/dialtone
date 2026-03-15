# Chrome Plugin v4 (src_v4) Migration Plan

## 1. Overview
The goal of the `chrome` plugin `src_v4` is to modernize Dialtone's browser automation capabilities by leveraging the newly introduced **native Model Context Protocol (MCP)** support in Chrome 146 and the emerging **WebMCP** standard.

By connecting directly to Chrome's native MCP server, Dialtone agents will be able to bypass traditional heavy automation frameworks (like Selenium or raw CDP screen scraping) to interact with websites through structured, token-efficient tool calls.

## 2. Core Objectives
- **Direct MCP Integration:** Build a Go-based client to connect to Chrome's native MCP interface via the new debugging endpoints.
- **WebMCP Support:** Implement support for both the Declarative API (HTML attributes like `toolname` and `tooldescription`) and Imperative API (`navigator.modelContext`).
- **Token Efficiency & Speed:** Reduce reliance on full DOM tree parsing. Use structured contracts exposed by modern web pages.
- **Graceful Fallback:** Maintain CDP-based fallback mechanisms (similar to Stagehand v3) for legacy sites that do not yet implement WebMCP.

## 3. Architecture: Direct Connection vs. NATS Daemon Proxy

When exposing CLI controls (e.g., `./dialtone.sh chrome src_v4 goto <url>`), there are two ways to route commands to the browser. The `src_v4` plugin will adopt the **NATS Daemon Proxy** model for the following reasons:

### Approach A: Direct Connection (CLI -> Browser)
The CLI script directly connects to Chrome's debugging port, sends the instruction, and disconnects.
- **Pros:** Simpler architecture; stateless.
- **Cons:** High latency (re-establishing WebSockets for every single CLI command like a keystroke or click), state loss between commands, and severe race conditions if multiple agents attempt to control the browser concurrently.

### Approach B: NATS Daemon Proxy (CLI -> NATS -> Daemon -> Browser) [Selected]
A background daemon (`dialtone-chrome-daemon`) holds a permanent, persistent connection to the browser. CLI commands are published to a NATS topic, executed instantly by the daemon, and results are returned.
- **Performance:** Extremely fast multi-step interactions because the underlying WebSocket/MCP connection is already established.
- **Concurrency & Safety:** The daemon acts as a traffic cop, queuing commands and preventing concurrent AI scripts from corrupting the browser state.
- **Mesh Capabilities:** NATS allows a user to run `./dialtone.sh` locally but target a Chrome daemon running on a remote server/node.
- **Passive Monitoring:** The daemon can stream real-time browser events (DOM mutations, console logs) over NATS for tools like `dialtone-tap` to monitor without interfering.

## 4. Architecture: CDP (`chromedp`) vs. Model Context Protocol (MCP)

Currently, Dialtone's `chrome src_v3` relies on the Chrome DevTools Protocol (CDP) using the `chromedp` Go library. Moving to `src_v4` with MCP represents a necessary paradigm shift for agentic automation. Based on the 2026 browser automation landscape, here is a verified point-by-point comparison:

### 1. Level of Abstraction
*   **CDP (`chromedp`):** Extremely low-level. Exposes hundreds of granular methods (e.g., `DOM.dispatchMouseEvent`, coordinate-based clicks). It was designed for debugging and deterministic QA testing, not for AI.
*   **MCP:** High-level and task-oriented. It abstracts complex DOM interactions into curated, semantic "tools" (e.g., `navigate_to`, `click_button`).

### 2. Token Efficiency & Context Window
*   **CDP (`chromedp`):** Requires "Screen Scraping." To know what to do, an AI must ingest a massive, noisy representation of the visual DOM or Accessibility Tree. This wastes thousands of context tokens and increases latency.
*   **MCP:** The browser (acting as an MCP Server) only passes the available *functions* (tools) and their JSON schemas to the LLM. The AI doesn't need to parse the entire HTML document to figure out how to book a flight; it just calls the exposed `book_flight` tool.

### 3. Implementation Complexity in Go
*   **CDP (`chromedp`):** A massive dependency. It requires managing complex Go contexts, remote allocators, and parsing thousands of lines of auto-generated CDP protocol types. The agent is forced to "guess" brittle CSS/XPath selectors.
*   **MCP:** A very thin client. Because MCP is built on standard JSON-RPC 2.0, the Go proxy in `src_v4` can drop the heavy `chromedp` dependency. It will use standard WebSockets to act as a translation layer, passing NATS messages (`chrome.call_tool`) to the browser's MCP endpoint.

### How They Work Together in `src_v4`
They are not necessarily mutually exclusive at the system level. A Browser MCP Server often uses CDP under the hood to actually execute the commands. However, the critical architectural shift in `src_v4` is moving the **Dialtone Agent interface** up to MCP. The agent speaks high-level MCP over NATS, while the daemon/browser handles the low-level CDP execution internally, shielding the AI from the brittle complexities of the DOM.

## 5. Architecture & Implementation Steps

### Phase 1: Scaffolding & Setup [COMPLETED]
- **Directory Structure:** Create `dialtone/src/plugins/chrome/src_v4`.
- **CLI & Entrypoints:** Set up the standard Dialtone plugin scaffolding (`cli.go`, `scaffold/main.go` logic via standard routing).
- **Dependencies:** Evaluate and import necessary Go libraries for connecting to standard MCP servers over WebSockets/stdio, adapting them to connect to Chrome's specific implementation.

### Phase 2: Chrome MCP Connection & Daemon Proxy
- **Daemon Proxy:** Develop a persistent background daemon that manages the lifecycle of the Chrome instance. This proxy will maintain persistent MCP and CDP connections to the browser and expose them safely (e.g. over NATS) to transient AI agents, avoiding the overhead of starting/stopping Chrome for each task.
- **Session Management:** Update the Chrome launch parameters to ensure the new remote debugging/MCP flags are enabled.
- **Protocol Implementation:** Implement a client that handshakes with the Chrome 146 MCP endpoint.
- **Context Extraction:** Create methods to query the browser for available WebMCP tools on the current page.

### Phase 3: Tool Execution (WebMCP)
- **Declarative Actions:** Implement logic to parse and trigger HTML elements that advertise themselves via `toolname` attributes.
- **Imperative Actions:** Create a bridge to call JavaScript functions exposed on `navigator.modelContext`.
- **Schema Validation:** Ensure inputs provided by the Dialtone agent match the schemas defined by the WebMCP tools before execution.

### Phase 4: Legacy Fallback (CDP Direct)
- For sites lacking WebMCP, implement an optimized direct-CDP approach (similar to the new Stagehand v3 philosophy) that bypasses traditional automation frameworks to provide faster DOM observation and accessibility tree extraction.

### Phase 5: Testing & Security
- **Security:** Implement strict origin checks and prompt injection mitigations, acknowledging the risks of native agent access to sensitive browser tabs.
- **Test Suite:** Create tests under `src_v4/test` against:
  - A mock WebMCP-enabled local server.
  - A legacy site requiring CDP fallback.

## 6. Plugin Creation Methodology (Based on `dialtone-tap`)

When implementing the new `src_v4` plugin, we will follow the standardized Dialtone plugin scaffold pattern used successfully to integrate the standalone `dialtone-tap` utility.

1. **Scaffold Entrypoint (`scaffold/main.go`)**
   Every plugin requires an entrypoint to be discoverable by the `dialtone.sh` router. We will create `src/plugins/chrome/scaffold/main.go`. This file acts as a simple wrapper that intercepts `dialtone.sh chrome src_v4 ...` and hands off execution to the CLI parser.

2. **CLI Parser (`src_v4/cli/cli.go`)**
   This package defines the `Run(args []string)` function. It is responsible for:
   - Ensuring the version string (`src_v1`, `src_v4`, etc.) is correctly processed and ignored for subsequent flag parsing.
   - Defining all the subcommands (e.g., `goto`, `click`, `daemon`) using `flag.NewFlagSet`.
   - Marshalling the CLI arguments into structured configuration passed to the core logic layer.

3. **Core Logic (`src_v4/go/`)**
   This is where the actual implementation lives (e.g., `chrome.go`, `daemon.go`).
   - The CLI layer will call functions like `chromev4.RunDaemon(cfg)` or `chromev4.ExecuteCommand("goto", "https...")`.
   - By separating the CLI from the logic, the core functionality can be easily unit-tested or imported by other Go components within the mesh without dealing with string arguments.

4. **Integration with `dialtone.sh`**
   Because Dialtone automatically scans for `scaffold/main.go` directories, once these files are written and compile successfully (`go build ./scaffold/main.go`), the command `./dialtone.sh chrome src_v4 --help` will immediately begin routing traffic to the new implementation.

## 7. Required Flags & Configuration
*When launching Chrome 146+ via Dialtone:*
- Ensure standard remote debugging is active.
- Discover and implement the specific Chrome flags required to activate the native MCP server (e.g., potential experimental flags if WebMCP is still behind a flag in standard releases).

## 8. Test Plan

To ensure the new `src_v4` plugin is robust and correctly handles both the new WebMCP standard and legacy sites, a comprehensive test suite will be created under `src_v4/test`.

### 1. WebMCP Compliance Tests
- **Declarative WebMCP:** Launch a local mock server serving a page with HTML elements annotated with `toolname` and `tooldescription`. Verify the Go proxy can successfully list these tools via `tools/list` and trigger them via `tools/call`.
- **Imperative WebMCP:** Launch a mock server exposing a complex JavaScript function via `navigator.modelContext`. Verify the proxy can execute the function with a structured JSON payload and return the correct result.

### 2. NATS Integration Tests
- **Daemon Lifecycle:** Verify the daemon can successfully start Chrome, establish the WebSocket connection, and cleanly shut down both Chrome and itself upon receiving a termination signal over NATS.
- **Message Translation:** Send mock `chrome.cmd.goto` and `chrome.cmd.mcp_call` NATS messages. Verify the daemon translates them correctly into JSON-RPC and returns the expected NATS response.

### 3. Fallback & Edge Case Tests
- **Legacy CDP Fallback:** Navigate to a standard, non-WebMCP website. Verify the proxy can fall back to basic DOM interaction (e.g., extracting an accessibility tree or performing a raw click) without crashing.
- **Concurrent Agent Traffic:** Flood the NATS proxy with simultaneous requests from multiple mocked agents to ensure the daemon properly queues actions and prevents browser state corruption.
- **Prompt Injection/Security:** Attempt to pass malicious Javascript payloads through the `mcp-call` arguments to verify validation and sandboxing.

## 9. Rollout Strategy
1. Keep `src_v3` active as the stable default.
2. Develop `src_v4` in parallel.
3. Expose `dialtone chrome src_v4` commands for opt-in testing by AI agents.
4. Graduate to default once Chrome 146 adoption is ubiquitous and stable.
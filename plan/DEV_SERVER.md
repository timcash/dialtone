# Generalized Dev Server Architecture

## Coordination Outline (Tailscale + NATS)

To coordinate a distributed development environment across multiple machines (e.g., Robot, Workstation, Tablet), we use a two-layer control plane:

1.  **Network Layer (Tailscale/tsnet)**:
    *   **Identity**: Each node (Backend, Dev Server, etc.) runs an embedded Tailscale node using `tsnet`.
    *   **Access**: Nodes reach each other via stable hostnames (e.g., `http://drone-1:8080`) without manual port forwarding or system-wide VPN configuration.
    *   **Security**: All traffic is encrypted end-to-end between nodes on the private Tailnet.

2.  **Messaging Layer (NATS)**:
    *   **Unified Bus**: Every node connects to a shared NATS cluster (local or remote).
    *   **Event Streaming**: The Backend publishes telemetry; the UI publishes "thoughts" and state changes; the Test Node listens for specific conditions to proceed.
    *   **Remote Control**: The `repl` plugin uses NATS subjects to execute commands on remote nodes and stream output back to the developer's console.

---

## Core Components

### 1. Backend Node (The "Robot" or Service)
*   **Role**: Runs the core logic, hardware interfaces, or heavy database services.
*   **Connectivity**: 
    *   Registers as a **Tailscale** node (via `tsnet`) to be reachable by a stable hostname.
    *   Connects to a **NATS** bus for real-time telemetry and command execution.
    *   Exposes a local API (e.g., `:8080`) that can be reached via Tailscale or forwarded over SSH.

### 2. Dev Server Node (The "Frontend")
*   **Role**: Runs the **Vite** development server with Hot Module Replacement (HMR).
*   **Workflow**:
    *   Proxies API requests (`/api/*`) to the Backend Node (either via Tailscale IP/hostname or a local SSH tunnel).
    *   Serves the UI to the local network or Tailscale network.
*   **Command**: Typically `./dialtone.sh <plugin> dev`.

### 3. Browser Node (The "View")
*   **Role**: Displays the application.
*   **Configuration**:
    *   Starts Chrome with `--remote-debugging-port=9222`.
    *   Can be the same machine as the Dev Server or a dedicated tablet/display.
    *   Writes `dev.browser.json` containing the WebSocket Debug URL for automation tools to attach.

### 4. Test/Automation Node (The "Controller")
*   **Role**: Runs `chromedp` or other automation scripts to drive the Browser Node.
*   **Connectivity**:
    *   Connects to the Browser Node's debug port (`9222`).
    *   If the browser is on a different machine, a bridge (SSH tunnel or TCP proxy) is used to expose `:9222` locally.
*   **Command**: `./dialtone.sh <plugin> test --attach`.

---

## Connectivity Infrastructure

### NATS (The Message Bus)
All nodes in the environment should ideally connect to a shared NATS cluster. This allows:
*   **Unified Logging**: Dev logs, test results, and backend telemetry streamed to a single subscriber.
*   **Cross-Process Synchronization**: Tests can wait for specific backend events or "thoughts" published by the UI.
*   **REPL Integration**: Remote execution of commands on any node via the `repl` plugin.

### Tailscale / tsnet (The Network)
`tsnet` allows any Dialtone process to become its own identity on the Tailnet without requiring system-level VPN installation.
*   **Stable Identity**: A robot at a remote site and a dev laptop at home can see each other as `drone-1` and `macbook-dev`.
*   **Zero-Config Tunnels**: Replaces complex SSH manual forwarding with simple hostname-based access.

### SSH Port Forwarding (The Legacy Bridge)
Used primarily for tools that expect local TCP access (like the Chrome DevTools Protocol):
*   `ssh -L 9222:localhost:9222 user@browser-node` allows a local `chromedp` script to control a remote browser.

---

## Tailscale Authentication & Connectivity

The `tsnet` plugin manages connectivity using two types of credentials from Tailscale:

### 1. Auth Key (`TS_AUTHKEY`)
*   **Purpose**: Used by an individual node (e.g., a Robot or Dev Server) to join the Tailnet.
*   **Usage**: When you run `./dialtone.sh tsnet up` or a plugin that uses `tsnet`, it looks for this key to authenticate the node.
*   **Ephemeral Hosts**: By default, nodes in Dialtone are marked as **ephemeral**. This means if the process stops, the node automatically disappears from your Tailscale admin console after a short period. This prevents "node bloat" from repeated development restarts.
*   **One Key Per Host?**: No. A single **reusable** auth key can be shared across all your development machines. However, for security, it is recommended to use an auth key with specific **tags** (e.g., `tag:dev`) so that ACLs can restrict access.

### 2. API Key (`TS_API_KEY`)
*   **Purpose**: Used by the Dialtone CLI to *manage* your Tailnet (e.g., creating new auth keys, listing devices, or pruning old nodes).
*   **Usage**: Required for commands like `./dialtone.sh tsnet keys provision` or `./dialtone.sh tsnet devices list`.
*   **Auto-Provisioning**: If you have a `TS_API_KEY` set but no `TS_AUTHKEY`, Dialtone can automatically provision a temporary, 24-hour ephemeral auth key for you when you start a node.

### Summary Table

| Credential | Scope | Key Task | Permanent? |
| :--- | :--- | :--- | :--- |
| **Auth Key** | Node-level | Joins the network | No (usually ephemeral) |
| **API Key** | Tailnet-level | Manages keys/devices | Yes (long-lived) |

---

## Hybrid Backend Management

A key pattern from the `robot` plugin is the **Dynamic Backend Switcher**. This allows a developer to work even when the remote hardware is offline.

1.  **Remote Mode**: Attempts to connect to the physical hardware (e.g., via SSH tunnel or Tailscale).
2.  **Mock Mode**: If the remote connection fails or is disabled, the system spawns a local "mock" backend process that emulates the API.
3.  **Automatic Recovery**: The dev server periodically probes the remote hardware and automatically swaps the proxy target back to "Remote" once it becomes available again.

This ensures a "batteries-included" development experience where the frontend remains functional regardless of backend availability.

## Metadata & Discovery

For integration between different tools (e.g., CLI, Browser, Test Runner), the following files are used for discovery:

*   **`dev.browser.json`**: Contains the `websocket_url` and `debug_port` of the active browser session.
*   **NATS Subject `logs.dev`**: A common stream where all dev-related events are published, allowing a global `repl` session to monitor the entire stack.

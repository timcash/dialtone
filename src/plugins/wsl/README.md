# WSL Plugin

A robust Go-based plugin for managing WSL 2 instances with a modern, section-based Web UI. This plugin migrates the original `wsl-tools` functionality into the Dialtone ecosystem, using Go for the backend and WebSocket-driven live updates.

## CLI Commands

Use the `./dialtone.sh` wrapper to manage the WSL plugin:

```bash
# 🛠️ Development: Start Go host and Vite UI
./dialtone.sh wsl dev src_v1

# 🏗️ Build: Compile UI assets
./dialtone.sh wsl build src_v1

# 💨 Smoke Test: Automated verification (Build + UI Test)
./dialtone.sh wsl smoke src_v1

# 🧹 Lint: Check Go and TypeScript standards
./dialtone.sh wsl lint
```

## Migration Strategy

The goal is to move as much logic as possible from the original Bun backend (`server.ts`) and PowerShell script (`wsl_tools.ps1`) into Go.

1.  **Backend (Go):** Handles HTTP API, WebSocket broadcasting, and process orchestration. It wraps `wsl.exe` calls and specific PowerShell helpers for complex tasks (like Task Scheduler integration).
2.  **Frontend (Vite/TS):** Uses the `nix` plugin's component-based architecture.
3.  **Real-time:** Replaces the Bun-based polling/streaming with a Go-based WebSocket hub.

## Smoke Test Strategy

The `smoke.go` test suite builds up the stack layer by layer:

1.  **Level 1: Backend API Health:** Verify the Go server starts and responds to `/api/status`.
2.  **Level 2: UI Asset Delivery:** Ensure the frontend is built and served correctly by the Go backend.
3.  **Level 3: Component Mounting:** Navigate to `#wsl-table` and verify ARIA labels are present.
4.  **Level 4: Instance Provisioning:** Click "Spawn Node", verify the POST request to Go, and wait for the `wsl.exe --import` to complete.
5.  **Level 5: WebSocket Telemetry:** Verify that once an instance is "Running", the UI receives memory/disk stats over WebSocket and updates the table row.
6.  **Level 6: Lifecycle Actions:** Test "Stop" and "Delete" actions via the UI, verifying both state changes in the DOM and actual system state.

## Technical Challenges

-   **UTF-16LE Output:** WSL output often needs normalization (handled in Go by stripping null bytes).
-   **Concurrency:** Go's `sync.Mutex` and channels will manage multiple simultaneous WSL operations.
-   **Persistence:** Integration with Windows Task Scheduler via PowerShell will be wrapped by Go CLI calls.

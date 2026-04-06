# WSL Plugin

A robust Go-based plugin for managing WSL 2 instances with a modern, component-based Web UI. This plugin migrates the original `wsl-tools` functionality into the Dialtone ecosystem.

## 🚀 Status: src_v3 Modernization In Progress

We are currently migrating to `src_v3`, which aligns the plugin with the latest Dialtone standards (versioned scaffolds, shared `@ui` library, and `test_v2` harness).

### What Has Been Done (src_v3)
- [x] **Versioned Scaffold:** Implemented `scaffold/main.go` to route commands to `src_v3` specifically.
- [x] **Standardized Layout:** Migrated code into `cmd/server`, `cmd/ops`, `go/`, and `test/` subdirectories.
- [x] **Modern UI Library:** Rebuilt the UI using the shared `@ui` library, adopting `SectionManager` and `setupApp`.
- [x] **Layout Parity:** Implemented the "calculator" layout for the WSL Spreadsheet, integrating the standardized `mode-form` button system.
- [x] **Path Resolution:** Added `go/paths.go` for centralized, predictable path management.
- [x] **Cross-Platform CLI:** Synchronized `dialtone.ps1` with `dialtone.sh` and implemented cross-platform command execution in `ops`.
- [x] **Modern Test Harness:** Ported the `robot` plugin's test structure to `src_v3/test`, including preflight and section validation.

### What's In Progress / Not Working Yet
- [ ] **WSL Timeout Issues:** Debugging `HCS_E_CONNECTION_TIMEOUT` errors during instance creation on Windows hosts.
- [ ] **Test Validation:** Complete end-to-end run of the `src_v3` test suite (currently blocked by WSL timeouts).
- [ ] **Windows Task Scheduler Integration:** Migration of "Daemon" persistence logic natively into Go.
- [ ] **Advanced Telemetry:** CPU/Memory sparklines are still UI placeholders.
- [x] **Terminal Integration:** The table now exposes a one-click desktop terminal launcher that opens an interactive WSL shell with a short usage banner.

## CLI Commands

Use the `./dialtone.ps1` (Windows) or `./dialtone.sh` (Linux/WSL) wrapper:

```bash
# 🛠️ Development: Start host and UI in dev mode (Managed Debug Browser)
.\dialtone wsl src_v3 dev

# 🏗️ Build: Compile UI assets and Go server
.\dialtone wsl src_v3 build

# ▶ Start: Keep a distro running from the Windows host side
.\dialtone.ps1 wsl src_v3 start --name Ubuntu-24.04

# ⏹ Stop: Terminate a distro from the Windows host side
.\dialtone.ps1 wsl src_v3 stop --name Ubuntu-24.04

# 🖥️ Terminal: Start the distro if needed, open a desktop terminal attached to the shared tmux session in the WSL repo, and queue Chrome warmup
.\dialtone.ps1 wsl src_v3 terminal --name Ubuntu-24.04

# 🧭 Shared tmux session: commands sent here show up in that same visible terminal
.\dialtone.ps1 tmux status -Session dialtone -Distro Ubuntu-24.04 -Cwd /home/user/dialtone
.\dialtone.ps1 tmux send -Session dialtone -Distro Ubuntu-24.04 -Cwd /home/user/dialtone -- ./dialtone.sh cad src_v1 test

# 💨 Test: Modernized verification suite
.\dialtone wsl src_v3 test

# 🧹 Lint: Check Go and TypeScript standards
.\dialtone wsl src_v3 lint
```

## Technical Architecture

1. **Scaffold Routing:** The root `scaffold/main.go` provides a version-aware entrypoint for all plugin operations.
2. **Operations (Ops):** Logic for `install`, `build`, and `dev` is encapsulated in `src_v3/cmd/ops` for maintainability.
3. **Shared UI:** The frontend imports the Dialtone shared UI library via the `@ui` alias, ensuring consistent styling and lifecycle management.
4. **Direct NATS/WS:** (Future) Move from standard REST polling to the NATS-based telemetry pattern used in the robot plugin.

## Technical Architecture

1. **Orchestration:** Go `os/exec` wraps `wsl.exe`, Windows Terminal, and Windows `cmd.exe` fallbacks. The terminal flow now starts the distro, opens the shell in the WSL repo root, and queues the Chrome `legion/dev` service warmup before handing control to the user prompt.
2. **WebSocket Hub:** A centralized hub broadcasts state changes to all connected UI clients.
3. **Snappy UI:** The UI uses CSS mandatory scroll-snapping and `IntersectionObserver` to manage section-specific logic (pausing 3D visuals, toggling global chrome).

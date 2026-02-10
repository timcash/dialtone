# WSL Plugin

A robust Go-based plugin for managing WSL 2 instances with a modern, component-based Web UI. This plugin migrates the original `wsl-tools` functionality into the Dialtone ecosystem.

## 🚀 Status: MVP Complete

We have successfully migrated the core orchestration and visualization logic from the standalone Bun/PowerShell prototype into a production-grade Go plugin.

### What Works
- [x] **Go Backend:** High-performance HTTP and WebSocket server handles all WSL orchestration.
- [x] **Real-time Telemetry:** Live memory and disk usage updates delivered via WebSocket (no more polling).
- [x] **Component Architecture:** UI refactored into encapsulated TypeScript components (`Home`, `Table`, `Settings`) with proper mount/unmount lifecycles.
- [x] **3D Hero Visualization:** Hero section features a Three.js-powered glowing cluster visual.
- [x] **Robust Smoke Testing:**
    - Level 0: Verifies Go standards (Vet) and TypeScript standards (Lint).
    - Level 1: Verifies UI build integrity (Vite).
    - Level 2: Verifies REST API health and logic before browser tests.
    - Level 3: Automated UI verification using headed Chrome, capturing console logs and screenshots for every action.
- [x] **Snappy Navigation:** CSS scroll-snapping and intelligent header/menu hiding based on active section.
- [x] **Windows Security Compatibility:** `dialtone.ps1` automatically resolves relative dependency paths to absolute ones to satisfy Go security requirements.

### What's In Progress / Not Working Yet
- [ ] **Windows Task Scheduler Integration:** The "Daemon" persistence logic (Scheduled Tasks) is still handled by legacy PowerShell wrappers. We need to migrate this logic natively into Go.
- [ ] **WSL Distro Variety:** Currently optimized for Alpine Linux. Support for Ubuntu and other distros is planned.
- [ ] **Advanced Telemetry:** History sparklines for CPU/Memory are currently UI placeholders.
- [ ] **Terminal Integration:** One-click "Open Terminal" button is not yet implemented in the new UI.

## CLI Commands

Use the `./dialtone.cmd` (Windows) or `./dialtone.sh` (Linux/WSL) wrapper:

```bash
# 🛠️ Development: Start host and UI in dev mode (Headed Debug Browser)
.\dialtone wsl dev src_v1

# 🏗️ Build: Compile UI assets (TypeScript -> dist/)
.\dialtone wsl build src_v1

# 💨 Smoke Test: Full verification suite (Generates SMOKE.md)
.\dialtone wsl smoke src_v1

# 🧹 Lint: Check Go and TypeScript standards
.\dialtone wsl lint
```

## Technical Architecture

1. **Orchestration:** Go `os/exec` wraps `wsl.exe`. Output is sanitized (null-byte stripping) for reliable JSON parsing.
2. **WebSocket Hub:** A centralized hub broadcasts state changes to all connected UI clients.
3. **Snappy UI:** The UI uses CSS mandatory scroll-snapping and `IntersectionObserver` to manage section-specific logic (pausing 3D visuals, toggling global chrome).
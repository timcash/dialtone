# Nix Plugin

A robust Go-based plugin for managing Nix sub-processes with a modern, section-based Web UI.

## Features

- **Host Node:** Go backend that spawns and monitors Nix processes.
- **3D Visualization:** Hero section featuring an interactive Three.js cluster visualization.
- **Spreadsheet Manager:** Real-time dashboard to monitor status and logs of Nix sub-processes.
- **Automated Smoke Testing:** Full UI lifecycle verification using `chromedp` with detailed reports.
- **Nix Integration:** Leverages Nix for reproducible development environments (`flake.nix`).

## CLI Commands

### `dev <dir>`
Starts the Nix host and UI in development mode.
```bash
./dialtone.sh nix dev src_v1
```

### `smoke <dir> [--smoke-timeout <sec>]`
Runs automated UI tests and generates a `SMOKE.md` report with screenshots and logs.
```bash
./dialtone.sh nix smoke src_v1
```

### `lint`
Lints Go code (vet/fmt) and TypeScript code (eslint via Bun).
```bash
./dialtone.sh nix lint
```

## Architecture

### Backend (`src/plugins/nix/<version>/nix.go`)
- Implements a REST API for process management.
- Captures STDOUT/STDERR from sub-processes.
- Serves static UI assets from the `ui/dist` folder.

### Frontend (`src/plugins/nix/<version>/ui/`)
- **Vite + TypeScript:** Modern frontend build pipeline.
- **SectionManager:** Handles lazy-loading and scroll-snapping between views.
- **Three.js:** Powering the `s-viz` hero visualization.
- **Xterm.js:** (Ready for integration) Supporting terminal interactions.

## Development Workflow

To create a new version of the plugin:
1. Copy `src_v1` to `src_v2`.
2. Update the Go package name and imports.
3. Modify the UI in `ui/src/`.
4. Run `nix dev src_v2` to test.

---
*Note for LLMs: When modifying the UI, always follow the `SectionManager` pattern and ensure `aria-label` attributes are present on interactive elements for smoke test reliability.*

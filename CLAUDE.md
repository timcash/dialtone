# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Dialtone

Dialtone is a distributed civic coordination platform combining a Go backend, web frontend (TypeScript/Vite/Three.js), and a plugin-based architecture. It provides a CLI (`./dialtone.sh`) for managing robots, radios, maps, tickets, and deployments over a P2P mesh network (NATS + Tailscale + Hyperswarm).

## Common Commands

All project tasks go through the shell wrapper. Run commands one at a time (no `&&` or `;` chaining).

```bash
# Install dependencies
./dialtone.sh install

# Build
./dialtone.sh build                    # Build web UI and Go binary
./dialtone.sh plugin build <name>      # Build a specific plugin

# Start the server (NATS + Web)
./dialtone.sh start

# Testing
./dialtone.sh plugin test <name>       # Run plugin tests (e.g., www)
./dialtone.sh ticket next              # TDD loop: run tests for current subtask

# WWW development
./dialtone.sh www dev                  # Start Vite dev server (port 5173)
./dialtone.sh www build                # Build www assets
./dialtone.sh www publish              # Deploy to Vercel (dialtone.earth)

# Tickets (primary unit of work)
./dialtone.sh ticket start <name>      # Create ticket + git branch
./dialtone.sh ticket review <name>     # Prep-only review of ticket DB/subtasks
./dialtone.sh ticket ask <question>    # Ask clarifying question
./dialtone.sh ticket log <message>     # Log a note
./dialtone.sh ticket done              # Finalize ticket

# Set a subtask's test command
./dialtone.sh ticket subtask testcmd <subtask> ./dialtone.sh plugin test www

# Plugins
./dialtone.sh plugin add <name>        # Create new plugin
./dialtone.sh plugin install <name>    # Install plugin deps

# Logs and diagnostics
./dialtone.sh logs --lines 200
./dialtone.sh diagnostic

# GitHub
./dialtone.sh github pr                # Create pull request
./dialtone.sh branch <name>            # Create/checkout feature branch

# Deploy
./dialtone.sh deploy                   # Deploy to remote robot
```

## Architecture

### Entry points
- `dialtone.sh` — Bash wrapper: loads env, dispatches commands to Go binary or plugin CLIs
- `src/cmd/dialtone/` — Go `main` package, compiles to `bin/`
- `src/dialtone.go` — Go application: `start` (NATS server + web server + Tailscale VPN) and `vpn` commands

### Plugin system (`src/plugins/`)
23 plugins, each self-contained with `/cli` (shell commands) and `/app` (application code) subdirectories. Key plugins:
- `www` — Public website (Vite SPA, Three.js, D3, globe.gl). Dev server on port 5173
- `swarm` — P2P mesh networking (Hyperswarm/Pear)
- `cad` — Parametric CAD (Python/VTK backend)
- `ticket` — Ticket/task management with DuckDB storage per ticket
- `ai` — AI assistant integration
- `chrome` — Chromedp browser automation
- `test` — Test framework and registry

Plugin manager delegates via shell; plugins own their own lifecycle (install, build, test).

### Core libraries (`src/core/`)
- `browser/` — Chrome automation and cleanup
- `build/` — Build orchestration
- `config/` — Configuration loading
- `earth/` — Geospatial utilities
- `logger/` — Project logger (`dialtone.LogInfo`, `dialtone.LogError`, `dialtone.LogFatal`)
- `mock/` — Test data mocking
- `test/` — Test registry (custom, not Go's testing package)
- `web/` — Core web library (Vite 7, TypeScript, Three.js, nats.ws, xterm). Built output embedded in Go binary via `src/core/web/dist/`

### Ticket storage
Each ticket is a directory at `src/tickets/<ticket-name>/` with its own DuckDB file. The current ticket pointer lives at `src/tickets/.current_ticket`.

### Web frontend (`src/plugins/www/app/`)
- Vite SPA with snap-scroll sections (home, robot, neural, math, CAD, about, docs)
- Lazy-loaded Three.js sections via `SectionManager`
- `VisibilityMixin` pattern for pausing off-screen animations
- CAD API proxy configured in `vite.config.mjs`

## Code Style Rules

- **Linear pipelines**: Avoid nested "pyramid" code. Keep the main execution path on the left margin with early returns.
- **Logging**: Always use `dialtone.LogInfo`, `dialtone.LogError`, `dialtone.LogFatal` from `src/logger.go`.
- **Go testing**: Do not use the standard `testing` package. Use the custom test registry in `src/core/test/`.
- **Functions**: Keep functions short, single-purpose. Prefer functions and structs over complex patterns.

## Testing

- `./dialtone.sh plugin test www` runs Chromedp-based integration tests that capture browser console output and JS exceptions. Tests fail on console errors.
- Browser tests use `--gpu` flag for rendering. Set `CAD_LIVE=true` for live CAD backend (otherwise mocked).
- Tests use a tag-based registry system for filtering.
- Tests must return `error` (nil = pass) and clean up spawned processes.

## Key Dependencies

Go 1.25.5 with: chromedp (browser automation), NATS (messaging), Tailscale (VPN), gomavlib (MAVLink), go-duckdb (ticket storage), websocket, sftp, protobuf.

Web: Vite, TypeScript 5.9, Three.js, D3.js, globe.gl, h3-js, nats.ws, xterm.

# Agent Guide: Dialtone

This guide provides essential information for AI agents working in the Dialtone repository.

## Project Overview
Dialtone is a robotic video operations network. The codebase is primarily Go, with a web UI (TypeScript/Vite) and some Python (JAX demo).

## Critical Workflow: "Ticket-First"
Every change MUST be part of a ticket. **Always run this first**:
```bash
./dialtone.sh ticket start <ticket-name>
```
This command:
1. Creates/switches to a git branch named `<ticket-name>`.
2. Scaffolds `tickets/<ticket-name>/` with `ticket.md`, `progress.txt`, and a test template.
3. Commits the scaffolding.
4. Pushes the branch and creates a GitHub Pull Request.

### Ticket Structure (`tickets/<ticket-name>/`)
- `ticket.md`: Requirements and subtasks. Use `dialtone.sh ticket validate` to check format.
- `progress.txt`: Log your progress and important notes here.
- `test/test.go`: Ticket-specific verification logic. Register tests here using `test.Register`.
- `code/`: Scratchpad for temporary code.

## Essential Commands
All development actions should go through `./dialtone.sh`, which wraps the `dialtone-dev` tool (`src/dev.go`).

| Command | Description |
| :--- | :--- |
| `./dialtone.sh install` | Install local dev dependencies (Go, Node, gh, etc.) |
| `./dialtone.sh build --full` | Build everything: Web UI + CLI + Robot binary |
| `./dialtone.sh test` | Run all tests |
| `./dialtone.sh test ticket <name>` | Run tests for a specific ticket |
| `./dialtone.sh ticket subtask list` | Show subtasks for the current ticket |
| `./dialtone.sh ticket subtask done <name>` | Mark a subtask as completed in `ticket.md` |
| `./dialtone.sh ticket done` | Final verification, push, and PR submission |

## Development Hierarchy
1. **Ticket**: New features or patches go here first.
2. **Plugin**: Mature features live in `src/plugins/<name>`.
3. **Core**: Critical networking/bootstrap logic in `src/core/`. Avoid changing unless necessary.

## Testing Pattern
Tests are the "most important concept" in Dialtone. 
- **Registry**: Use `src/core/test/registry.go` to register tests.
- **Ticket Tests**: Located in `tickets/<name>/test/test.go`.
- **Plugin Tests**: Located in `src/plugins/<name>/test/`.
- **Command**: `./dialtone.sh test [ticket|plugin|tags] <name>`.

## Code Organization
- `src/cmd/dialtone`: Production binary entry point.
- `src/cmd/dev`: Development binary entry point.
- `src/plugins/`: Feature modules (vpn, camera, mavlink, ai, etc.).
- `src/core/`: Shared libraries (logger, config, nats, tailscale).
- `src/plugins/www/app`: Main website (Vercel/Vite).

## Gotchas & Conventions
- **Single Binary**: The project aims for a "Single Software Binary" (SSB).
- **Embedded Assets**: Web assets are embedded into the Go binary using `//go:embed`.
- **Tailscale**: Networking relies heavily on `tsnet`. `TS_AUTHKEY` is required for many operations.
- **Mock Mode**: Use `./dialtone.sh start --mock` to develop without hardware.
- **Style**: Follow existing Go patterns. Use the `logger` package for all output.
- **Naming**: Use kebab-case for ticket names and subtask names.
- **PRs**: `dialtone.sh ticket start` creates the PR immediately. Use `dialtone.sh ticket done` to finalize.

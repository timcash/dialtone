# REPL Plugin Review

## Overview
The REPL plugin is the interactive heart of the Dialtone project. It provides a managed environment for executing commands, coordinating multiple agents, and maintaining persistent services. It distinguishes itself by offering both a local-first interactive experience and a distributed, NATS-powered shared environment.

## Core Components

### 1. Interactive REPL (`run`)
- **Persona**: Uses a "Virtual Librarian" persona for user interaction.
- **Subtones**: Commands are executed as "subtones" managed via the `proc` plugin. This provides structured logging, PID tracking, and the ability to run tasks in the background (`&`).
- **Output Handling**: Supports structured log levels (`[INFO]`, `[ERROR]`, etc.) and prefixes output with the source (e.g., `DIALTONE:1234>`).

### 2. Distributed REPL (`serve` / `join`)
- **NATS Bus**: Uses a frame-based protocol over NATS (`repl.<room>`).
- **Roles**:
    - **Host (`serve`)**: Listens for `input` frames, executes them, and broadcasts `line` frames (stdout/stderr) and `server` status.
    - **Client (`join`)**: Sends `input` frames and displays broadcasted output.
- **Integrations**: 
    - **Tailscale (`tsnet`)**: Can optionally start an embedded Tailscale identity for the host.
    - **Embedded NATS**: Can automatically start a NATS broker if one isn't available.

### 3. Service Supervisor (`service`)
The `service` command provides a robust way to run the REPL as a persistent background process with self-updating capabilities.

#### How the Auto-Download Daemon Works (`--mode run`):
1.  **Bootstrap**: On startup, it checks the local install directory (default `~/.dialtone/repl`) and identifies the `current` active binary.
2.  **Latest Release Query**: It polls the GitHub API for the target repository (default `timcash/dialtone`). It uses `GITHUB_TOKEN` from the environment if available.
3.  **Architecture Matching**: It looks for an asset matching the local OS and architecture (e.g., `repl-src_v1-linux-arm64`).
4.  **Version Comparison**: It uses a semver-aware comparison logic to determine if the GitHub version is newer than the running version.
5.  **Atomic Updates**:
    - Downloads the new binary to a versioned subdirectory (`releases/<tag>/`).
    - Updates a `current` symlink to point to the new binary.
6.  **Hot-Swapping**:
    - The supervisor process remains alive.
    - It sends a `SIGTERM` to the existing worker (the `serve` process).
    - Once the old worker exits, it spawns a new one using the updated binary.
7.  **Persistence**: Includes `install` logic for `systemd` (Linux) and `launchd` (macOS) to ensure the supervisor itself starts on boot.

### 4. Release Tooling (`release`)
- **Cross-Compilation**: The `build` command automates Go cross-compilation for `linux`, `darwin`, and `windows` across `amd64` and `arm64`.
- **Automated Publishing**: The `publish` command uses the `github` plugin to create a release and upload all architecture-specific binaries as assets.

## Architectural Strengths
- **Resilience**: The supervisor/worker split ensures that even if a worker crashes or is being updated, the management layer remains stable.
- **Zero-Config Networking**: The combination of embedded NATS and Tailscale makes it easy to set up a shared REPL across different networks.
- **Plugin Synergy**: It effectively composes functionality from `proc` (execution), `logs` (transport), `config` (paths), and `github` (updates).

## Areas for Investigation / Improvement
- **Windows Service Support**: While it builds for Windows, the `install` logic currently only targets Linux and macOS.
- **Update Security**: Adding checksum validation (e.g., SHA256) for downloaded binaries would enhance security.
- **Task Log Integration**: The `TASK_LOG.md` defines a sophisticated dependency-based workflow that could be further integrated into the core REPL loop to automate multi-step engineering tasks.

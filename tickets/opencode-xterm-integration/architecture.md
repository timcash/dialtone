# Architecture: Opencode Xterm Integration

This document outlines the data flow and components involved in the Opencode Xterm.js integration within Dialtone.

## Components

### 1. Opencode Shim
- **Location**: `/home/tim/dialtone_deploy/opencode` (or dynamic shim in dev/test)
- **Role**: Intercepts `opencode` CLI commands.
- **Action**: Redirects arguments to `dialtone ai chat "$@"`.

### 2. AI Plugin (Bridge)
- **File**: `src/plugins/ai/app/ai.go`
- **Function**: `RunOpencodeServer` / `bridgeOpencodeToNATS`
- **Role**: Bridges TTY I/O to NATS.
- **Mechanism**:
  - Starts a `bash` shell.
  - Subscribes to `ai.opencode.input`.
  - Publishes `stdout` and `stderr` to `ai.opencode.output`.

### 3. NATS Server
- **Role**: Messaging Backbone.
- **Topics**:
  - `ai.opencode.input`: Commands sent from UI Terminal -> AI Plugin.
  - `ai.opencode.output`: Output from AI Plugin/Shell -> UI Terminal.

### 4. Web UI (Dashboard)
- **File**: `src/core/web/src/main.ts`
- **Component**: `xterm.js` Terminal.
- **Role**: User Interface.
- **Flow**:
  - User types in `xterm.js`.
  - JS publishes to `ai.opencode.input` via `nats.ws`.
  - JS subscribes to `ai.opencode.output`.
  - Received data is written to `xterm.js` display.

## Data Flow Diagram

```mermaid
graph TD
    User[User @ Browser] -->|Types Command| GUI[xterm.js]
    GUI -->|NATS WS "ai.opencode.input"| NATS[NATS Server]
    NATS -->|Subscribed| Bridge[AI Plugin Bridge]
    Bridge -->|Writes to Stdin| Shell[Bash Shell / Opencode]
    Shell -->|Stdout/Stderr| Bridge
    Bridge -->|NATS "ai.opencode.output"| NATS
    NATS -->|NATS WS Subscribed| GUI
    GUI -->|Displays| User
```

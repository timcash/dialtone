# REPL v3: Semantic Orchestration & Unified Event Bus

## 1. Goal: High-Signal Terminals
The core mission of REPL v3 is to ensure the **Interactive Session** remains high-signal while background **Worker Payloads** are isolated but easily inspectable.

- **Index Room (`repl.room.index`)**: A high-level stream of command submissions, subtone lifecycle events (Started/Exited), and system status.
- **Subtone Rooms (`repl.subtone.<pid>`)**: Dedicated data streams for raw worker output.

---

## 2. Infrastructure Layer (The "Clean Room" Initiative)

### A. Logging Library (`logs/src_v1`) - [STABLE]
- ✅ **Context Detection**: Library checks for `DIALTONE_CONTEXT=repl`.
- ✅ **Header Suppression**: Headers like `[T+0000s|INFO|...]` are suppressed in REPL context.
- ✅ **Semantic APIs**: `logs.System()` and `logs.User()` exist and reduce manual string branding.
- [NEXT] **Direct Frame Emission**: Refactor `publishPrimary` to emit `BusFrame` JSON directly to NATS instead of relying on the Leader to wrap text lines.

### B. REPL Leader (The "Smart Router") - [STABLE CORE]
- ✅ **Isolated Publishing**: `publishScopedFrame` routes by `Scope`.
- ✅ **Lifecycle Mirroring**: The index room carries subtone start/room/log-path/exit lifecycle events.
- ✅ **Strict Isolation**: Detailed subtone stdout/stderr is already confined to `repl.subtone.<pid>`; the Index Room is lifecycle/status only.
- [NEXT] **Silence Local Leader Output**: When running in REPL mode, the Leader process should not print NATS-bound frames to its own `os.Stdout` to prevent double-printing for the local user.

---

## 3. User Experience (UX) Layer

### A. The "Attach" Primitive - [STABLE]
- ✅ **Implementation**: `/subtone-attach` and `/subtone-detach` are now implemented in `RunJoin`.
- ✅ **Subscription Management**: The client dynamically manages secondary NATS subscriptions.
- [NEXT] **History Playback**: When `/subtone-attach` is called, the client should tail the last 20 lines of the subtone's log file to provide immediate context.

### B. Semantic Rendering - [STABLE]
- ✅ **Branding Ownership**: The Renderer (`core_join_console.go`) and `dialtone-tap` now centrally manage the `DIALTONE>` and `DIALTONE:PID>` prefixes based on frame metadata.
- ✅ **Consistent Prefixing**: `printFrame` updated to ensure all frame types (Chat, Server, Line) use the metadata-driven branding.

---

## 4. Stability & Testing

### A. Metadata-Driven Assertions
- ✅ **Update**: `09_subtone_attach` uses frame-field assertions.
- [NEXT] **Regression Cleanup**: Migrate older tests in `src/plugins/repl/src_v3/test/**` to assert on `frame.Kind` and `frame.Type` rather than regex-matching rendered strings.

### B. Registry-Based Discovery
- ✅ **Leader Registry**: The Leader now maintains a recent subtone registry with PID, Command, Room, LogPath, active/exited state, and exit code.
- ✅ **Remote List**: `subtone-list` now queries this registry over NATS first, and `subtone-log` can resolve log paths from the same registry.
- [NEXT] **Registry as Single Source**: Move `/ps` fully onto the registry model and remove the remaining fallback behavior.

---

## 5. Current State
- ✅ `./dialtone.sh repl src_v3 test` passes end to end.
- ✅ `src_v3` is exercising the main v4 operator model: index room for lifecycle, subtone rooms for payload.
- ✅ The tmp bootstrap test entrypoint cleans process state itself; tmp folders are cleaned separately via `./dialtone.sh repl src_v3 test-clean`.

## 6. Priority Roadmap (The "Final Polish")
1.  **Disable Mirroring**: Silence raw subtone logs in the Index Room.
2.  **Fix Double-Printing**: Ensure the Leader doesn't print what it's already publishing to NATS.
3.  **Registry Cleanup**: Remove the remaining `ListManagedProcesses` fallback and let `/ps` rely entirely on the Leader registry.
4.  **History Playback**: Add "Last 20 lines" context to `/subtone-attach`.

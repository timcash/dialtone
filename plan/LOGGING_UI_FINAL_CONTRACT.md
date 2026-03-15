# Final Logging & UI Abstraction Contract

## 1. Goal
Decouple project branding (`DIALTONE>`) and developer metadata (`[T+0000s|...]`) from business logic. Move all presentation choices to the final display layer (REPL/Tap).

## 2. The Semantic Protocol (`BusFrame`)
The project now uses a metadata-first NATS protocol. The `BusFrame` struct in `core_runtime.go` is the source of truth for all events.

### Key Fields:
- `Scope`: `index` (Global session) or `subtone` (Specific worker process).
- `Kind`: `lifecycle` (Started/Stopped), `status` (System feedback), `log` (Stdout), `error` (Stderr/Failure), `chat` (User input).
- `SubtonePID`: The logical ID of the worker process.
- `Message`: The **raw** content string, free of any hardcoded prefixes.

### Semantic Rules:
- `Scope` controls routing and default renderer context.
- `Kind` controls event meaning and styling.
- `Message` is user-facing copy, not protocol.
- Tests should prefer semantic fields over exact message wording whenever possible.

---

## 3. Mandatory Implementation Steps

### Phase 1: Logging Library Refactor (`logs/src_v1`)
The library must support semantic modes to clean up the `Message` field.
1.  **Add `logs.System(format, ...)`**: Emits a structured log record with semantic `Kind: "status"`.
2.  **Add `logs.User(format, ...)`**: Emits a structured log record with semantic `Kind: "chat"`.
3.  **Header Suppression**:
    - If `DIALTONE_CONTEXT=repl` is set, the library must **not** prepend developer headers (`[T+0000s|INFO|...]`) to the string sent over NATS.
    - These headers should still be written to the local `.log` file for debugging.
    - This env-var behavior is a migration aid, not the architectural center of the design.

### Phase 2: REPL Runtime Integration
1.  **Update `executeCommand`**: Ensure all lifecycle events (Started, Log file, Exited) use the `BusFrame` fields correctly.
2.  **Middleman Logic**: The REPL Leader should continue to wrap raw subtone stdout as `Kind: "log"` and stderr as `Kind: "error"`, ensuring `Scope: "subtone"` is always set.
3.  **Structured Pass-Through**: If structured log records are available, the runtime may map them into `BusFrame`s directly. Parsing JSON stdout should be treated as migration compatibility, not the long-term primary design.

### Phase 3: Display Layer (Renderer)
The renderer (in `core_join_console.go` and `dialtone-tap`) is the **only** place that adds prefixes.
- **Rule 1**: If `Scope == "index"`, prepend `DIALTONE> `.
- **Rule 2**: If `Scope == "subtone"`, prepend `DIALTONE:PID> ` when attached to that subtone view.
- **Rule 3**: `Kind` controls styling and emphasis, not routing. For example, `Kind == "error"` may render as red text or include an `[ERROR]` tag.

### Phase 4: Business Logic Cleanup
1.  **Remove Literals**: Run a global search for `"DIALTONE> "` and remove them from all Go files.
2.  **Swap Methods**: Replace `logs.Info("DIALTONE> ...")` with `logs.System("...")`.

---

## 4. Current Progress Review
- ✅ **BusFrame Contract**: Implemented in `core_runtime.go`.
- ✅ **Semantic Routing**: `publishScopedFrame` correctly routes by `Scope`.
- ⚠️ **Logging Library**: Still needs `System()`/`User()` methods and header suppression. (Highest Priority).
- ⚠️ **Tests**: Should move further toward semantic assertions on `Scope`, `Kind`, `SubtonePID`, `LogPath`, `ExitCode`, and `Ready`, instead of depending on lifecycle copy.

---

## 5. Technical Risks
- **Duplicate Broadcasts**: Separate leader debug logs from REPL room events. Suppressing one must not accidentally suppress the other.
- **Bootstrapping**: The logging library must gracefully fallback to stdout with a default `DIALTONE>` prefix if NATS is unavailable during the very first seconds of process startup.

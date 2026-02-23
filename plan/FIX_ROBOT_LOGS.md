# Workflow: Fix Robot Plugin Logging

This document outlines the steps to migrate `src/plugins/robot/src_v1` from native printing (`fmt`, `log`, `console`) to the centralized `@src/plugins/logs/src_v1` library.

## Goal
Ensure all runtime telemetry, errors, and lifecycle events are captured by the `logs` plugin, enabling structured logging, NATS-based log streaming, and persistence in `test.log`.

---

## Phase 1: Go Backend (Priority: High)

The Go server and sleep relay currently bypass the logging pipeline, meaning their output isn't captured in structured formats or streamed via NATS.

### 1. Update `cmd/server/main.go` and `cmd/sleep/main.go`
- **Imports**: Add `"dialtone/dev/plugins/logs/src_v1/go"` as `logs`.
- **Replacement Mapping**:
  - `log.Printf(...)` -> `logs.Info(...)`
  - `log.Fatalf(...)` -> `logs.Fatal(...)`
  - Errors inside handlers -> `logs.Error(...)`
- **Configuration**: Ensure `logs.SetOutput(os.Stdout)` is called in `main()` if terminal feedback is needed, but the primary goal is triggering the `publishPrimary` NATS hook inside the library.

### 2. Update `cmd/ops/*.go`
- Replace `fmt.Printf` and `fmt.Println` used for status updates with `logs.Info`.
- **Exception**: Keep `fmt.Printf` for the final result line if specific CLI parsers depend on the raw stdout format (e.g., build paths).

---

## Phase 2: Test Suite (Priority: Medium)

The test suite uses `fmt.Println` for progress tracking. Moving these to `logs` ensures they appear in the `test.log` artifact with correct timestamps.

### 1. Update `test/*.go`
- **Context Logging**: Use `logs.InfoFromTest("robot-test", ...)` instead of `fmt.Println`.
- **Verification**: Update `Run09ExpectedErrorsProofOfLife` to verify that the `logs` library successfully captured the intent, rather than just checking raw stdout.

---

## Phase 3: UI Frontend (Priority: Medium)

The UI uses `console.log`. To integrate with the `logs` plugin, we should route critical UI events through the established NATS connection.

### 1. Create a UI Log Utility
- In `ui/src/data/logging.ts`, create a wrapper that:
  1. Calls `console.log` (for local dev).
  2. Publishes a JSON payload to `logs.ui.robot` via NATS if the connection is active.
- **Payload Structure**: Match the `Record` struct in `@src/plugins/logs/src_v1/go/nats.go`:
  ```json
  {
    "level": "INFO",
    "message": "...",
    "source": "ui/component-name",
    "timestamp": "ISO-8601"
  }
  ```

### 2. Replace `console` calls
- Update `ui/src/main.ts` and `ui/src/data/connection.ts` to use this new utility.

---

## Phase 4: Validation

1. **Build and Run**:
   ```bash
   ./dialtone.sh robot src_v1 build
   ./dialtone.sh robot src_v1 test
   ```
2. **Inspect Artifacts**:
   - Check `src/plugins/robot/src_v1/test/test.log`. It should now contain the "Robot server config" and "MAVLink bridge" lines that were previously lost to stdout.
3. **NATS Verification**:
   - Run a NATS listener: `./dialtone.sh logs listen logs.runtime`
   - Start the robot server and verify that startup logs appear in the NATS stream.

---

## Cheat Sheet: `logs` Library API

| Old Call | New Call | Note |
| :--- | :--- | :--- |
| `log.Printf(f, a)` | `logs.Info(f, a)` | Standard info log |
| `fmt.Errorf(f, a)` | `logs.Errorf(f, a)` | Logs error AND returns error object |
| `log.Fatalf(f, a)` | `logs.Fatal(f, a)` | Logs and calls `os.Exit(1)` |
| `fmt.Println(s)` | `logs.Raw("%s", s)` | Log without the standard prefix |

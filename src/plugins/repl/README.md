# REPL Plugin

The REPL plugin provides focused tooling to develop and test the `dialtone.sh` interactive flow.

## Commands

```bash
./dialtone.sh repl install
./dialtone.sh repl test
./dialtone.sh repl help
```

After install, run the local Python bridge with pixi:

```bash
cd src/plugins/repl
pixi run bridge
```

## Purpose

- Validate `USER-1>` and `DIALTONE>` dialog behavior.
- Verify `@DIALTONE ...` request handling.
- Verify task-sign gating and subtone PID streaming output.

## Example

```bash
./dialtone.sh repl test --timeout 180
```

## Writing REPL Tests

The REPL testing framework (`src/plugins/repl/src_v1/test/`) simulates a user interacting with the `dialtone.sh` shell. It supports asynchronous operations, log verification, and robust process cleanup.

### Interaction Model

Tests run an interactive `dialtone.sh` session in the background. You send input and poll for expected output.

```go
func RunMyTest(ctx *testCtx) (string, error) {
    // 1. Start the REPL
    if err := ctx.StartREPL(); err != nil {
        return "", err
    }
    defer ctx.Cleanup() // Ensures REPL and subtones are killed

    // 2. Wait for prompt
    if err := ctx.WaitForOutput("USER-1>", 5*time.Second); err != nil {
        return "", err
    }

    // 3. Send command
    if err := ctx.SendInput("my-plugin run"); err != nil {
        return "", err
    }
    
    // ...
}
```

### Verifying Subtones (Async Processes)

Commands often spawn "subtones" (background processes) that log to files rather than the console to keep the REPL clean. To verify these, use `WaitForLogEntry` instead of checking REPL output.

```go
    // Wait for the subtone to start (REPL acknowledges receipt)
    if err := ctx.WaitForOutput("Started at", 5*time.Second); err != nil {
        return "", err
    }

    // Verify the command execution by checking the persistent logs
    // Look for a log file with "subtone-" prefix containing specific text
    if err := ctx.WaitForLogEntry("subtone-", "Operation completed successfully", 10*time.Second); err != nil {
        return "", err
    }
```

### Process Cleanup

Tests must ensure that spawned processes do not leak into subsequent tests.

1.  **Defer Cleanup:** Always `defer ctx.Cleanup()` immediately after `StartREPL`. This kills the REPL process and attempts to kill subtones.
2.  **Explicit Wait (Optional):** For tests that spawn long-running background tasks (like `proc sleep`), you may want to explicitly wait for them to finish to ensure a clean state.

```go
    // Poll 'ps' until all subtones are gone
    deadline := time.Now().Add(15 * time.Second)
    for time.Now().Before(deadline) {
        ctx.SendInput("ps")
        if ctx.WaitForOutput("No active subtones", 1*time.Second) == nil {
            break
        }
        time.Sleep(1 * time.Second)
    }
```

### Test Context Tools

The `testCtx` object provides:

- `StartREPL()`: Launches `dialtone.sh` in the background.
- `SendInput(cmd)`: types a command into the REPL.
- `WaitForOutput(text, timeout)`: Scans the cumulative REPL output for a string.
- `WaitForLogEntry(filePattern, content, timeout)`: Scans `.dialtone/logs/` for specific content.
- `SetTimeout(duration)`: Sets the global test timeout.
- `Cleanup()`: Terminates the session.

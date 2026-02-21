# Logs Plugin – NATS-First Logging

All logs go through **NATS** by default. Producers (plugins, services) publish only to NATS; no file, no stdout from the app. Readers (CLI, file writer, browser) subscribe to NATS and can attach stdout, files, or UIs as they want. 

---

## Core Mandates

1.  **Library Usage:** All plugin code, scaffolds, and orchestrators (including `dev.go`) **must** use the `logs` library (`dialtone/dev/plugins/logs/src_v1/go`) instead of `fmt`.
2.  **Silence by Default:** The global logger is silenced (`io.Discard`) by default. Output is only visible via NATS subscription or explicit listener.
3.  **No Direct Print:** The use of `fmt.Print/Printf/Println` for standard output is prohibited. All communication must flow through the structured logging API.

---

## How It Works

### 1. Single bus: NATS

- **Producers** publish log lines to NATS subjects. They do not write to files or stdout.
- **Consumers** attach to NATS:
  - **CLI** can subscribe and print to **stdout** via `./dialtone.sh logs stream`.
  - **File**: Use `logs.ListenToFile(conn, subject, filePath)` to start a listener.
  - **Browser** subscribes over NATS (WebSocket) and displays in an xterm.

### 2. Subject naming

- **Pattern**: `logs.<plugin-name>.<run-or-test-name>`
- **Examples**: `logs.dag.my-test`, `logs.task.smoke`, `logs.robot.telemetry`

### 3. CLI Commands

- **`./dialtone.sh logs test src_v1`** – Run the logs plugin verification suite.
- **`./dialtone.sh logs stream --topic <subject>`** – Stream logs to stdout.
- **`./dialtone.sh logs nats-start`** – Start the local NATS daemon.

#### Topic Filtering Examples (NATS Wildcards)

The `--topic` flag supports standard NATS subject wildcards:
- `*` matches a single token.
- `>` matches all remaining tokens.

| Command | Description |
|---------|-------------|
| `./dialtone.sh logs stream --topic 'logs.>'` | Stream **all** logs from all plugins. |
| `./dialtone.sh logs stream --topic 'logs.task.>'` | Stream all logs for the **task** plugin. |
| `./dialtone.sh logs stream --topic 'logs.*.smoke'` | Stream **smoke test** logs for any plugin. |
| `./dialtone.sh logs stream --topic 'logs.dag.v1'` | Stream a **specific** DAG v1 log run. |


---

## Plugin Author Guide

### Go (backend)

1.  **Import:** `import logs "dialtone/dev/plugins/logs/src_v1/go"`
2.  **Log:** Call `logs.Info("message")`. The message is published to NATS and silent on stdout.

### NATS Verification in Tests

When writing tests, use the `test` plugin's `StepContext` to verify behavior via NATS topics:

```go
func RunMyStep(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
    ctx.Logf("Triggering action...")
    // Wait for a specific message on a NATS topic
    err := ctx.WaitForMessage("logs.my-plugin.results", "success-marker", 5*time.Second)
    if err != nil {
        return testv1.StepRunResult{}, err
    }
    return testv1.StepRunResult{Report: "Verified via NATS!"}, nil
}
```

---

## Verification

The logs system itself is verified via:
- `./dialtone.sh logs test src_v1`
